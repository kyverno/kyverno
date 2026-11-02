package globalcontext

import (
	"context"
	"sync"
	"testing"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	kyvernov2beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2beta1"
	kconfig "github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// observedCall captures one ObserveProjectionLanguage/ObserveJMESPathFunctionUsage invocation.
type observedCall struct {
	kind  string // "language" or "function"
	value string
	count int
}

// fakeGlobalContextMetrics is a test double for metrics.GlobalContextMetrics.
type fakeGlobalContextMetrics struct {
	mu    sync.Mutex
	calls []observedCall
}

func (f *fakeGlobalContextMetrics) ObserveProjectionLanguage(_ context.Context, _ metric.Observer, language string, count int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, observedCall{kind: "language", value: language, count: count})
}

func (f *fakeGlobalContextMetrics) ObserveJMESPathFunctionUsage(_ context.Context, _ metric.Observer, function string, count int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, observedCall{kind: "function", value: function, count: count})
}

func (f *fakeGlobalContextMetrics) RegisterCallback(metric.Callback) (metric.Registration, error) {
	return nil, nil
}

// fakeMetricsManager implements metrics.MetricsConfigManager, exposing only GlobalContextMetrics;
// reportProjectionMetrics never touches the rest of the interface.
type fakeMetricsManager struct {
	gctx *fakeGlobalContextMetrics
}

func (f *fakeMetricsManager) Config() kconfig.MetricsConfiguration { return nil }
func (f *fakeMetricsManager) RecordClientQueries(context.Context, metrics.ClientQueryOperation, metrics.ClientType, string, string) {
}
func (f *fakeMetricsManager) BreakerMetrics() metrics.BreakerMetrics             { return nil }
func (f *fakeMetricsManager) ControllerMetrics() metrics.ControllerMetrics       { return nil }
func (f *fakeMetricsManager) DeletingMetrics() metrics.DeletingMetrics           { return nil }
func (f *fakeMetricsManager) UpdateRequestMetrics() metrics.UpdateRequestMetrics { return nil }
func (f *fakeMetricsManager) TTLInfoMetrics() metrics.TTLInfoMetrics             { return nil }
func (f *fakeMetricsManager) EventMetrics() metrics.EventMetrics                 { return nil }
func (f *fakeMetricsManager) AdmissionMetrics() metrics.AdmissionMetrics         { return nil }
func (f *fakeMetricsManager) DeprecatedAPIRequestMetrics() metrics.DeprecatedAPIRequestMetrics {
	return nil
}
func (f *fakeMetricsManager) LegacyPolicyMetrics() metrics.LegacyPolicyMetrics   { return nil }
func (f *fakeMetricsManager) HTTPMetrics() metrics.HTTPMetrics                   { return nil }
func (f *fakeMetricsManager) VPOLMetrics() metrics.ValidatingMetrics             { return nil }
func (f *fakeMetricsManager) IVPOLMetrics() metrics.ImageValidatingMetrics       { return nil }
func (f *fakeMetricsManager) MPOLMetrics() metrics.MutatingMetrics               { return nil }
func (f *fakeMetricsManager) GPOLMetrics() metrics.GeneratingMetrics             { return nil }
func (f *fakeMetricsManager) GlobalContextMetrics() metrics.GlobalContextMetrics { return f.gctx }

// fakeGCELister is a minimal GlobalContextEntryLister test double backed by a fixed slice,
// so reportProjectionMetrics can be exercised without an informer.
type fakeGCELister struct {
	entries []*kyvernov2beta1.GlobalContextEntry
}

func (f *fakeGCELister) List(selector labels.Selector) ([]*kyvernov2beta1.GlobalContextEntry, error) {
	return f.entries, nil
}

func (f *fakeGCELister) Get(name string) (*kyvernov2beta1.GlobalContextEntry, error) {
	for _, gce := range f.entries {
		if gce.Name == name {
			return gce, nil
		}
	}
	return nil, nil
}

var _ kyvernov2beta1listers.GlobalContextEntryLister = &fakeGCELister{}

func callsOfKind(calls []observedCall, kind string) map[string]int {
	out := map[string]int{}
	for _, c := range calls {
		if c.kind == kind {
			out[c.value] = c.count
		}
	}
	return out
}

func TestReportProjectionMetrics(t *testing.T) {
	fakeGctx := &fakeGlobalContextMetrics{}
	metrics.SetManager(&fakeMetricsManager{gctx: fakeGctx})
	t.Cleanup(func() { metrics.SetManager(nil) })

	gceA := &kyvernov2beta1.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "a"},
		Spec: kyvernov2beta1.GlobalContextEntrySpec{
			Projections: []kyvernov2beta1.GlobalContextEntryProjection{
				{Name: "a", JMESPath: "add(replicas, 1)"},
				{Name: "b", Expression: "object.replicas + 1"},
				{Name: "c", JMESPath: "deployments[*].name"},
			},
		},
	}
	gceB := &kyvernov2beta1.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "b"},
		Spec: kyvernov2beta1.GlobalContextEntrySpec{
			Projections: []kyvernov2beta1.GlobalContextEntryProjection{
				{Name: "d", JMESPath: "add(replicas, 2)"},
			},
		},
	}

	c := &controller{gceLister: &fakeGCELister{entries: []*kyvernov2beta1.GlobalContextEntry{gceA, gceB}}}
	err := c.reportProjectionMetrics(context.Background(), nil)
	assert.NoError(t, err)

	fakeGctx.mu.Lock()
	defer fakeGctx.mu.Unlock()

	languages := callsOfKind(fakeGctx.calls, "language")
	assert.Equal(t, 3, languages["jmespath"]) // a, c, d
	assert.Equal(t, 1, languages["cel"])      // b

	functions := callsOfKind(fakeGctx.calls, "function")
	assert.Equal(t, 2, functions["add"]) // a and d call add(); c and b never do
	assert.Equal(t, 1, len(functions))
}

func TestReportProjectionMetrics_NoEntries(t *testing.T) {
	fakeGctx := &fakeGlobalContextMetrics{}
	metrics.SetManager(&fakeMetricsManager{gctx: fakeGctx})
	t.Cleanup(func() { metrics.SetManager(nil) })

	c := &controller{gceLister: &fakeGCELister{}}
	err := c.reportProjectionMetrics(context.Background(), nil)
	assert.NoError(t, err)

	fakeGctx.mu.Lock()
	defer fakeGctx.mu.Unlock()

	languages := callsOfKind(fakeGctx.calls, "language")
	assert.Equal(t, 0, languages["jmespath"])
	assert.Equal(t, 0, languages["cel"])
	assert.Equal(t, 0, len(callsOfKind(fakeGctx.calls, "function")))
}

func TestReportProjectionMetrics_NilManager(t *testing.T) {
	metrics.SetManager(nil)

	c := &controller{gceLister: &fakeGCELister{entries: []*kyvernov2beta1.GlobalContextEntry{
		{Spec: kyvernov2beta1.GlobalContextEntrySpec{
			Projections: []kyvernov2beta1.GlobalContextEntryProjection{{Name: "a", JMESPath: "deployments"}},
		}},
	}}}
	assert.NotPanics(t, func() {
		err := c.reportProjectionMetrics(context.Background(), nil)
		assert.NoError(t, err)
	})
}
