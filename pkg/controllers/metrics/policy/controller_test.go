package policy

import (
	"context"
	"sync"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	policyChangesMetric "github.com/kyverno/kyverno/pkg/metrics/policychanges"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

// Fake PolicyRuleMetrics

type fakePolicyRuleMetrics struct {
	mu     sync.Mutex
	calls  []string // policy names passed to RecordPolicyRuleInfo
	recErr error    // error returned by RecordPolicyRuleInfo
	regErr error    // error returned by RegisterCallback
}

func (f *fakePolicyRuleMetrics) RecordPolicyRuleInfo(_ context.Context, policy kyvernov1.PolicyInterface, _ metric.Observer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, policy.GetName())
	return f.recErr
}

func (f *fakePolicyRuleMetrics) RegisterCallback(_ metric.Callback) (metric.Registration, error) {
	return nil, f.regErr
}

func (f *fakePolicyRuleMetrics) recordedCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

// Fake MetricsConfigManager for observing policy change metrics

type recordedPolicyChange struct {
	ValidationMode   metrics.PolicyValidationMode
	PolicyType       metrics.PolicyType
	BackgroundMode   metrics.PolicyBackgroundMode
	PolicyNamespace  string
	PolicyName       string
	PolicyChangeType string
}

type fakeMetricsManager struct {
	metrics.MetricsConfigManager
	mu            sync.Mutex
	policyChanges []recordedPolicyChange
}

func newFakeMetricsManager() *fakeMetricsManager {
	return &fakeMetricsManager{
		MetricsConfigManager: metrics.NewFakeMetricsConfig(),
	}
}

func (f *fakeMetricsManager) RecordPolicyChanges(
	ctx context.Context,
	policyValidationMode metrics.PolicyValidationMode,
	policyType metrics.PolicyType,
	policyBackgroundMode metrics.PolicyBackgroundMode,
	policyNamespace string,
	policyName string,
	policyChangeType string,
) {
	f.mu.Lock()
	f.policyChanges = append(f.policyChanges, recordedPolicyChange{
		ValidationMode:   policyValidationMode,
		PolicyType:       policyType,
		BackgroundMode:   policyBackgroundMode,
		PolicyNamespace:  policyNamespace,
		PolicyName:       policyName,
		PolicyChangeType: policyChangeType,
	})
	f.mu.Unlock()
	f.MetricsConfigManager.RecordPolicyChanges(ctx, policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, policyChangeType)
}

func (f *fakeMetricsManager) recordedChanges() []recordedPolicyChange {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]recordedPolicyChange, len(f.policyChanges))
	copy(out, f.policyChanges)
	return out
}

// Global test mutex protecting the package-level metrics manager.
var testMetricsMu sync.Mutex

func setupMetricsTest(t *testing.T) *fakeMetricsManager {
	t.Helper()
	testMetricsMu.Lock()
	orig := metrics.GetManager()
	fake := newFakeMetricsManager()
	metrics.SetManager(fake)
	t.Cleanup(func() {
		metrics.SetManager(orig)
		testMetricsMu.Unlock()
	})
	return fake
}

// Helpers

func newNamespaceIndexer() cache.Indexer {
	return cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
}

// newTestController constructs a controller with real lister-backed indexers and
// an optional fake PolicyRuleMetrics. Use nil for rm to test the nil-ruleInfo path.
func newTestController(cpolIndexer, polIndexer cache.Indexer, rm metrics.PolicyRuleMetrics) *controller {
	wg := &wait.Group{}
	return &controller{
		ruleInfo:   rm,
		cpolLister: kyvernov1listers.NewClusterPolicyLister(cpolIndexer),
		polLister:  kyvernov1listers.NewPolicyLister(polIndexer),
		waitGroup:  wg,
	}
}

func makeClusterPolicy(name string) *kyvernov1.ClusterPolicy {
	return &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

func makePolicy(namespace, name string) *kyvernov1.Policy {
	return &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
	}
}

// drainWaitGroup blocks until all goroutines launched via waitGroup.Start finish.
func drainWaitGroup(c *controller) {
	c.waitGroup.Wait()
}

// report() tests

func TestReport_AggregatesClusterPolicies(t *testing.T) {
	t.Parallel()

	cpolIndexer := newNamespaceIndexer()
	polIndexer := newNamespaceIndexer()

	cpol1 := makeClusterPolicy("cpol-1")
	cpol2 := makeClusterPolicy("cpol-2")
	require.NoError(t, cpolIndexer.Add(cpol1))
	require.NoError(t, cpolIndexer.Add(cpol2))

	rm := &fakePolicyRuleMetrics{}
	c := newTestController(cpolIndexer, polIndexer, rm)

	err := c.report(context.Background(), nil)
	assert.NoError(t, err)

	calls := rm.recordedCalls()
	assert.ElementsMatch(t, []string{"cpol-1", "cpol-2"}, calls)
}

func TestReport_AggregatesNamespacedPolicies(t *testing.T) {
	t.Parallel()

	cpolIndexer := newNamespaceIndexer()
	polIndexer := newNamespaceIndexer()

	pol1 := makePolicy("ns1", "pol-1")
	pol2 := makePolicy("ns2", "pol-2")
	require.NoError(t, polIndexer.Add(pol1))
	require.NoError(t, polIndexer.Add(pol2))

	rm := &fakePolicyRuleMetrics{}
	c := newTestController(cpolIndexer, polIndexer, rm)

	err := c.report(context.Background(), nil)
	assert.NoError(t, err)

	calls := rm.recordedCalls()
	assert.ElementsMatch(t, []string{"pol-1", "pol-2"}, calls)
}

func TestReport_AggregatesMixedPolicies(t *testing.T) {
	t.Parallel()

	cpolIndexer := newNamespaceIndexer()
	polIndexer := newNamespaceIndexer()

	require.NoError(t, cpolIndexer.Add(makeClusterPolicy("cpol-a")))
	require.NoError(t, polIndexer.Add(makePolicy("ns1", "pol-b")))

	rm := &fakePolicyRuleMetrics{}
	c := newTestController(cpolIndexer, polIndexer, rm)

	err := c.report(context.Background(), nil)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"pol-b", "cpol-a"}, rm.recordedCalls())
}

func TestReport_EmptyListers_NoError(t *testing.T) {
	t.Parallel()

	rm := &fakePolicyRuleMetrics{}
	c := newTestController(newNamespaceIndexer(), newNamespaceIndexer(), rm)

	err := c.report(context.Background(), nil)
	assert.NoError(t, err)
	assert.Empty(t, rm.recordedCalls())
}

func TestReport_RecordPolicyRuleInfo_ErrorPropagates(t *testing.T) {
	t.Parallel()

	cpolIndexer := newNamespaceIndexer()
	polIndexer := newNamespaceIndexer()
	require.NoError(t, polIndexer.Add(makePolicy("ns1", "pol-err")))

	rm := &fakePolicyRuleMetrics{recErr: assert.AnError}
	c := newTestController(cpolIndexer, polIndexer, rm)

	err := c.report(context.Background(), nil)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestReport_NilRuleInfo_DoesNotPanic(t *testing.T) {
	t.Parallel()

	cpolIndexer := newNamespaceIndexer()
	polIndexer := newNamespaceIndexer()
	require.NoError(t, cpolIndexer.Add(makeClusterPolicy("cpol-1")))
	require.NoError(t, polIndexer.Add(makePolicy("ns1", "pol-1")))

	// nil ruleInfo — report() is guarded against nil ruleInfo and should return nil without panicking.
	c := newTestController(cpolIndexer, polIndexer, nil)

	err := c.report(context.Background(), nil)
	assert.NoError(t, err)
}

// addPolicy / deletePolicy (ClusterPolicy event handlers)
// Note: Event handler tests mutate the global metrics manager and are serialized via setupMetricsTest.

func TestAddPolicy_SchedulesRoutine(t *testing.T) {
	fakeManager := setupMetricsTest(t)

	c := newTestController(newNamespaceIndexer(), newNamespaceIndexer(), nil)

	cpol := makeClusterPolicy("cpol-add")
	c.addPolicy(cpol)

	drainWaitGroup(c)

	records := fakeManager.recordedChanges()
	require.Len(t, records, 1)
	assert.Equal(t, "cpol-add", records[0].PolicyName)
	assert.Equal(t, "-", records[0].PolicyNamespace)
	assert.Equal(t, metrics.Cluster, records[0].PolicyType)
	assert.Equal(t, string(policyChangesMetric.PolicyCreated), records[0].PolicyChangeType)
}

func TestDeletePolicy_DirectObject_SchedulesRoutine(t *testing.T) {
	fakeManager := setupMetricsTest(t)

	c := newTestController(newNamespaceIndexer(), newNamespaceIndexer(), nil)

	cpol := makeClusterPolicy("cpol-del")
	c.deletePolicy(cpol) // raw *ClusterPolicy, no tombstone

	drainWaitGroup(c)

	records := fakeManager.recordedChanges()
	require.Len(t, records, 1)
	assert.Equal(t, "cpol-del", records[0].PolicyName)
	assert.Equal(t, "-", records[0].PolicyNamespace)
	assert.Equal(t, metrics.Cluster, records[0].PolicyType)
	assert.Equal(t, string(policyChangesMetric.PolicyDeleted), records[0].PolicyChangeType)
}

func TestDeletePolicy_Tombstone_SchedulesRoutine(t *testing.T) {
	fakeManager := setupMetricsTest(t)

	c := newTestController(newNamespaceIndexer(), newNamespaceIndexer(), nil)

	cpol := makeClusterPolicy("cpol-del-tombstone")
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "cpol-del-tombstone",
		Obj: cpol,
	}
	c.deletePolicy(tombstone)

	drainWaitGroup(c)

	records := fakeManager.recordedChanges()
	require.Len(t, records, 1)
	assert.Equal(t, "cpol-del-tombstone", records[0].PolicyName)
	assert.Equal(t, "-", records[0].PolicyNamespace)
	assert.Equal(t, metrics.Cluster, records[0].PolicyType)
	assert.Equal(t, string(policyChangesMetric.PolicyDeleted), records[0].PolicyChangeType)
}

func TestDeletePolicy_InvalidObject_NoRoutine(t *testing.T) {
	fakeManager := setupMetricsTest(t)

	c := newTestController(newNamespaceIndexer(), newNamespaceIndexer(), nil)

	// Pass an object that cannot be type-asserted to *ClusterPolicy.
	// deletePolicy logs a warning and returns — no goroutine is started and no metric is recorded.
	c.deletePolicy("not-a-policy")

	drainWaitGroup(c)

	assert.Empty(t, fakeManager.recordedChanges())
}

func TestUpdatePolicy_SameSpec_NoRoutineStarted(t *testing.T) {
	fakeManager := setupMetricsTest(t)

	c := newTestController(newNamespaceIndexer(), newNamespaceIndexer(), nil)

	cpol := makeClusterPolicy("cpol-upd-same")
	// old and new are identical → registerPolicyChangesMetricUpdatePolicy returns early without recording metrics.
	c.updatePolicy(cpol, cpol)

	drainWaitGroup(c)

	assert.Empty(t, fakeManager.recordedChanges())
}

func TestUpdatePolicy_DifferentSpec_SchedulesRoutine(t *testing.T) {
	fakeManager := setupMetricsTest(t)

	c := newTestController(newNamespaceIndexer(), newNamespaceIndexer(), nil)

	oldCpol := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "cpol-upd"},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: kyvernov1.Audit,
		},
	}
	newCpol := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "cpol-upd"},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: kyvernov1.Enforce,
		},
	}

	c.updatePolicy(oldCpol, newCpol)

	drainWaitGroup(c)

	records := fakeManager.recordedChanges()
	require.Len(t, records, 2)
	assert.Equal(t, "cpol-upd", records[0].PolicyName)
	assert.Equal(t, "-", records[0].PolicyNamespace)
	assert.Equal(t, metrics.Cluster, records[0].PolicyType)
	assert.Equal(t, string(policyChangesMetric.PolicyUpdated), records[0].PolicyChangeType)
	assert.Equal(t, metrics.Audit, records[0].ValidationMode)

	assert.Equal(t, "cpol-upd", records[1].PolicyName)
	assert.Equal(t, "-", records[1].PolicyNamespace)
	assert.Equal(t, metrics.Cluster, records[1].PolicyType)
	assert.Equal(t, string(policyChangesMetric.PolicyUpdated), records[1].PolicyChangeType)
	assert.Equal(t, metrics.Enforce, records[1].ValidationMode)
}

func TestUpdatePolicy_DifferentSpec_NoModeChange_SingleRecord(t *testing.T) {
	fakeManager := setupMetricsTest(t)

	c := newTestController(newNamespaceIndexer(), newNamespaceIndexer(), nil)

	oldCpol := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "cpol-upd-single"},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: kyvernov1.Audit,
			ApplyRules:              ptr.To(kyvernov1.ApplyOne),
		},
	}
	newCpol := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "cpol-upd-single"},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: kyvernov1.Audit,
			ApplyRules:              ptr.To(kyvernov1.ApplyAll),
		},
	}

	c.updatePolicy(oldCpol, newCpol)

	drainWaitGroup(c)

	records := fakeManager.recordedChanges()
	require.Len(t, records, 1)
	assert.Equal(t, "cpol-upd-single", records[0].PolicyName)
	assert.Equal(t, "-", records[0].PolicyNamespace)
	assert.Equal(t, metrics.Cluster, records[0].PolicyType)
	assert.Equal(t, string(policyChangesMetric.PolicyUpdated), records[0].PolicyChangeType)
	assert.Equal(t, metrics.Audit, records[0].ValidationMode)
}

// addNsPolicy / deleteNsPolicy (namespaced Policy event handlers)

func TestAddNsPolicy_SchedulesRoutine(t *testing.T) {
	fakeManager := setupMetricsTest(t)

	c := newTestController(newNamespaceIndexer(), newNamespaceIndexer(), nil)

	pol := makePolicy("ns1", "pol-add")
	c.addNsPolicy(pol)

	drainWaitGroup(c)

	records := fakeManager.recordedChanges()
	require.Len(t, records, 1)
	assert.Equal(t, "pol-add", records[0].PolicyName)
	assert.Equal(t, "ns1", records[0].PolicyNamespace)
	assert.Equal(t, metrics.Namespaced, records[0].PolicyType)
	assert.Equal(t, string(policyChangesMetric.PolicyCreated), records[0].PolicyChangeType)
}

func TestDeleteNsPolicy_DirectObject_SchedulesRoutine(t *testing.T) {
	fakeManager := setupMetricsTest(t)

	c := newTestController(newNamespaceIndexer(), newNamespaceIndexer(), nil)

	pol := makePolicy("ns1", "pol-del")
	c.deleteNsPolicy(pol)

	drainWaitGroup(c)

	records := fakeManager.recordedChanges()
	require.Len(t, records, 1)
	assert.Equal(t, "pol-del", records[0].PolicyName)
	assert.Equal(t, "ns1", records[0].PolicyNamespace)
	assert.Equal(t, metrics.Namespaced, records[0].PolicyType)
	assert.Equal(t, string(policyChangesMetric.PolicyDeleted), records[0].PolicyChangeType)
}

func TestDeleteNsPolicy_Tombstone_SchedulesRoutine(t *testing.T) {
	fakeManager := setupMetricsTest(t)

	c := newTestController(newNamespaceIndexer(), newNamespaceIndexer(), nil)

	pol := makePolicy("ns1", "pol-del-tombstone")
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "ns1/pol-del-tombstone",
		Obj: pol,
	}
	c.deleteNsPolicy(tombstone)

	drainWaitGroup(c)

	records := fakeManager.recordedChanges()
	require.Len(t, records, 1)
	assert.Equal(t, "pol-del-tombstone", records[0].PolicyName)
	assert.Equal(t, "ns1", records[0].PolicyNamespace)
	assert.Equal(t, metrics.Namespaced, records[0].PolicyType)
	assert.Equal(t, string(policyChangesMetric.PolicyDeleted), records[0].PolicyChangeType)
}

func TestDeleteNsPolicy_InvalidObject_NoRoutine(t *testing.T) {
	fakeManager := setupMetricsTest(t)

	c := newTestController(newNamespaceIndexer(), newNamespaceIndexer(), nil)

	c.deleteNsPolicy("not-a-policy")

	drainWaitGroup(c)

	assert.Empty(t, fakeManager.recordedChanges())
}

func TestUpdateNsPolicy_SameSpec_NoRoutineStarted(t *testing.T) {
	fakeManager := setupMetricsTest(t)

	c := newTestController(newNamespaceIndexer(), newNamespaceIndexer(), nil)

	pol := makePolicy("ns1", "pol-upd-same")
	c.updateNsPolicy(pol, pol)

	drainWaitGroup(c)

	assert.Empty(t, fakeManager.recordedChanges())
}

func TestUpdateNsPolicy_DifferentSpec_SchedulesRoutine(t *testing.T) {
	fakeManager := setupMetricsTest(t)

	c := newTestController(newNamespaceIndexer(), newNamespaceIndexer(), nil)

	oldPol := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "pol-upd"},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: kyvernov1.Audit,
		},
	}
	newPol := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "pol-upd"},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: kyvernov1.Enforce,
		},
	}

	c.updateNsPolicy(oldPol, newPol)

	drainWaitGroup(c)

	records := fakeManager.recordedChanges()
	require.Len(t, records, 2)
	assert.Equal(t, "pol-upd", records[0].PolicyName)
	assert.Equal(t, "ns1", records[0].PolicyNamespace)
	assert.Equal(t, metrics.Namespaced, records[0].PolicyType)
	assert.Equal(t, string(policyChangesMetric.PolicyUpdated), records[0].PolicyChangeType)
	assert.Equal(t, metrics.Audit, records[0].ValidationMode)

	assert.Equal(t, "pol-upd", records[1].PolicyName)
	assert.Equal(t, "ns1", records[1].PolicyNamespace)
	assert.Equal(t, metrics.Namespaced, records[1].PolicyType)
	assert.Equal(t, string(policyChangesMetric.PolicyUpdated), records[1].PolicyChangeType)
	assert.Equal(t, metrics.Enforce, records[1].ValidationMode)
}

func TestUpdateNsPolicy_DifferentSpec_NoModeChange_SingleRecord(t *testing.T) {
	fakeManager := setupMetricsTest(t)

	c := newTestController(newNamespaceIndexer(), newNamespaceIndexer(), nil)

	oldPol := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "pol-upd-single"},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: kyvernov1.Audit,
			ApplyRules:              ptr.To(kyvernov1.ApplyOne),
		},
	}
	newPol := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "pol-upd-single"},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: kyvernov1.Audit,
			ApplyRules:              ptr.To(kyvernov1.ApplyAll),
		},
	}

	c.updateNsPolicy(oldPol, newPol)

	drainWaitGroup(c)

	records := fakeManager.recordedChanges()
	require.Len(t, records, 1)
	assert.Equal(t, "pol-upd-single", records[0].PolicyName)
	assert.Equal(t, "ns1", records[0].PolicyNamespace)
	assert.Equal(t, metrics.Namespaced, records[0].PolicyType)
	assert.Equal(t, string(policyChangesMetric.PolicyUpdated), records[0].PolicyChangeType)
	assert.Equal(t, metrics.Audit, records[0].ValidationMode)
}

// report() — multiple policies, concurrency

func TestReport_MultipleClusterPolicies_AllReported(t *testing.T) {
	t.Parallel()

	cpolIndexer := newNamespaceIndexer()
	polIndexer := newNamespaceIndexer()

	names := []string{"alpha", "beta", "gamma"}
	for _, n := range names {
		require.NoError(t, cpolIndexer.Add(makeClusterPolicy(n)))
	}

	rm := &fakePolicyRuleMetrics{}
	c := newTestController(cpolIndexer, polIndexer, rm)

	err := c.report(context.Background(), nil)
	assert.NoError(t, err)
	assert.ElementsMatch(t, names, rm.recordedCalls())
}

func TestReport_MultipleNamespacedPolicies_AllReported(t *testing.T) {
	t.Parallel()

	cpolIndexer := newNamespaceIndexer()
	polIndexer := newNamespaceIndexer()

	require.NoError(t, polIndexer.Add(makePolicy("ns1", "pol-x")))
	require.NoError(t, polIndexer.Add(makePolicy("ns2", "pol-y")))
	require.NoError(t, polIndexer.Add(makePolicy("ns3", "pol-z")))

	rm := &fakePolicyRuleMetrics{}
	c := newTestController(cpolIndexer, polIndexer, rm)

	err := c.report(context.Background(), nil)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"pol-x", "pol-y", "pol-z"}, rm.recordedCalls())
}
