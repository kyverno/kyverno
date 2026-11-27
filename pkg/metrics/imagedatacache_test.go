package metrics

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kconfig "github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// collectImageDataCacheRequests wires a real MetricsConfig to a manual reader, runs record, and returns the
// request counter by result label.
func collectImageDataCacheRequests(t *testing.T, record func(ImageDataCacheMetrics)) map[string]int64 {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	config := NewMetricsConfigManager(logr.Discard(), kconfig.NewDefaultMetricsConfiguration())
	require.NoError(t, config.initializeMetrics(provider))

	record(config.ImageDataCacheMetrics())

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	got := map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		for _, inst := range sm.Metrics {
			if inst.Name != "kyverno_image_data_cache_requests" {
				continue
			}
			sum, ok := inst.Data.(metricdata.Sum[int64])
			require.True(t, ok, "unexpected data type %T", inst.Data)
			for _, dp := range sum.DataPoints {
				result, ok := dp.Attributes.Value("result")
				require.True(t, ok, "data point has no result label")
				got[result.AsString()] = dp.Value
			}
		}
	}
	return got
}

func TestImageDataCacheMetrics_RecordRequest(t *testing.T) {
	got := collectImageDataCacheRequests(t, func(m ImageDataCacheMetrics) {
		ctx := context.Background()
		m.RecordRequest(ctx, ImageDataCacheHit)
		m.RecordRequest(ctx, ImageDataCacheHit)
		m.RecordRequest(ctx, ImageDataCacheMiss)
		m.RecordRequest(ctx, ImageDataCacheError)
	})
	assert.Equal(t, map[string]int64{"hit": 2, "miss": 1, "error": 1}, got)
}

func TestImageDataCacheMetrics_RecordRequestWithoutCounterIsANoop(t *testing.T) {
	// the counter stays nil when its registration fails or the meter provider is never initialised
	m := &imageDataCacheMetrics{logger: logr.Discard()}
	assert.NotPanics(t, func() { m.RecordRequest(context.Background(), ImageDataCacheHit) })
}

// not parallel: it swaps the package level manager.
func TestGetImageDataCacheMetrics(t *testing.T) {
	previous := GetManager()
	t.Cleanup(func() { SetManager(previous) })

	SetManager(nil)
	assert.Nil(t, GetImageDataCacheMetrics(), "callers rely on nil to skip recording when metrics are disabled")

	config := NewMetricsConfigManager(logr.Discard(), kconfig.NewDefaultMetricsConfiguration())
	SetManager(config)
	assert.Same(t, config.datacacheMetrics, GetImageDataCacheMetrics())
}
