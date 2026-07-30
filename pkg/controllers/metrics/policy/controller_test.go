package policy

import (
	"context"
	"testing"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	kyvernoconfig "github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/metrics"
	policyChangesMetric "github.com/kyverno/kyverno/pkg/metrics/policychanges"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mockPolicyRuleMetrics struct {
}

func (m *mockPolicyRuleMetrics) RecordPolicyRuleInfo(ctx context.Context, policy kyvernov1.PolicyInterface, observer metric.Observer) error {
	return nil
}

func (m *mockPolicyRuleMetrics) RegisterCallback(f metric.Callback) (metric.Registration, error) {
	return nil, nil
}

type mockMetricsManager struct {
	metrics.MetricsConfigManager
}

func (m *mockMetricsManager) Config() kyvernoconfig.MetricsConfiguration {
	return kyvernoconfig.NewDefaultMetricsConfiguration()
}

func (m *mockMetricsManager) RecordPolicyChanges(ctx context.Context, policyValidationMode metrics.PolicyValidationMode, policyType metrics.PolicyType, policyBackgroundMode metrics.PolicyBackgroundMode, policyNamespace string, policyName string, policyChangeType string) {
}

func (m *mockMetricsManager) PolicyRuleMetrics() metrics.PolicyRuleMetrics {
	return &mockPolicyRuleMetrics{}
}

func TestController(t *testing.T) {
	metrics.SetManager(&mockMetricsManager{})

	client := fake.NewSimpleClientset()
	informerFactory := kyvernov1informers.NewSharedInformerFactory(client, time.Minute)

	cpolInformer := informerFactory.Kyverno().V1().ClusterPolicies()
	polInformer := informerFactory.Kyverno().V1().Policies()

	ctrl := NewController(cpolInformer, polInformer)
	assert.NotNil(t, ctrl)

	c := ctrl.(*controller)

	// test addPolicy
	p := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cpol"},
	}
	c.addPolicy(p)

	event, quit := c.queue.Get()
	assert.False(t, quit)
	assert.Equal(t, p, event.cur)
	assert.Equal(t, policyChangesMetric.PolicyCreated, event.eventType)
	c.queue.Done(event)

	// test updatePolicy
	p2 := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cpol", ResourceVersion: "2"},
	}
	c.updatePolicy(p, p2)
	event, _ = c.queue.Get()
	assert.Equal(t, p, event.old)
	assert.Equal(t, p2, event.cur)
	assert.Equal(t, policyChangesMetric.PolicyUpdated, event.eventType)
	c.queue.Done(event)

	// test deletePolicy
	c.deletePolicy(p2)
	event, _ = c.queue.Get()
	assert.Equal(t, p2, event.cur)
	assert.Equal(t, policyChangesMetric.PolicyDeleted, event.eventType)
	c.queue.Done(event)

	// test addNsPolicy
	nsp := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pol", Namespace: "default"},
	}
	c.addNsPolicy(nsp)
	event, _ = c.queue.Get()
	assert.Equal(t, nsp, event.cur)
	assert.Equal(t, policyChangesMetric.PolicyCreated, event.eventType)
	c.queue.Done(event)

	// test updateNsPolicy
	nsp2 := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pol", Namespace: "default", ResourceVersion: "2"},
	}
	c.updateNsPolicy(nsp, nsp2)
	event, _ = c.queue.Get()
	assert.Equal(t, nsp, event.old)
	assert.Equal(t, nsp2, event.cur)
	assert.Equal(t, policyChangesMetric.PolicyUpdated, event.eventType)
	c.queue.Done(event)

	// test deleteNsPolicy
	c.deleteNsPolicy(nsp2)
	event, _ = c.queue.Get()
	assert.Equal(t, nsp2, event.cur)
	assert.Equal(t, policyChangesMetric.PolicyDeleted, event.eventType)
	c.queue.Done(event)

	// processNextWorkItem
	c.queue.Add(event)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Should process the queued item and return true
	res := c.processNextWorkItem(ctx)
	assert.True(t, res)

	c.queue.ShutDown()
	res = c.processNextWorkItem(ctx)
	assert.False(t, res)
}

func TestReport(t *testing.T) {
	client := fake.NewSimpleClientset()
	informerFactory := kyvernov1informers.NewSharedInformerFactory(client, time.Minute)

	cpolInformer := informerFactory.Kyverno().V1().ClusterPolicies()
	polInformer := informerFactory.Kyverno().V1().Policies()

	ctrl := NewController(cpolInformer, polInformer)
	c := ctrl.(*controller)

	// We just ensure report doesn't panic. Detailed metric reporting is tested in metrics pkg.
	err := c.report(context.Background(), nil)
	assert.NoError(t, err)
}
