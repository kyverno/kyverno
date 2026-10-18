package policy

import (
	"context"
	"sync"
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
	built    []string
	replayed []metrics.PolicyRuleObservation
}

func (m *mockPolicyRuleMetrics) BuildPolicyRuleObservations(policy kyvernov1.PolicyInterface) (metrics.PolicyRuleObservation, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.built = append(m.built, policy.GetName())
	// each build returns an observation of a different length, so a test can
	// tell whether a later build actually replaced the stored one
	return make(metrics.PolicyRuleObservation, len(m.built)), nil
}

func (m *mockPolicyRuleMetrics) RecordPolicyRuleObservations(obs metrics.PolicyRuleObservation, observer metric.Observer) {
	m.replayed = append(m.replayed, obs)
}

func (m *mockPolicyRuleMetrics) RegisterCallback(f metric.Callback) (metric.Registration, error) {
	return nil, nil
}

type observableMetricsManager struct {
	metrics.MetricsConfigManager
	mux           sync.Mutex
	recordedCalls int
}

func (m *observableMetricsManager) RecordPolicyChanges(ctx context.Context, policyValidationMode metrics.PolicyValidationMode, policyType metrics.PolicyType, policyBackgroundMode metrics.PolicyBackgroundMode, policyNamespace string, policyName string, policyChangeType string) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.recordedCalls++
}

func Test_report_NilRuleInfo(t *testing.T) {
	c := &controller{
		ruleInfo: nil,
	}
	err := c.report(context.TODO(), &mockObserver{})
	assert.NoError(t, err)
}

func Test_report_ReplaysSnapshotObservations(t *testing.T) {
	mockManager := &observableMetricsManager{MetricsConfigManager: metrics.NewFakeMetricsConfig()}
	metrics.SetManager(mockManager)
	defer metrics.SetManager(metrics.NewFakeMetricsConfig())

	mockRuleInfo := &mockPolicyRuleMetrics{}
	wg := &wait.Group{}
	c := &controller{
		ruleInfo:  mockRuleInfo,
		snapshot:  newPolicySnapshot(),
		waitGroup: wg,
	}
	c.addPolicy(&kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster-policy"},
	})
	c.addNsPolicy(&kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
	})
	wg.Wait()

	err := c.report(context.TODO(), &mockObserver{})
	assert.NoError(t, err)
	assert.Equal(t, []string{"test-cluster-policy", "test-policy"}, mockRuleInfo.built)
	assert.Len(t, mockRuleInfo.replayed, 2)
}

func Test_report_SkipsFailedObservationBuilds(t *testing.T) {
	mockManager := &observableMetricsManager{MetricsConfigManager: metrics.NewFakeMetricsConfig()}
	metrics.SetManager(mockManager)
	defer metrics.SetManager(metrics.NewFakeMetricsConfig())

	mockRuleInfo := &mockPolicyRuleMetrics{err: assert.AnError}
	wg := &wait.Group{}
	c := &controller{
		ruleInfo:  mockRuleInfo,
		snapshot:  newPolicySnapshot(),
		waitGroup: wg,
	}
	c.addPolicy(&kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster-policy"},
	})
	wg.Wait()

	// the failed build is not kept in the snapshot, so the scrape replays nothing
	err := c.report(context.TODO(), &mockObserver{})
	assert.NoError(t, err)
	assert.Empty(t, mockRuleInfo.replayed)
}

func Test_BuildPolicyRuleObservations(t *testing.T) {
	ruleInfo := metrics.GetPolicyInfoMetrics()
	obs, err := ruleInfo.BuildPolicyRuleObservations(&kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster-policy"},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{{
				Name: "test-rule",
				MatchResources: kyvernov1.MatchResources{
					Any: []kyvernov1.ResourceFilter{{
						ResourceDescription: kyvernov1.ResourceDescription{
							Kinds: []string{"ConfigMap"},
						},
					}},
				},
				Validation: &kyvernov1.Validation{
					Message: "test",
				},
			}},
		},
	})
	assert.NoError(t, err)
	assert.Len(t, obs, 1)
}

func Test_addUpdateDeletePolicy_SnapshotTransitions(t *testing.T) {
	mockManager := &observableMetricsManager{MetricsConfigManager: metrics.NewFakeMetricsConfig()}
	metrics.SetManager(mockManager)
	defer metrics.SetManager(metrics.NewFakeMetricsConfig())

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

	// add then update keeps a single entry, rebuilt for the latest version
	c.addPolicy(p1)
	c.updatePolicy(p1, p2)
	wg.Wait()
	cpols, pols := c.snapshot.list()
	assert.Len(t, cpols, 1)
	assert.Len(t, cpols[0], 2, "expected the update to replace the stored observation")
	assert.Empty(t, pols)

	// delete drops the entry, tombstone or not
	c.deletePolicy(p2)
	wg.Wait()
	cpols, pols = c.snapshot.list()
	assert.Empty(t, cpols)
	assert.Empty(t, pols)

	// a tombstone without the object still identifies the policy by its key
	c.addPolicy(p1)
	c.deletePolicy(cache.DeletedFinalStateUnknown{Key: "test-policy"})
	wg.Wait()
	cpols, pols = c.snapshot.list()
	assert.Empty(t, cpols)
	assert.Empty(t, pols)
}

func Test_addUpdateDeleteNsPolicy_SnapshotTransitions(t *testing.T) {
	mockManager := &observableMetricsManager{MetricsConfigManager: metrics.NewFakeMetricsConfig()}
	metrics.SetManager(mockManager)
	defer metrics.SetManager(metrics.NewFakeMetricsConfig())

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

	// add then update keeps a single entry, rebuilt for the latest version
	c.addNsPolicy(p1)
	c.updateNsPolicy(p1, p2)
	wg.Wait()
	cpols, pols := c.snapshot.list()
	assert.Empty(t, cpols)
	assert.Len(t, pols, 1)
	assert.Len(t, pols[0], 2, "expected the update to replace the stored observation")

	// delete drops the entry, tombstone or not
	c.deleteNsPolicy(p2)
	wg.Wait()
	cpols, pols = c.snapshot.list()
	assert.Empty(t, cpols)
	assert.Empty(t, pols)

	// a tombstone without the object still identifies the policy by its key
	c.addNsPolicy(p1)
	c.deleteNsPolicy(cache.DeletedFinalStateUnknown{Key: "default/test-policy"})
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
