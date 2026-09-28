package policy

import (
	"context"
	"fmt"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

func init() {
	metrics.SetManager(metrics.NewFakeMetricsConfig())
}

type mockObserver struct {
	metric.Observer
}

type mockPolicyRuleMetrics struct {
	err error
}

func (m *mockPolicyRuleMetrics) RecordPolicyRuleInfo(ctx context.Context, policy kyvernov1.PolicyInterface, observer metric.Observer) error {
	return m.err
}

func (m *mockPolicyRuleMetrics) RegisterCallback(f metric.Callback) (metric.Registration, error) {
	return nil, nil
}

type observableMetricsManager struct {
	metrics.MetricsConfigManager
	recordedCalls int
}

func (m *observableMetricsManager) RecordPolicyChanges(ctx context.Context, policyValidationMode metrics.PolicyValidationMode, policyType metrics.PolicyType, policyBackgroundMode metrics.PolicyBackgroundMode, policyNamespace string, policyName string, policyChangeType string) {
	m.recordedCalls++
}

func Test_report_NilRuleInfo(t *testing.T) {
	c := &controller{
		ruleInfo: nil,
	}
	err := c.report(context.TODO(), &mockObserver{})
	assert.NoError(t, err)
}

func Test_report_ErrorPropagation(t *testing.T) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	indexer.Add(&kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
	})

	polLister := kyvernov1listers.NewPolicyLister(indexer)
	cpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	cpolLister := kyvernov1listers.NewClusterPolicyLister(cpolIndexer)

	expectedErr := fmt.Errorf("mock error")
	c := &controller{
		ruleInfo:   &mockPolicyRuleMetrics{err: expectedErr},
		polLister:  polLister,
		cpolLister: cpolLister,
	}

	err := c.report(context.TODO(), &mockObserver{})
	assert.ErrorIs(t, err, expectedErr)
}

func Test_deletePolicy_Tombstone(t *testing.T) {
	wg := &wait.Group{}
	c := &controller{
		waitGroup: wg,
	}

	p := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
	}
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "test-policy",
		Obj: p,
	}

	mockManager := &observableMetricsManager{MetricsConfigManager: metrics.NewFakeMetricsConfig()}
	metrics.SetManager(mockManager)
	defer metrics.SetManager(metrics.NewFakeMetricsConfig())

	assert.NotPanics(t, func() {
		c.deletePolicy(tombstone)
	})

	wg.Wait()
	assert.Equal(t, 1, mockManager.recordedCalls, "expected delete metric to be registered")
}

func Test_deleteNsPolicy_Tombstone(t *testing.T) {
	wg := &wait.Group{}
	c := &controller{
		waitGroup: wg,
	}

	p := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ns-policy",
			Namespace: "default",
		},
	}
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "default/test-ns-policy",
		Obj: p,
	}

	mockManager := &observableMetricsManager{MetricsConfigManager: metrics.NewFakeMetricsConfig()}
	metrics.SetManager(mockManager)
	defer metrics.SetManager(metrics.NewFakeMetricsConfig())

	assert.NotPanics(t, func() {
		c.deleteNsPolicy(tombstone)
	})

	wg.Wait()
	assert.Equal(t, 1, mockManager.recordedCalls, "expected delete metric to be registered")
}

func Test_updatePolicy_SameSpec(t *testing.T) {
	wg := &wait.Group{}
	c := &controller{
		waitGroup: wg,
	}

	p1 := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: kyvernov1.Spec{
			Background: &[]bool{true}[0],
		},
	}

	p2 := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: kyvernov1.Spec{
			Background: &[]bool{true}[0],
		},
	}

	mockManager := &observableMetricsManager{MetricsConfigManager: metrics.NewFakeMetricsConfig()}
	metrics.SetManager(mockManager)
	defer metrics.SetManager(metrics.NewFakeMetricsConfig())

	assert.NotPanics(t, func() {
		c.updatePolicy(p1, p2)
	})

	wg.Wait()
	assert.Equal(t, 0, mockManager.recordedCalls, "expected no metrics update for equal spec")
}

func Test_updateNsPolicy_SameSpec(t *testing.T) {
	wg := &wait.Group{}
	c := &controller{
		waitGroup: wg,
	}

	p1 := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: kyvernov1.Spec{
			Background: &[]bool{true}[0],
		},
	}

	p2 := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: kyvernov1.Spec{
			Background: &[]bool{true}[0],
		},
	}

	mockManager := &observableMetricsManager{MetricsConfigManager: metrics.NewFakeMetricsConfig()}
	metrics.SetManager(mockManager)
	defer metrics.SetManager(metrics.NewFakeMetricsConfig())

	assert.NotPanics(t, func() {
		c.updateNsPolicy(p1, p2)
	})

	wg.Wait()
	assert.Equal(t, 0, mockManager.recordedCalls, "expected no metrics update for equal spec")
}
