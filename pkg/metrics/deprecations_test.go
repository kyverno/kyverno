package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestDeprecationMetrics_Record(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	m := &deprecationsMetrics{}
	m.init(provider.Meter("test"))

	ctx := context.Background()
	m.RecordDeprecatedAPIRequest(ctx, "kyverno.io", "v2beta1", "ClusterPolicy", "")
	m.RecordDeprecatedAPIRequest(ctx, "kyverno.io", "v2beta1", "ClusterPolicy", "spec.validationFailureAction")

	var rm metricdata.ResourceMetrics
	assert.NoError(t, reader.Collect(ctx, &rm))

	var sum *metricdata.Sum[int64]
	for _, sm := range rm.ScopeMetrics {
		for i := range sm.Metrics {
			if sm.Metrics[i].Name == "kyverno_deprecated_api_requests_total" {
				s := sm.Metrics[i].Data.(metricdata.Sum[int64])
				sum = &s
			}
		}
	}
	assert.NotNil(t, sum, "metric kyverno_deprecated_api_requests_total not found")
	assert.Len(t, sum.DataPoints, 2)

	var total int64
	for _, dp := range sum.DataPoints {
		total += dp.Value
		attrs := make(map[string]string)
		for _, kv := range dp.Attributes.ToSlice() {
			attrs[string(kv.Key)] = kv.Value.AsString()
		}
		assert.Equal(t, "kyverno.io", attrs["group"])
		assert.Equal(t, "v2beta1", attrs["version"])
		assert.Equal(t, "ClusterPolicy", attrs["kind"])
	}
	assert.Equal(t, int64(2), total)
}

func TestDeprecationMetrics_RecordNilInstrument(t *testing.T) {
	m := &deprecationsMetrics{} // requestsMetric is nil
	assert.NotPanics(t, func() {
		m.RecordDeprecatedAPIRequest(context.Background(), "g", "v", "k", "f")
	})
}

func TestGetDeprecationMetrics_NilManager(t *testing.T) {
	old := GetManager()
	t.Cleanup(func() { SetManager(old) })
	SetManager(nil)
	assert.Nil(t, GetDeprecationMetrics())
}
