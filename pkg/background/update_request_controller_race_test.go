package background

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/gpol"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	gpolengine "github.com/kyverno/kyverno/pkg/cel/policies/gpol/engine"
	kyvernofake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/logging"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// raceDelegate is a real AggregatedDiscoveryInterface: v1/Namespace always
// resolves, but the target group only appears after readyAt.
type raceDelegate struct {
	targetGV       schema.GroupVersion
	targetKind     string
	targetResource string
	readyAt        time.Time
}

func (d *raceDelegate) GroupsAndMaybeResources() (*metav1.APIGroupList, map[schema.GroupVersion]*metav1.APIResourceList, map[schema.GroupVersion]error, error) {
	coreGV := schema.GroupVersion{Version: "v1"}
	groups := &metav1.APIGroupList{Groups: []metav1.APIGroup{
		{
			Name:             "",
			Versions:         []metav1.GroupVersionForDiscovery{{GroupVersion: "v1", Version: "v1"}},
			PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "v1", Version: "v1"},
		},
	}}
	resources := map[schema.GroupVersion]*metav1.APIResourceList{
		coreGV: {
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{{Name: "namespaces", Kind: "Namespace", Namespaced: false}},
		},
	}
	if !time.Now().Before(d.readyAt) {
		groups.Groups = append(groups.Groups, metav1.APIGroup{
			Name:             d.targetGV.Group,
			Versions:         []metav1.GroupVersionForDiscovery{{GroupVersion: d.targetGV.String(), Version: d.targetGV.Version}},
			PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: d.targetGV.String(), Version: d.targetGV.Version},
		})
		resources[d.targetGV] = &metav1.APIResourceList{
			GroupVersion: d.targetGV.String(),
			APIResources: []metav1.APIResource{
				{Name: d.targetResource, Kind: d.targetKind, Group: d.targetGV.Group, Version: d.targetGV.Version, Namespaced: true},
			},
		}
	}
	return groups, resources, nil, nil
}

func (d *raceDelegate) RESTClient() restclient.Interface { return nil }
func (d *raceDelegate) ServerGroups() (*metav1.APIGroupList, error) {
	panic("not used by the aggregated refresh path")
}

func (d *raceDelegate) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	panic("not used by the aggregated refresh path")
}

func (d *raceDelegate) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	panic("not used by the aggregated refresh path")
}

func (d *raceDelegate) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	panic("not used by the aggregated refresh path")
}

func (d *raceDelegate) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	panic("not used by the aggregated refresh path")
}

func (d *raceDelegate) ServerVersion() (*version.Info, error) {
	panic("not used by the aggregated refresh path")
}

func (d *raceDelegate) OpenAPISchema() (*openapi_v2.Document, error) {
	panic("not used by the aggregated refresh path")
}

func (d *raceDelegate) OpenAPIV3() openapi.Client {
	panic("not used by the aggregated refresh path")
}

func (d *raceDelegate) WithLegacy() discovery.DiscoveryInterface {
	panic("not used by the aggregated refresh path")
}

var _ discovery.AggregatedDiscoveryInterface = &raceDelegate{}

// raceEngine stands in for the CEL engine's GenerateResources step: it
// resolves the target GVR and calls client.CreateResource, same as production.
type raceEngine struct {
	client   dclient.Interface
	target   *unstructured.Unstructured
	attempts atomic.Int32
}

func (e *raceEngine) Handle(_ celengine.EngineRequest, p gpolengine.Policy, _ bool) (gpolengine.EngineResponse, error) {
	e.attempts.Add(1)
	gvk := e.target.GroupVersionKind()
	if _, err := e.client.Discovery().GetGVRFromGVK(gvk); err != nil {
		return gpolengine.EngineResponse{
			Trigger: &unstructured.Unstructured{},
			Policies: []gpolengine.GeneratingPolicyResponse{{
				Policy: p.Policy,
				Result: engineapi.RuleError("rule", engineapi.Generation, "failed to generate resource", err, nil),
			}},
		}, nil
	}
	created, err := e.client.CreateResource(context.TODO(), e.target.GetAPIVersion(), e.target.GetKind(), e.target.GetNamespace(), e.target.DeepCopy(), false)
	if err != nil {
		return gpolengine.EngineResponse{
			Trigger: &unstructured.Unstructured{},
			Policies: []gpolengine.GeneratingPolicyResponse{{
				Policy: p.Policy,
				Result: engineapi.RuleError("rule", engineapi.Generation, "failed to create generated resource", err, nil),
			}},
		}, nil
	}
	return gpolengine.EngineResponse{
		Trigger: &unstructured.Unstructured{},
		Policies: []gpolengine.GeneratingPolicyResponse{{
			Policy: p.Policy,
			Result: engineapi.RulePass("rule", engineapi.Generation, "generated", nil).WithGeneratedResources([]*unstructured.Unstructured{created}),
		}},
	}, nil
}

type raceProvider struct{ policy gpolengine.Policy }

func (p *raceProvider) Get(context.Context, string) (gpolengine.Policy, error) { return p.policy, nil }

type noopEventGen struct{}

func (noopEventGen) Add(...event.Info) {}

// TestCELGenerate_NewCRDDiscoveryRace: a GeneratingPolicy targets a CRD
// installed moments earlier, so early attempts hit a discovery miss. A UR is
// attempted at ~0, 0.3s, 0.9s, 2.1s and 4.5s, and its 5th failure deletes it,
// so recovering on the last attempt leaves no slack.
func TestCELGenerate_NewCRDDiscoveryRace(t *testing.T) {
	for name, readyAfter := range map[string]time.Duration{
		"kind appears before the first retry": 500 * time.Millisecond,
		"kind appears between retries":        1500 * time.Millisecond,
	} {
		t.Run(name, func(t *testing.T) {
			testNewCRDDiscoveryRace(t, readyAfter)
		})
	}
}

func testNewCRDDiscoveryRace(t *testing.T, readyAfter time.Duration) {
	t.Helper()
	prevReportingCfg := reportutils.ReportingCfg
	reportutils.ReportingCfg = reportutils.NewReportingConfig(nil)
	t.Cleanup(func() { reportutils.ReportingCfg = prevReportingCfg })

	targetGVK := schema.GroupVersionKind{Group: "cilium.io", Version: "v2", Kind: "CiliumNetworkPolicy"}
	targetGVR := schema.GroupVersionResource{Group: "cilium.io", Version: "v2", Resource: "ciliumnetworkpolicies"}

	// Mirrors the real fixture:
	// test/conformance/chainsaw/generating-policies/template/generate-ciliumnetworkpolicy
	namespace := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]interface{}{
			"name": "cnp-template-ns",
			"uid":  "trigger-uid",
		},
	}}
	target := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cilium.io/v2",
		"kind":       "CiliumNetworkPolicy",
		"metadata": map[string]interface{}{
			"name":      "cnp-template-ns-victoria-scraping",
			"namespace": "cnp-template-ns",
		},
		"spec": map[string]interface{}{
			"endpointSelector": map[string]interface{}{
				"matchLabels": map[string]interface{}{"kubernetes.io/metadata.name": "cnp-template-ns"},
			},
		},
	}}

	client, err := dclient.NewFakeClient(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		targetGVR: "CiliumNetworkPolicyList",
	}, namespace)
	if err != nil {
		t.Fatalf("failed to build fake client: %v", err)
	}
	client.SetDiscovery(dclient.NewServerResourcesDiscovery(&raceDelegate{
		targetGV:       targetGVK.GroupVersion(),
		targetKind:     targetGVK.Kind,
		targetResource: targetGVR.Resource,
		readyAt:        time.Now().Add(readyAfter),
	}))

	policy := &policiesv1beta1.GeneratingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "generate-cilium-scrape-policy"}}
	engine := &raceEngine{client: client, target: target}

	kyvernoClient := kyvernofake.NewSimpleClientset()
	factory := kyvernoinformers.NewSharedInformerFactory(kyvernoClient, 0)
	urInformer := factory.Kyverno().V2().UpdateRequests()
	// syncUpdateRequest always looks up ur.Spec.Policy as a legacy policy
	// too, so these listers must be non-nil even though the lookup misses.
	cpolInformer := factory.Kyverno().V1().ClusterPolicies()
	polInformer := factory.Kyverno().V1().Policies()

	c := &controller{
		client:        client,
		kyvernoClient: kyvernoClient,
		cpolLister:    cpolInformer.Lister(),
		polLister:     polInformer.Lister(),
		urLister:      urInformer.Lister().UpdateRequests(config.KyvernoNamespace()),
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: "test-crd-discovery-race"},
		),
		context:      libs.NewFakeContextProvider(),
		gpolEngine:   engine,
		gpolProvider: &raceProvider{policy: gpolengine.Policy{Policy: policy}},
		watchManager: gpol.NewWatchManager(logging.WithName("test-watch-manager"), client),
		eventGen:     noopEventGen{},
	}
	if _, err := urInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addUR,
		UpdateFunc: c.updateUR,
	}); err != nil {
		t.Fatalf("failed to add event handler: %v", err)
	}

	stop := make(chan struct{})
	defer close(stop)
	factory.Start(stop)
	if !cache.WaitForCacheSync(stop, urInformer.Informer().HasSynced, cpolInformer.Informer().HasSynced, polInformer.Informer().HasSynced) {
		t.Fatal("failed to sync informers")
	}

	rawNS, err := namespace.MarshalJSON()
	if err != nil {
		t.Fatalf("failed to marshal namespace: %v", err)
	}
	adm := &admissionv1.AdmissionRequest{
		UID:       types.UID("admission-uid"),
		Operation: admissionv1.Create,
		Kind:      metav1.GroupVersionKind{Version: "v1", Kind: "Namespace"},
		Object:    runtime.RawExtension{Raw: rawNS},
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-cnp-template", Namespace: config.KyvernoNamespace()},
		Spec: kyvernov2.UpdateRequestSpec{
			Type:   kyvernov2.CELGenerate,
			Policy: policy.GetName(),
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
					AdmissionRequest: adm,
					Operation:        admissionv1.Create,
				},
			},
			RuleContext: []kyvernov2.RuleContext{{
				Rule: "generate-cilium-scrape-policy",
				Trigger: kyvernov1.ResourceSpec{
					APIVersion: "v1",
					Kind:       "Namespace",
					Name:       "cnp-template-ns",
					UID:        types.UID("trigger-uid"),
				},
			}},
		},
		Status: kyvernov2.UpdateRequestStatus{State: kyvernov2.Pending},
	}
	if _, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Create(context.TODO(), ur, metav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to create UpdateRequest: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for c.processNextWorkItem() {
		}
	}()
	t.Cleanup(func() {
		c.queue.ShutDown()
		wg.Wait()
	})

	// Generous: backoff plus -race/host-load overhead can push this well
	// past readyAt before the retry that finally lands succeeds.
	deadline := time.Now().Add(25 * time.Second)
	var created bool
	for time.Now().Before(deadline) {
		if _, err := client.GetDynamicInterface().Resource(targetGVR).Namespace(target.GetNamespace()).Get(context.TODO(), target.GetName(), metav1.GetOptions{}); err == nil {
			created = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !created {
		t.Fatalf("CiliumNetworkPolicy %s/%s was never generated: the discovery-cache miss for the just-created CRD was not retried long enough to survive", target.GetNamespace(), target.GetName())
	}
	if got := engine.attempts.Load(); got > 4 {
		t.Fatalf("CiliumNetworkPolicy was only generated on attempt %d of 5: the discovery cache kept serving a snapshot without the CRD", got)
	}
}
