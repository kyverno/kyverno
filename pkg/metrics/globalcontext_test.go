package metrics

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// collectGlobalContextMetrics builds a real (manual-reader-backed) meter, wires m into it the
// same way MetricsConfig.init does, registers report as the gauge callback, collects once, and
// returns every int64 gauge data point recorded under instrumentName.
func collectGlobalContextMetrics(t *testing.T, m *globalContextMetrics, report metric.Callback, instrumentName string) []metricdata.DataPoint[int64] {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	m.callback = report
	m.init(meter)

	var rm metricdata.ResourceMetrics
	assert.NoError(t, reader.Collect(context.Background(), &rm))

	for _, sm := range rm.ScopeMetrics {
		for _, inst := range sm.Metrics {
			if inst.Name != instrumentName {
				continue
			}
			gauge, ok := inst.Data.(metricdata.Gauge[int64])
			if !ok {
				t.Fatalf("instrument %q is not an int64 gauge: %T", instrumentName, inst.Data)
			}
			return gauge.DataPoints
		}
	}
	return nil
}

func attrValue(dp metricdata.DataPoint[int64], key string) (string, bool) {
	v, ok := dp.Attributes.Value(attribute.Key(key))
	if !ok {
		return "", false
	}
	return v.AsString(), true
}

func TestGlobalContextMetrics_ObserveProjectionLanguage(t *testing.T) {
	m := &globalContextMetrics{logger: logr.Discard()}
	report := func(_ context.Context, observer metric.Observer) error {
		m.ObserveProjectionLanguage(context.Background(), observer, "jmespath", 3)
		m.ObserveProjectionLanguage(context.Background(), observer, "cel", 1)
		return nil
	}

	points := collectGlobalContextMetrics(t, m, report, "kyverno_globalcontextentry_projection_language")
	assert.Len(t, points, 2)

	byLanguage := map[string]int64{}
	for _, dp := range points {
		lang, ok := attrValue(dp, "language")
		assert.True(t, ok)
		byLanguage[lang] = dp.Value
	}
	assert.Equal(t, int64(3), byLanguage["jmespath"])
	assert.Equal(t, int64(1), byLanguage["cel"])
}

func TestGlobalContextMetrics_ObserveJMESPathFunctionUsage(t *testing.T) {
	m := &globalContextMetrics{logger: logr.Discard()}
	report := func(_ context.Context, observer metric.Observer) error {
		m.ObserveJMESPathFunctionUsage(context.Background(), observer, "add", 2)
		return nil
	}

	points := collectGlobalContextMetrics(t, m, report, "kyverno_globalcontextentry_jmespath_function_usage")
	assert.Len(t, points, 1)
	fn, ok := attrValue(points[0], "function")
	assert.True(t, ok)
	assert.Equal(t, "add", fn)
	assert.Equal(t, int64(2), points[0].Value)
}

func TestGlobalContextMetrics_NilInstrumentGuard(t *testing.T) {
	m := &globalContextMetrics{logger: logr.Discard()}

	// Neither instrument was created (e.g. meter.Int64ObservableGauge failed); observing
	// must not panic even with a nil observer.
	assert.NotPanics(t, func() {
		m.ObserveProjectionLanguage(context.Background(), nil, "jmespath", 1)
		m.ObserveJMESPathFunctionUsage(context.Background(), nil, "add", 1)
	})
}
