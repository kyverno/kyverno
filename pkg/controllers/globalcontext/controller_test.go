package globalcontext

import (
	"context"
	"sync"
	"testing"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	kconfig "github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/stretchr/testify/assert"
)

// recordedCall captures one RecordProjectionLanguage/RecordJMESPathFunctionUsage invocation.
type recordedCall struct {
	kind  string // "language" or "function"
	value string
}

// fakeGlobalContextMetrics is a test double for metrics.GlobalContextMetrics.
type fakeGlobalContextMetrics struct {
	mu    sync.Mutex
	calls []recordedCall
}

func (f *fakeGlobalContextMetrics) RecordProjectionLanguage(_ context.Context, language string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, recordedCall{kind: "language", value: language})
}

func (f *fakeGlobalContextMetrics) RecordJMESPathFunctionUsage(_ context.Context, function string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, recordedCall{kind: "function", value: function})
}

// fakeMetricsManager implements metrics.MetricsConfigManager, exposing only GlobalContextMetrics;
// recordProjectionMetrics never touches the rest of the interface.
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

func TestRecordProjectionMetrics(t *testing.T) {
	fakeGctx := &fakeGlobalContextMetrics{}
	metrics.SetManager(&fakeMetricsManager{gctx: fakeGctx})
	t.Cleanup(func() { metrics.SetManager(nil) })

	gce := &kyvernov2beta1.GlobalContextEntry{
		Spec: kyvernov2beta1.GlobalContextEntrySpec{
			Projections: []kyvernov2beta1.GlobalContextEntryProjection{
				{Name: "a", JMESPath: "add(replicas, 1)"},
				{Name: "b", Expression: "object.replicas + 1"},
				{Name: "c", JMESPath: "deployments[*].name"},
			},
		},
	}

	c := &controller{}
	c.recordProjectionMetrics(context.Background(), gce)

	fakeGctx.mu.Lock()
	defer fakeGctx.mu.Unlock()
	assert.Contains(t, fakeGctx.calls, recordedCall{kind: "language", value: "jmespath"})
	assert.Contains(t, fakeGctx.calls, recordedCall{kind: "language", value: "cel"})
	assert.Contains(t, fakeGctx.calls, recordedCall{kind: "function", value: "add"})

	// Projection "c" has no known function calls, and the CEL projection "b" must never produce
	// a function-usage call at all.
	functionCalls := 0
	for _, call := range fakeGctx.calls {
		if call.kind == "function" {
			functionCalls++
		}
	}
	assert.Equal(t, 1, functionCalls)
}

func TestRecordProjectionMetrics_NilManager(t *testing.T) {
	metrics.SetManager(nil)

	gce := &kyvernov2beta1.GlobalContextEntry{
		Spec: kyvernov2beta1.GlobalContextEntrySpec{
			Projections: []kyvernov2beta1.GlobalContextEntryProjection{
				{Name: "a", JMESPath: "deployments"},
			},
		},
	}

	c := &controller{}
	assert.NotPanics(t, func() {
		c.recordProjectionMetrics(context.Background(), gce)
	})
}
