package policycache

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	pcache "github.com/kyverno/kyverno/pkg/policycache"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

func testClusterPolicy(t *testing.T, name string, admission, ready bool, matchConditions []admissionregistrationv1.MatchCondition) *kyvernov1.ClusterPolicy {
	t.Helper()
	var policy kyvernov1.ClusterPolicy
	raw := `{
		"apiVersion":"kyverno.io/v1",
		"kind":"ClusterPolicy",
		"metadata":{"name":"` + name + `"},
		"spec":{
			"admission":` + map[bool]string{true: "true", false: "false"}[admission] + `,
			"background":true,
			"validationFailureAction":"Enforce",
			"rules":[{
				"name":"require-label",
				"match":{"any":[{"resources":{"kinds":["Pod"]}}]},
				"validate":{"message":"label required","pattern":{"metadata":{"labels":{"app":"?*"}}}}
			}]
		}
	}`
	if err := json.Unmarshal([]byte(raw), &policy); err != nil {
		t.Fatalf("unmarshal cluster policy %q: %v", name, err)
	}
	if len(matchConditions) > 0 {
		policy.Spec.WebhookConfiguration = &kyvernov1.WebhookConfiguration{
			MatchConditions: matchConditions,
		}
	}
	policy.Status.SetReady(ready, "test")
	return &policy
}

func TestWarmUpRespectsAdmissionAndCustomWebhookMatchConditions(t *testing.T) {
	podGVR := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}

	disabledReady := testClusterPolicy(t, "00-disabled-ready", false, true, nil)
	customMatchCondReady := testClusterPolicy(t, "01-custom-match-cond-ready", true, true, []admissionregistrationv1.MatchCondition{
		{Name: "only-prod", Expression: "object.metadata.Namespace == 'prod'"},
	})
	enabledNotReady := testClusterPolicy(t, "02-enabled-not-ready", true, false, nil)
	enabledReadyA := testClusterPolicy(t, "03-enabled-ready-a", true, true, nil)
	enabledReadyB := testClusterPolicy(t, "04-enabled-ready-b", true, true, nil)

	cpols := []*kyvernov1.ClusterPolicy{
		disabledReady,
		customMatchCondReady,
		enabledNotReady,
		enabledReadyA,
		enabledReadyB,
	}

	clusterIndex := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, cp := range cpols {
		if err := clusterIndex.Add(cp); err != nil {
			t.Fatalf("add cluster policy %q: %v", cp.Name, err)
		}
	}
	nsIndex := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	client := dclient.NewEmptyFakeClient()
	client.SetDiscovery(dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{podGVR}))

	policyCache := pcache.NewCache()
	ctrl := &controller{
		cache:      policyCache,
		cpolLister: kyvernov1listers.NewClusterPolicyLister(clusterIndex),
		polLister:  kyvernov1listers.NewPolicyLister(nsIndex),
		client:     client,
	}

	if err := ctrl.WarmUp(); err != nil {
		t.Fatalf("WarmUp failed: %v", err)
	}

	got := policyCache.GetPolicies(pcache.ValidateEnforce, podGVR, "", ns)
	gotNames := map[string]bool{}
	for _, p := range got {
		gotNames[p.GetName()] = true
	}
	if len(gotNames) != 2 || !gotNames["03-enabled-ready-a"] || !gotNames["04-enabled-ready-b"] {
		t.Fatalf("WarmUp cached unexpected policies: %#v", gotNames)
	}

	reconcileCache := pcache.NewCache()
	reconcileCtrl := &controller{
		cache:      reconcileCache,
		cpolLister: kyvernov1listers.NewClusterPolicyLister(clusterIndex),
		polLister:  kyvernov1listers.NewPolicyLister(nsIndex),
		client:     client,
	}
	for _, cp := range cpols {
		if err := reconcileCtrl.reconcile(context.Background(), logr.Discard(), cp.Name, "", cp.Name); err != nil {
			t.Fatalf("reconcile %q failed: %v", cp.Name, err)
		}
	}
	reconcileGot := reconcileCache.GetPolicies(pcache.ValidateEnforce, podGVR, "", ns)
	if len(reconcileGot) != len(got) {
		t.Fatalf("WarmUp and reconcile diverged: warmup=%d reconcile=%d", len(got), len(reconcileGot))
	}
}
