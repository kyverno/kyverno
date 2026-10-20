package policy

import (
	"context"
	"sync"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	corev1 "k8s.io/api/core/v1"
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

func snapshotEntries(s *policySnapshot) []policyObservation {
	var entries []policyObservation
	s.rangeObservations(func(obs policyObservation) {
		entries = append(entries, obs)
	})
	return entries
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

func Test_report_AppliesCurrentNamespaceFilter(t *testing.T) {
	manager := &observableMetricsManager{MetricsConfigManager: metrics.NewFakeMetricsConfig()}
	metrics.SetManager(manager)
	defer metrics.SetManager(metrics.NewFakeMetricsConfig())

	mockRuleInfo := &mockPolicyRuleMetrics{}
	wg := &wait.Group{}
	c := &controller{
		ruleInfo:  mockRuleInfo,
		snapshot:  newPolicySnapshot(),
		waitGroup: wg,
	}
	c.addNsPolicy(&kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "included", Namespace: "default"},
	})
	c.addNsPolicy(&kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "excluded", Namespace: "other"},
	})
	wg.Wait()

	// excluding a namespace after the observations were built must hide them on
	// the next report, without waiting for another policy event
	manager.Config().Load(&corev1.ConfigMap{Data: map[string]string{
		"namespaces": `{"exclude":["default"]}`,
	}})
	mockRuleInfo.replayed = nil
	assert.NoError(t, c.report(context.TODO(), &mockObserver{}))
	assert.Len(t, mockRuleInfo.replayed, 1)
	assert.Len(t, mockRuleInfo.replayed[0], 2, "expected only the observation of the namespace still included")

	// and including it again must bring the stored observation back
	manager.Config().Load(&corev1.ConfigMap{Data: map[string]string{
		"namespaces": `{"exclude":["other"]}`,
	}})
	mockRuleInfo.replayed = nil
	assert.NoError(t, c.report(context.TODO(), &mockObserver{}))
	assert.Len(t, mockRuleInfo.replayed, 1)
	assert.Len(t, mockRuleInfo.replayed[0], 1, "expected only the observation of the namespace still included")
}

type policyRuleSample struct {
	value      float64
	attributes map[string]any
}

// collectPolicyRuleObservations replays observations through a real SDK
// provider and returns the data points Prometheus would scrape, so tests can
// assert the metric output (value and attributes) rather than the internal
// observe options.
func collectPolicyRuleObservations(t *testing.T, obs metrics.PolicyRuleObservation) []policyRuleSample {
	t.Helper()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		otel.SetMeterProvider(previous)
		_ = provider.Shutdown(context.Background())
	})

	ruleInfo := metrics.NewFakeMetricsConfig().PolicyRuleMetrics()
	_, err := ruleInfo.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		ruleInfo.RecordPolicyRuleObservations(obs, observer)
		return nil
	})
	assert.NoError(t, err)

	var data metricdata.ResourceMetrics
	err = reader.Collect(context.Background(), &data)
	assert.NoError(t, err)

	var samples []policyRuleSample
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != "kyverno_policy_rule_info_total" {
				continue
			}
			gauge, ok := m.Data.(metricdata.Gauge[float64])
			if !assert.True(t, ok, "expected %s to be a float64 gauge", m.Name) {
				continue
			}
			for _, point := range gauge.DataPoints {
				attributes := make(map[string]any, point.Attributes.Len())
				for _, kv := range point.Attributes.ToSlice() {
					attributes[string(kv.Key)] = kv.Value.AsInterface()
				}
				samples = append(samples, policyRuleSample{value: point.Value, attributes: attributes})
			}
		}
	}
	return samples
}

func Test_updatePolicy_FailedRebuildDropsEntry(t *testing.T) {
	manager := &observableMetricsManager{MetricsConfigManager: metrics.NewFakeMetricsConfig()}
	metrics.SetManager(manager)
	defer metrics.SetManager(metrics.NewFakeMetricsConfig())

	mockRuleInfo := &mockPolicyRuleMetrics{}
	wg := &wait.Group{}
	c := &controller{
		ruleInfo:  mockRuleInfo,
		snapshot:  newPolicySnapshot(),
		waitGroup: wg,
	}
	p := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
	}

	c.addPolicy(p)
	mockRuleInfo.err = assert.AnError
	c.updatePolicy(p, p)
	wg.Wait()

	// a rebuild failure must drop the entry stored by the earlier successful add
	assert.Empty(t, snapshotEntries(c.snapshot))
}

func Test_BuildPolicyRuleObservations(t *testing.T) {
	matchConfigMaps := kyvernov1.MatchResources{
		Any: []kyvernov1.ResourceFilter{{
			ResourceDescription: kyvernov1.ResourceDescription{
				Kinds: []string{"ConfigMap"},
			},
		}},
	}

	tests := []struct {
		name   string
		policy kyvernov1.PolicyInterface
		want   map[string]any
	}{
		{
			name: "cluster policy",
			policy: &kyvernov1.ClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster-policy"},
				Spec: kyvernov1.Spec{
					Rules: []kyvernov1.Rule{{
						Name:           "test-rule",
						MatchResources: matchConfigMaps,
						Validation: &kyvernov1.Validation{
							Message: "test",
						},
					}},
				},
			},
			want: map[string]any{
				"policy_namespace":       "-",
				"policy_name":            "test-cluster-policy",
				"policy_type":            "cluster",
				"status_ready":           false,
				"policy_validation_mode": "audit",
				"policy_background_mode": "true",
				"rule_name":              "test-rule",
				"rule_type":              "validate",
			},
		},
		{
			name: "namespaced policy",
			policy: &kyvernov1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
				Spec: kyvernov1.Spec{
					Rules: []kyvernov1.Rule{{
						Name:           "mutate-rule",
						MatchResources: matchConfigMaps,
						Mutation: &kyvernov1.Mutation{
							PatchesJSON6902: "[]",
						},
					}},
				},
			},
			want: map[string]any{
				"policy_namespace":       "default",
				"policy_name":            "test-policy",
				"policy_type":            "namespaced",
				"status_ready":           false,
				"policy_validation_mode": "audit",
				"policy_background_mode": "true",
				"rule_name":              "mutate-rule",
				"rule_type":              "mutate",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs, err := metrics.GetPolicyInfoMetrics().BuildPolicyRuleObservations(tt.policy)
			assert.NoError(t, err)

			samples := collectPolicyRuleObservations(t, obs)
			assert.Len(t, samples, 1)
			assert.Equal(t, float64(1), samples[0].value)
			assert.Equal(t, tt.want, samples[0].attributes)
		})
	}
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
	entries := snapshotEntries(c.snapshot)
	assert.Len(t, entries, 1)
	assert.Empty(t, entries[0].namespace)
	assert.Len(t, entries[0].series, 2, "expected the update to replace the stored observation")

	// delete drops the entry, tombstone or not
	c.deletePolicy(p2)
	wg.Wait()
	assert.Empty(t, snapshotEntries(c.snapshot))

	// a tombstone without the object still identifies the policy by its key
	c.addPolicy(p1)
	c.deletePolicy(cache.DeletedFinalStateUnknown{Key: "test-policy"})
	wg.Wait()
	assert.Empty(t, snapshotEntries(c.snapshot))
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
	entries := snapshotEntries(c.snapshot)
	assert.Len(t, entries, 1)
	assert.Equal(t, "default", entries[0].namespace)
	assert.Len(t, entries[0].series, 2, "expected the update to replace the stored observation")

	// delete drops the entry, tombstone or not
	c.deleteNsPolicy(p2)
	wg.Wait()
	assert.Empty(t, snapshotEntries(c.snapshot))

	// a tombstone without the object still identifies the policy by its key
	c.addNsPolicy(p1)
	c.deleteNsPolicy(cache.DeletedFinalStateUnknown{Key: "default/test-policy"})
	wg.Wait()
	assert.Empty(t, snapshotEntries(c.snapshot))
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
