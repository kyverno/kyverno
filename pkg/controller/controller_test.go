package controller

import (
	"testing"

	"github.com/golang/glog"
	types "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	event "github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/sharedinformer"
	violation "github.com/nirmata/kyverno/pkg/violation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/sample-controller/pkg/signals"
)

func TestCreatePolicy(t *testing.T) {
	f := newFixture(t)
	// new policy is added to policy lister and explictly passed to sync-handler
	// to process the existing
	policy := newPolicy("test-policy")
	f.policyLister = append(f.policyLister, policy)
	f.objects = append(f.objects, policy)
	// run controller
	f.runControler("test-policy")
}

func (f *fixture) runControler(policyName string) {
	policyInformerFactory, err := sharedinformer.NewFakeSharedInformerFactory()
	if err != nil {
		f.t.Fatal(err)
	}

	eventController := event.NewEventController(f.Client, policyInformerFactory)
	violationBuilder := violation.NewPolicyViolationBuilder(f.Client, policyInformerFactory, eventController)

	// new controller
	policyController := NewPolicyController(
		f.Client,
		policyInformerFactory,
		violationBuilder,
		eventController,
		nil)

	stopCh := signals.SetupSignalHandler()
	// start informer & controller
	policyInformerFactory.Run(stopCh)
	if err = policyController.Run(stopCh); err != nil {
		glog.Fatalf("Error running PolicyController: %v\n", err)
	}
	// add policy to the informer
	for _, p := range f.policyLister {
		policyInformerFactory.GetInfomer().GetIndexer().Add(p)
	}

	// sync handler
	// reads the policy from the policy lister and processes them
	err = policyController.syncHandler(policyName)
	if err != nil {
		f.t.Fatal(err)
	}
	policyController.Stop()

}

type fixture struct {
	t            *testing.T
	Client       *client.Client
	policyLister []*types.Policy
	objects      []runtime.Object
}

func newFixture(t *testing.T) *fixture {

	// init groupversion
	regResource := []schema.GroupVersionResource{
		schema.GroupVersionResource{Group: "group", Version: "version", Resource: "thekinds"},
		schema.GroupVersionResource{Group: "group2", Version: "version", Resource: "thekinds"},
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
	}

	objects := []runtime.Object{newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
		newUnstructured("group2/version", "TheKind", "ns-foo", "name2-foo"),
		newUnstructured("group/version", "TheKind", "ns-foo", "name-bar"),
		newUnstructured("group/version", "TheKind", "ns-foo", "name-baz"),
		newUnstructured("group2/version", "TheKind", "ns-foo", "name2-baz"),
		newUnstructured("apps/v1", "Deployment", "kyverno", "kyverno"),
	}

	scheme := runtime.NewScheme()
	// Create mock client
	fclient, err := client.NewMockClient(scheme, objects...)
	if err != nil {
		t.Fatal(err)
	}

	// set discovery Client
	fclient.SetDiscovery(client.NewFakeDiscoveryClient(regResource))

	f := &fixture{
		t:      t,
		Client: fclient,
	}

	return f
}

// create mock client with initial resouces
// set registered resources for gvr
func (f *fixture) setupFixture() {
	scheme := runtime.NewScheme()
	fclient, err := client.NewMockClient(scheme, f.objects...)
	if err != nil {
		f.t.Fatal(err)
	}

	regresource := []schema.GroupVersionResource{
		schema.GroupVersionResource{Group: "kyverno.io",
			Version:  "v1alpha1",
			Resource: "policys"}}
	fclient.SetDiscovery(client.NewFakeDiscoveryClient(regresource))
}

func newPolicy(name string) *types.Policy {
	return &types.Policy{
		TypeMeta: metav1.TypeMeta{APIVersion: types.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
		},
	}
}
