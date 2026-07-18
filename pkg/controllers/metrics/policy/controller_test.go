package policy

import (
	"context"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"go.opentelemetry.io/otel/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

// fakePolicyRuleMetrics is a no-op implementation of PolicyRuleMetrics for testing.
type fakePolicyRuleMetrics struct {
	recorded []kyvernov1.PolicyInterface
}

func (f *fakePolicyRuleMetrics) RecordPolicyRuleInfo(_ context.Context, policy kyvernov1.PolicyInterface, _ metric.Observer) error {
	f.recorded = append(f.recorded, policy)
	return nil
}

func (f *fakePolicyRuleMetrics) RegisterCallback(_ metric.Callback) (metric.Registration, error) {
	return nil, nil
}

func makeClusterPolicy(name string, uid types.UID) *kyvernov1.ClusterPolicy {
	return &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  uid,
		},
	}
}

func makePolicy(namespace, name string, uid types.UID) *kyvernov1.Policy {
	return &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			UID:       uid,
		},
	}
}

func newTestController(fake *fakePolicyRuleMetrics) *controller {
	return &controller{
		ruleInfo:  fake,
		policies:  make(map[types.UID]kyvernov1.PolicyInterface),
		waitGroup: &wait.Group{},
	}
}

func TestControllerCacheAdd(t *testing.T) {
	fake := &fakePolicyRuleMetrics{}
	c := newTestController(fake)

	cpol := makeClusterPolicy("policy-a", "uid-1")
	c.addPolicy(cpol)
	pol := makePolicy("ns", "policy-b", "uid-2")
	c.addNsPolicy(pol)

	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.policies) != 2 {
		t.Fatalf("expected 2 cached policies, got %d", len(c.policies))
	}
}

func TestControllerCacheUpdate(t *testing.T) {
	fake := &fakePolicyRuleMetrics{}
	c := newTestController(fake)

	orig := makeClusterPolicy("policy-a", "uid-1")
	c.addPolicy(orig)

	updated := makeClusterPolicy("policy-a-updated", "uid-1")
	c.updatePolicy(orig, updated)

	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.policies) != 1 {
		t.Fatalf("expected 1 cached policy after update, got %d", len(c.policies))
	}
	if c.policies["uid-1"].GetName() != "policy-a-updated" {
		t.Errorf("expected updated policy name, got %q", c.policies["uid-1"].GetName())
	}
}

func TestControllerCacheDelete(t *testing.T) {
	fake := &fakePolicyRuleMetrics{}
	c := newTestController(fake)

	cpol := makeClusterPolicy("policy-a", "uid-1")
	c.addPolicy(cpol)
	pol := makePolicy("ns", "policy-b", "uid-2")
	c.addNsPolicy(pol)

	c.deletePolicy(cpol)

	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.policies) != 1 {
		t.Fatalf("expected 1 cached policy after delete, got %d", len(c.policies))
	}
	if _, exists := c.policies["uid-1"]; exists {
		t.Error("deleted ClusterPolicy should not be in cache")
	}
}

func TestControllerReportUsesCache(t *testing.T) {
	fake := &fakePolicyRuleMetrics{}
	c := newTestController(fake)

	c.addPolicy(makeClusterPolicy("cpol-1", "uid-1"))
	c.addNsPolicy(makePolicy("ns", "pol-1", "uid-2"))

	if err := c.report(context.Background(), nil); err != nil {
		t.Fatalf("report() error: %v", err)
	}

	if len(fake.recorded) != 2 {
		t.Errorf("expected 2 recorded policies, got %d", len(fake.recorded))
	}
}

func TestControllerReportAfterDelete(t *testing.T) {
	fake := &fakePolicyRuleMetrics{}
	c := newTestController(fake)

	cpol := makeClusterPolicy("cpol-1", "uid-1")
	c.addPolicy(cpol)
	c.addNsPolicy(makePolicy("ns", "pol-1", "uid-2"))
	c.deletePolicy(cpol)

	if err := c.report(context.Background(), nil); err != nil {
		t.Fatalf("report() error: %v", err)
	}

	if len(fake.recorded) != 1 {
		t.Errorf("expected 1 recorded policy after delete, got %d", len(fake.recorded))
	}
}
