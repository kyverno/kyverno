package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func GetImageDataCacheMetrics() ImageDataCacheMetrics {
	if metricsConfig == nil {
		return nil
	}
	return metricsConfig.ImageDataCacheMetrics()
}

type ImageDataCacheResult string

const (
	ImageDataCacheHit   ImageDataCacheResult = "hit"
	ImageDataCacheMiss  ImageDataCacheResult = "miss"
	ImageDataCacheError ImageDataCacheResult = "error"
)

type ImageDataCacheMetrics interface {
	RecordRequest(ctx context.Context, result ImageDataCacheResult)
}

type imageDataCacheMetrics struct {
	requestCounter metric.Int64Counter

	logger logr.Logger
}

func (m *imageDataCacheMetrics) init(meter metric.Meter) {
	var err error

	m.requestCounter, err = meter.Int64Counter(
		"kyverno_image_data_cache_requests",
		metric.WithDescription("can be used to track the lookups of the image data cache, by result (hit, miss or error; lookups that shared another lookup's registry call count as hits), to measure how many registry requests the cache saves."),
	)
	if err != nil {
		m.logger.Error(err, "failed to register metric kyverno_image_data_cache_requests")
	}
}

func (m *imageDataCacheMetrics) RecordRequest(ctx context.Context, result ImageDataCacheResult) {
	if m.requestCounter == nil {
		return
	}
	m.requestCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("result", string(result))))
}
