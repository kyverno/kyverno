package policycache

import (
	"context"
	"encoding/json"
	"reflect"
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

// testClusterPolicy builds a ClusterPolicy fixture with configurable admission, readiness, and matchConditions.
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

// testNamespacedPolicy builds a namespaced Policy fixture with configurable admission, readiness, and matchConditions.
func testNamespacedPolicy(t *testing.T, namespace, name string, admission, ready bool, matchConditions []admissionregistrationv1.MatchCondition) *kyvernov1.Policy {
	t.Helper()
	var policy kyvernov1.Policy
	raw := `{
		"apiVersion":"kyverno.io/v1",
		"kind":"Policy",
		"metadata":{"namespace":"` + namespace + `","name":"` + name + `"},
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
		t.Fatalf("unmarshal namespaced policy %q/%q: %v", namespace, name, err)
	}
	if len(matchConditions) > 0 {
		policy.Spec.WebhookConfiguration = &kyvernov1.WebhookConfiguration{
			MatchConditions: matchConditions,
		}
	}
	policy.Status.SetReady(ready, "test")
	return &policy
}

// TestWarmUpRespectsAdmissionAndCustomWebhookMatchConditions verifies that WarmUp and reconcile
// apply identical eligibility filtering and evict previously cached policies that become ineligible.
func TestWarmUpRespectsAdmissionAndCustomWebhookMatchConditions(t *testing.T) {
	podGVR := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	matchCond := []admissionregistrationv1.MatchCondition{
		{Name: "only-prod", Expression: "object.metadata.Namespace == 'prod'"},
	}

	disabledReady := testClusterPolicy(t, "00-disabled-ready", false, true, nil)
	customMatchCondReady := testClusterPolicy(t, "01-custom-match-cond-ready", true, true, matchCond)
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

	nsDisabledReady := testNamespacedPolicy(t, ns.Name, "00-ns-disabled-ready", false, true, nil)
	nsCustomMatchCondReady := testNamespacedPolicy(t, ns.Name, "01-ns-custom-match-cond-ready", true, true, matchCond)
	nsEnabledNotReady := testNamespacedPolicy(t, ns.Name, "02-ns-enabled-not-ready", true, false, nil)
	nsEnabledReadyA := testNamespacedPolicy(t, ns.Name, "03-ns-enabled-ready-a", true, true, nil)
	nsEnabledReadyB := testNamespacedPolicy(t, ns.Name, "04-ns-enabled-ready-b", true, true, nil)

	pols := []*kyvernov1.Policy{
		nsDisabledReady,
		nsCustomMatchCondReady,
		nsEnabledNotReady,
		nsEnabledReadyA,
		nsEnabledReadyB,
	}

	clusterIndex := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, cp := range cpols {
		if err := clusterIndex.Add(cp); err != nil {
			t.Fatalf("add cluster policy %q: %v", cp.Name, err)
		}
	}
	nsIndex := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})
	for _, p := range pols {
		if err := nsIndex.Add(p); err != nil {
			t.Fatalf("add namespaced policy %q/%q: %v", p.Namespace, p.Name, err)
		}
	}

	client := dclient.NewEmptyFakeClient()
	client.SetDiscovery(dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{
		podGVR,
		{Version: "v1", Resource: "replicationcontrollers"},
		{Group: "apps", Version: "v1", Resource: "replicasets"},
		{Group: "batch", Version: "v1", Resource: "jobs"},
		{Group: "batch", Version: "v1", Resource: "cronjobs"},
	}))

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
	wantInitial := map[string]bool{
		"03-enabled-ready-a":    true,
		"04-enabled-ready-b":    true,
		"03-ns-enabled-ready-a": true,
		"04-ns-enabled-ready-b": true,
	}
	if !reflect.DeepEqual(gotNames, wantInitial) {
		t.Fatalf("WarmUp cached unexpected policies: got=%v want=%v", gotNames, wantInitial)
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
	for _, p := range pols {
		key := p.Namespace + "/" + p.Name
		if err := reconcileCtrl.reconcile(context.Background(), logr.Discard(), key, p.Namespace, p.Name); err != nil {
			t.Fatalf("reconcile %q failed: %v", key, err)
		}
	}
	reconcileGot := reconcileCache.GetPolicies(pcache.ValidateEnforce, podGVR, "", ns)
	reconcileNames := map[string]bool{}
	for _, p := range reconcileGot {
		reconcileNames[p.GetName()] = true
	}
	if !reflect.DeepEqual(gotNames, reconcileNames) {
		t.Fatalf("WarmUp and reconcile diverged: warmup=%v reconcile=%v", gotNames, reconcileNames)
	}

	// Transition previously cached policies to ineligible states and verify Unset removes them.
	admissionFalse := false
	enabledReadyB.Spec.Admission = &admissionFalse
	if err := clusterIndex.Update(enabledReadyB); err != nil {
		t.Fatalf("update cluster policy %q: %v", enabledReadyB.Name, err)
	}
	nsEnabledReadyB.Status.SetReady(false, "not-ready")
	if err := nsIndex.Update(nsEnabledReadyB); err != nil {
		t.Fatalf("update namespaced policy %q/%q: %v", nsEnabledReadyB.Namespace, nsEnabledReadyB.Name, err)
	}

	if err := ctrl.WarmUp(); err != nil {
		t.Fatalf("second WarmUp failed: %v", err)
	}
	if err := reconcileCtrl.reconcile(context.Background(), logr.Discard(), enabledReadyB.Name, "", enabledReadyB.Name); err != nil {
		t.Fatalf("reconcile updated %q failed: %v", enabledReadyB.Name, err)
	}
	nsKeyB := nsEnabledReadyB.Namespace + "/" + nsEnabledReadyB.Name
	if err := reconcileCtrl.reconcile(context.Background(), logr.Discard(), nsKeyB, nsEnabledReadyB.Namespace, nsEnabledReadyB.Name); err != nil {
		t.Fatalf("reconcile updated %q failed: %v", nsKeyB, err)
	}

	afterWarmUpNames := map[string]bool{}
	for _, p := range policyCache.GetPolicies(pcache.ValidateEnforce, podGVR, "", ns) {
		afterWarmUpNames[p.GetName()] = true
	}
	afterReconcileNames := map[string]bool{}
	for _, p := range reconcileCache.GetPolicies(pcache.ValidateEnforce, podGVR, "", ns) {
		afterReconcileNames[p.GetName()] = true
	}
	wantAfterEviction := map[string]bool{
		"03-enabled-ready-a":    true,
		"03-ns-enabled-ready-a": true,
	}
	if !reflect.DeepEqual(afterWarmUpNames, wantAfterEviction) {
		t.Fatalf("WarmUp did not evict ineligible policies: got=%v want=%v", afterWarmUpNames, wantAfterEviction)
	}
	if !reflect.DeepEqual(afterWarmUpNames, afterReconcileNames) {
		t.Fatalf("WarmUp and reconcile diverged after eviction: warmup=%v reconcile=%v", afterWarmUpNames, afterReconcileNames)
	}
}
