package metrics

import (
	"context"
	"runtime"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ResourceMetrics provides advanced resource monitoring capabilities
type ResourceMetrics struct {
	logger logr.Logger
	meter  metric.Meter

	// Memory and runtime metrics
	memoryUsageGauge    metric.Int64ObservableGauge
	goroutineCountGauge metric.Int64ObservableGauge
	gcCountCounter      metric.Int64Counter

	// Cache metrics
	cacheHitCounter  metric.Int64Counter
	cacheMissCounter metric.Int64Counter
	cacheSizeGauge   metric.Int64ObservableGauge

	// Admission queue metrics
	queueDepthGauge       metric.Int64ObservableGauge
	processingTimeHisto   metric.Float64Histogram
	admissionLatencyHisto metric.Float64Histogram

	// System health indicators
	healthStatusGauge metric.Int64ObservableGauge
}

// NewResourceMetrics creates a new ResourceMetrics instance
func NewResourceMetrics(logger logr.Logger) (*ResourceMetrics, error) {
	meter := otel.GetMeterProvider().Meter(MeterName)

	rm := &ResourceMetrics{
		logger: logger,
		meter:  meter,
	}

	var err error

	// Initialize memory and runtime metrics
	rm.memoryUsageGauge, err = meter.Int64ObservableGauge(
		"kyverno_memory_usage_bytes",
		metric.WithDescription("Current memory usage in bytes"),
	)
	if err != nil {
		return nil, err
	}

	rm.goroutineCountGauge, err = meter.Int64ObservableGauge(
		"kyverno_goroutine_count",
		metric.WithDescription("Current number of goroutines"),
	)
	if err != nil {
		return nil, err
	}

	rm.gcCountCounter, err = meter.Int64Counter(
		"kyverno_gc_count_total",
		metric.WithDescription("Total number of garbage collection cycles"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize cache metrics
	rm.cacheHitCounter, err = meter.Int64Counter(
		"kyverno_cache_hits_total",
		metric.WithDescription("Total number of cache hits"),
	)
	if err != nil {
		return nil, err
	}

	rm.cacheMissCounter, err = meter.Int64Counter(
		"kyverno_cache_misses_total",
		metric.WithDescription("Total number of cache misses"),
	)
	if err != nil {
		return nil, err
	}

	rm.cacheSizeGauge, err = meter.Int64ObservableGauge(
		"kyverno_cache_size",
		metric.WithDescription("Current cache size"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize admission queue metrics
	rm.queueDepthGauge, err = meter.Int64ObservableGauge(
		"kyverno_admission_queue_depth",
		metric.WithDescription("Current admission request queue depth"),
	)
	if err != nil {
		return nil, err
	}

	rm.processingTimeHisto, err = meter.Float64Histogram(
		"kyverno_processing_time_seconds",
		metric.WithDescription("Time spent processing requests"),
	)
	if err != nil {
		return nil, err
	}

	rm.admissionLatencyHisto, err = meter.Float64Histogram(
		"kyverno_admission_latency_seconds",
		metric.WithDescription("Admission request latency"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize system health metrics
	rm.healthStatusGauge, err = meter.Int64ObservableGauge(
		"kyverno_health_status",
		metric.WithDescription("System health status (1=healthy, 0=unhealthy)"),
	)
	if err != nil {
		return nil, err
	}

	// Register observable gauge callbacks
	_, err = meter.RegisterCallback(
		rm.collectRuntimeMetrics,
		rm.memoryUsageGauge,
		rm.goroutineCountGauge,
		rm.cacheSizeGauge,
		rm.queueDepthGauge,
		rm.healthStatusGauge,
	)
	if err != nil {
		return nil, err
	}

	return rm, nil
}

// collectRuntimeMetrics collects runtime and system metrics
func (rm *ResourceMetrics) collectRuntimeMetrics(ctx context.Context, observer metric.Observer) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Memory metrics
	observer.ObserveInt64(rm.memoryUsageGauge, int64(m.Alloc))

	// Goroutine count
	observer.ObserveInt64(rm.goroutineCountGauge, int64(runtime.NumGoroutine()))

	// Cache size (placeholder - would be actual cache size in real implementation)
	observer.ObserveInt64(rm.cacheSizeGauge, 100)

	// Queue depth (placeholder)
	observer.ObserveInt64(rm.queueDepthGauge, 0)

	// Health status (placeholder)
	observer.ObserveInt64(rm.healthStatusGauge, 1)

	return nil
}

// RecordCacheHit records a cache hit
func (rm *ResourceMetrics) RecordCacheHit(ctx context.Context, cacheType string) {
	rm.cacheHitCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("cache_type", cacheType),
	))
}

// RecordCacheMiss records a cache miss
func (rm *ResourceMetrics) RecordCacheMiss(ctx context.Context, cacheType string) {
	rm.cacheMissCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("cache_type", cacheType),
	))
}

// RecordProcessingTime records processing time for a specific operation
func (rm *ResourceMetrics) RecordProcessingTime(ctx context.Context, operation string, duration time.Duration) {
	rm.processingTimeHisto.Record(ctx, duration.Seconds(), metric.WithAttributes(
		attribute.String("operation", operation),
	))
}

// RecordAdmissionLatency records admission request latency
func (rm *ResourceMetrics) RecordAdmissionLatency(ctx context.Context, latency time.Duration, resource string) {
	rm.admissionLatencyHisto.Record(ctx, latency.Seconds(), metric.WithAttributes(
		attribute.String("resource", resource),
	))
}

// RecordGCCycle records a garbage collection cycle
func (rm *ResourceMetrics) RecordGCCycle(ctx context.Context) {
	rm.gcCountCounter.Add(ctx, 1)
}
