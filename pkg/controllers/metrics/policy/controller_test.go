package policy

import (
	"context"
	"fmt"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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
	err      error
	recorded []kyvernov1.PolicyInterface
}

func (m *mockPolicyRuleMetrics) RecordPolicyRuleInfo(ctx context.Context, policy kyvernov1.PolicyInterface, observer metric.Observer) error {
	if m.err != nil {
		return m.err
	}
	m.recorded = append(m.recorded, policy)
	return nil
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
	expectedErr := fmt.Errorf("mock error")
	c := &controller{
		ruleInfo: &mockPolicyRuleMetrics{err: expectedErr},
		snapshot: newPolicySnapshot(),
	}
	c.snapshot.upsert(&kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
	})

	err := c.report(context.TODO(), &mockObserver{})
	assert.ErrorIs(t, err, expectedErr)
}

func Test_report_RecordsSnapshotPolicies(t *testing.T) {
	nsPolicy := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
	}
	clusterPolicy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster-policy"},
	}
	mockRuleInfo := &mockPolicyRuleMetrics{}
	c := &controller{
		ruleInfo: mockRuleInfo,
		snapshot: newPolicySnapshot(),
	}
	c.snapshot.upsert(clusterPolicy)
	c.snapshot.upsert(nsPolicy)

	err := c.report(context.TODO(), &mockObserver{})
	assert.NoError(t, err)
	assert.ElementsMatch(t, []kyvernov1.PolicyInterface{nsPolicy, clusterPolicy}, mockRuleInfo.recorded)
}

func Test_report_ExcludesDeletedPolicies(t *testing.T) {
	nsPolicy := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
	}
	clusterPolicy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster-policy"},
	}
	mockRuleInfo := &mockPolicyRuleMetrics{}
	c := &controller{
		ruleInfo: mockRuleInfo,
		snapshot: newPolicySnapshot(),
	}
	c.snapshot.upsert(clusterPolicy)
	c.snapshot.upsert(nsPolicy)
	c.snapshot.remove(cache.DeletedFinalStateUnknown{
		Key: "default/test-policy",
		Obj: nsPolicy,
	})

	err := c.report(context.TODO(), &mockObserver{})
	assert.NoError(t, err)
	assert.Equal(t, []kyvernov1.PolicyInterface{clusterPolicy}, mockRuleInfo.recorded)
}

func Test_snapshot_UpsertReplacesPreviousVersion(t *testing.T) {
	s := newPolicySnapshot()
	oldPolicy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
	}
	newPolicy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
	}

	s.upsert(oldPolicy)
	s.upsert(newPolicy)

	cpols, pols := s.list()
	assert.Equal(t, []kyvernov1.PolicyInterface{newPolicy}, cpols)
	assert.Empty(t, pols)
}

func Test_snapshot_RemoveByTombstoneWithoutObject(t *testing.T) {
	s := newPolicySnapshot()
	s.upsert(&kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
	})

	s.remove(cache.DeletedFinalStateUnknown{Key: "default/test-policy"})

	_, pols := s.list()
	assert.Empty(t, pols)
}

func Test_addUpdateDeletePolicy_SnapshotTransitions(t *testing.T) {
	wg := &wait.Group{}
	c := &controller{
		ruleInfo:  &mockPolicyRuleMetrics{},
		snapshot:  newPolicySnapshot(),
		waitGroup: wg,
	}

	p1 := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
	}
	p2 := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
	}

	// add then update keeps a single entry with the latest version
	c.addPolicy(p1)
	c.updatePolicy(p1, p2)
	wg.Wait()
	cpols, pols := c.snapshot.list()
	assert.Equal(t, []kyvernov1.PolicyInterface{p2}, cpols)
	assert.Empty(t, pols)

	// delete drops the entry, tombstone or not
	c.deletePolicy(p2)
	wg.Wait()
	cpols, pols = c.snapshot.list()
	assert.Empty(t, cpols)
	assert.Empty(t, pols)

	c.addPolicy(p1)
	c.deletePolicy(cache.DeletedFinalStateUnknown{Key: "test-policy", Obj: p1})
	wg.Wait()
	cpols, pols = c.snapshot.list()
	assert.Empty(t, cpols)
	assert.Empty(t, pols)
}

func Test_addUpdateDeleteNsPolicy_SnapshotTransitions(t *testing.T) {
	wg := &wait.Group{}
	c := &controller{
		ruleInfo:  &mockPolicyRuleMetrics{},
		snapshot:  newPolicySnapshot(),
		waitGroup: wg,
	}

	p1 := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
	}
	p2 := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
	}

	// add then update keeps a single entry with the latest version
	c.addNsPolicy(p1)
	c.updateNsPolicy(p1, p2)
	wg.Wait()
	cpols, pols := c.snapshot.list()
	assert.Empty(t, cpols)
	assert.Equal(t, []kyvernov1.PolicyInterface{p2}, pols)

	// delete drops the entry, tombstone or not
	c.deleteNsPolicy(p2)
	wg.Wait()
	cpols, pols = c.snapshot.list()
	assert.Empty(t, cpols)
	assert.Empty(t, pols)

	c.addNsPolicy(p1)
	c.deleteNsPolicy(cache.DeletedFinalStateUnknown{Key: "default/test-policy", Obj: p1})
	wg.Wait()
	cpols, pols = c.snapshot.list()
	assert.Empty(t, cpols)
	assert.Empty(t, pols)
}

func Test_deletePolicy_Tombstone(t *testing.T) {
	wg := &wait.Group{}
	c := &controller{
		snapshot:  newPolicySnapshot(),
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
		snapshot:  newPolicySnapshot(),
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
		snapshot:  newPolicySnapshot(),
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
		snapshot:  newPolicySnapshot(),
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
