package policy

import (
	"context"
	"fmt"
	"slices"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"go.opentelemetry.io/otel/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type fakePolicyRuleMetrics struct {
	recorded []string
}

func (f *fakePolicyRuleMetrics) RecordPolicyRuleInfo(_ context.Context, policy kyvernov1.PolicyInterface, _ metric.Observer) error {
	f.recorded = append(f.recorded, policyKey(policy))
	return nil
}

func (f *fakePolicyRuleMetrics) RegisterCallback(metric.Callback) (metric.Registration, error) {
	return nil, nil
}

type fakeClusterPolicyLister struct {
	policies  []*kyvernov1.ClusterPolicy
	listCalls int
}

func (f *fakeClusterPolicyLister) List(labels.Selector) ([]*kyvernov1.ClusterPolicy, error) {
	f.listCalls++
	return f.policies, nil
}

func (f *fakeClusterPolicyLister) Get(name string) (*kyvernov1.ClusterPolicy, error) {
	for _, policy := range f.policies {
		if policy.GetName() == name {
			return policy, nil
		}
	}
	return nil, fmt.Errorf("clusterpolicy %s not found", name)
}

type fakePolicyLister struct {
	policies  []*kyvernov1.Policy
	listCalls int
}

func (f *fakePolicyLister) List(labels.Selector) ([]*kyvernov1.Policy, error) {
	return f.policies, nil
}

func (f *fakePolicyLister) Policies(namespace string) kyvernov1listers.PolicyNamespaceLister {
	return &fakePolicyNamespaceLister{parent: f, namespace: namespace}
}

type fakePolicyNamespaceLister struct {
	parent    *fakePolicyLister
	namespace string
}

func (f *fakePolicyNamespaceLister) List(labels.Selector) ([]*kyvernov1.Policy, error) {
	f.parent.listCalls++
	if f.namespace == metav1.NamespaceAll {
		return f.parent.policies, nil
	}

	filtered := make([]*kyvernov1.Policy, 0, len(f.parent.policies))
	for _, policy := range f.parent.policies {
		if policy.GetNamespace() == f.namespace {
			filtered = append(filtered, policy)
		}
	}
	return filtered, nil
}

func (f *fakePolicyNamespaceLister) Get(name string) (*kyvernov1.Policy, error) {
	for _, policy := range f.parent.policies {
		if policy.GetNamespace() == f.namespace && policy.GetName() == name {
			return policy, nil
		}
	}
	return nil, fmt.Errorf("policy %s/%s not found", f.namespace, name)
}

func TestReportInitializesPolicyCacheOnce(t *testing.T) {
	ruleInfo := &fakePolicyRuleMetrics{}
	polLister := &fakePolicyLister{
		policies: []*kyvernov1.Policy{
			newPolicy("default", "ns-policy", "v1"),
		},
	}
	cpolLister := &fakeClusterPolicyLister{
		policies: []*kyvernov1.ClusterPolicy{
			newClusterPolicy("cluster-policy", "v1"),
		},
	}
	c := controller{
		ruleInfo:   ruleInfo,
		polLister:  polLister,
		cpolLister: cpolLister,
		policies:   map[string]kyvernov1.PolicyInterface{},
	}

	if err := c.report(context.Background(), nil); err != nil {
		t.Fatalf("report returned error: %v", err)
	}
	if err := c.report(context.Background(), nil); err != nil {
		t.Fatalf("second report returned error: %v", err)
	}

	if polLister.listCalls != 1 {
		t.Fatalf("expected policy lister to be called once, got %d", polLister.listCalls)
	}
	if cpolLister.listCalls != 1 {
		t.Fatalf("expected cluster policy lister to be called once, got %d", cpolLister.listCalls)
	}
	if len(ruleInfo.recorded) != 4 {
		t.Fatalf("expected 4 metric records across two scrapes, got %d", len(ruleInfo.recorded))
	}
}

func TestPolicyCacheTracksAddUpdateDelete(t *testing.T) {
	c := controller{
		ruleInfo:         &fakePolicyRuleMetrics{},
		policies:         map[string]kyvernov1.PolicyInterface{},
		cacheInitialized: true,
	}

	initial := newPolicy("default", "ns-policy", "v1")
	cluster := newClusterPolicy("cluster-policy", "v1")
	c.storePolicy(initial)
	c.storePolicy(cluster)

	snapshot := sortedPolicyVersions(c.policySnapshot())
	expected := []string{"-/cluster-policy=v1", "default/ns-policy=v1"}
	if !slices.Equal(snapshot, expected) {
		t.Fatalf("unexpected initial snapshot: got %v want %v", snapshot, expected)
	}

	updated := newPolicy("default", "ns-policy", "v2")
	c.storePolicy(updated)
	snapshot = sortedPolicyVersions(c.policySnapshot())
	expected = []string{"-/cluster-policy=v1", "default/ns-policy=v2"}
	if !slices.Equal(snapshot, expected) {
		t.Fatalf("unexpected updated snapshot: got %v want %v", snapshot, expected)
	}

	c.deletePolicyFromCache(updated)
	snapshot = sortedPolicyVersions(c.policySnapshot())
	expected = []string{"-/cluster-policy=v1"}
	if !slices.Equal(snapshot, expected) {
		t.Fatalf("unexpected snapshot after delete: got %v want %v", snapshot, expected)
	}
}

func sortedPolicyVersions(policies []kyvernov1.PolicyInterface) []string {
	versions := make([]string, 0, len(policies))
	for _, policy := range policies {
		namespace := policy.GetNamespace()
		if namespace == "" {
			namespace = "-"
		}
		versions = append(versions, fmt.Sprintf("%s/%s=%s", namespace, policy.GetName(), policy.GetLabels()["version"]))
	}
	slices.Sort(versions)
	return versions
}

func newPolicy(namespace, name, version string) *kyvernov1.Policy {
	return &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels: map[string]string{
				"version": version,
			},
		},
	}
}

func newClusterPolicy(name, version string) *kyvernov1.ClusterPolicy {
	return &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"version": version,
			},
		},
	}
}
