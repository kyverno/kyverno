package metrics

import (
	"context"
	"math"
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
	// Check if a proper meter provider is available
	meterProvider := otel.GetMeterProvider()
	if meterProvider == nil {
		logger.Info("OpenTelemetry meter provider not available, creating no-op ResourceMetrics")
		return &ResourceMetrics{
			logger: logger,
			// Leave all metrics nil - they will be safely ignored
		}, nil
	}

	meter := meterProvider.Meter(MeterName)
	if meter == nil {
		logger.Info("OpenTelemetry meter not available, creating no-op ResourceMetrics")
		return &ResourceMetrics{
			logger: logger,
		}, nil
	}

	rm := &ResourceMetrics{
		logger: logger,
		meter:  meter,
	}

	var err error

	// Initialize memory and runtime metrics with defensive error handling
	rm.memoryUsageGauge, err = meter.Int64ObservableGauge(
		"kyverno_memory_usage_bytes",
		metric.WithDescription("Current memory usage in bytes"),
	)
	if err != nil {
		logger.Error(err, "Failed to create memory usage gauge, continuing without it")
		rm.memoryUsageGauge = nil
	}

	rm.goroutineCountGauge, err = meter.Int64ObservableGauge(
		"kyverno_goroutine_count",
		metric.WithDescription("Current number of goroutines"),
	)
	if err != nil {
		logger.Error(err, "Failed to create goroutine count gauge, continuing without it")
		rm.goroutineCountGauge = nil
	}

	rm.gcCountCounter, err = meter.Int64Counter(
		"kyverno_gc_count_total",
		metric.WithDescription("Total number of garbage collection cycles"),
	)
	if err != nil {
		logger.Error(err, "Failed to create GC count counter, continuing without it")
		rm.gcCountCounter = nil
	}

	// Initialize cache metrics
	rm.cacheHitCounter, err = meter.Int64Counter(
		"kyverno_cache_hits_total",
		metric.WithDescription("Total number of cache hits"),
	)
	if err != nil {
		logger.Error(err, "Failed to create cache hit counter, continuing without it")
		rm.cacheHitCounter = nil
	}

	rm.cacheMissCounter, err = meter.Int64Counter(
		"kyverno_cache_misses_total",
		metric.WithDescription("Total number of cache misses"),
	)
	if err != nil {
		logger.Error(err, "Failed to create cache miss counter, continuing without it")
		rm.cacheMissCounter = nil
	}

	rm.cacheSizeGauge, err = meter.Int64ObservableGauge(
		"kyverno_cache_size",
		metric.WithDescription("Current cache size"),
	)
	if err != nil {
		logger.Error(err, "Failed to create cache size gauge, continuing without it")
		rm.cacheSizeGauge = nil
	}

	// Initialize admission queue metrics
	rm.queueDepthGauge, err = meter.Int64ObservableGauge(
		"kyverno_admission_queue_depth",
		metric.WithDescription("Current admission queue depth"),
	)
	if err != nil {
		logger.Error(err, "Failed to create queue depth gauge, continuing without it")
		rm.queueDepthGauge = nil
	}

	// Initialize processing time histogram
	rm.processingTimeHisto, err = meter.Float64Histogram(
		"kyverno_policy_processing_duration_seconds",
		metric.WithDescription("Time spent processing policies"),
		metric.WithUnit("s"),
	)
	if err != nil {
		logger.Error(err, "Failed to create processing time histogram, continuing without it")
		rm.processingTimeHisto = nil
	}

	// Initialize admission latency histogram
	rm.admissionLatencyHisto, err = meter.Float64Histogram(
		"kyverno_admission_latency_seconds",
		metric.WithDescription("Time spent processing admission requests"),
		metric.WithUnit("s"),
	)
	if err != nil {
		logger.Error(err, "Failed to create admission latency histogram, continuing without it")
		rm.admissionLatencyHisto = nil
	}

	// Initialize health status gauge
	rm.healthStatusGauge, err = meter.Int64ObservableGauge(
		"kyverno_health_status",
		metric.WithDescription("Overall health status (1=healthy, 0=unhealthy)"),
	)
	if err != nil {
		logger.Error(err, "Failed to create health status gauge, continuing without it")
		rm.healthStatusGauge = nil
	}

	// Register runtime metrics callback only if we have gauges available
	if rm.memoryUsageGauge != nil || rm.goroutineCountGauge != nil || rm.cacheSizeGauge != nil || rm.queueDepthGauge != nil || rm.healthStatusGauge != nil {
		_, err = meter.RegisterCallback(rm.collectRuntimeMetrics,
			rm.memoryUsageGauge,
			rm.goroutineCountGauge,
			rm.cacheSizeGauge,
			rm.queueDepthGauge,
			rm.healthStatusGauge,
		)
		if err != nil {
			logger.Error(err, "Failed to register runtime metrics callback, runtime metrics will not be available")
		}
	}

	return rm, nil
}

// safeUint64ToInt64 safely converts uint64 to int64, capping at MaxInt64 to prevent overflow
func safeUint64ToInt64(val uint64) int64 {
	if val > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(val)
}

// collectRuntimeMetrics collects runtime and system metrics
func (rm *ResourceMetrics) collectRuntimeMetrics(ctx context.Context, observer metric.Observer) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Memory metrics - use safe conversion to prevent integer overflow
	if rm.memoryUsageGauge != nil {
		observer.ObserveInt64(rm.memoryUsageGauge, safeUint64ToInt64(m.Alloc))
	}

	// Goroutine count
	if rm.goroutineCountGauge != nil {
		observer.ObserveInt64(rm.goroutineCountGauge, int64(runtime.NumGoroutine()))
	}

	// Cache size (placeholder - would be actual cache size in real implementation)
	if rm.cacheSizeGauge != nil {
		observer.ObserveInt64(rm.cacheSizeGauge, 100)
	}

	// Queue depth (placeholder)
	if rm.queueDepthGauge != nil {
		observer.ObserveInt64(rm.queueDepthGauge, 0)
	}

	// Health status (placeholder)
	if rm.healthStatusGauge != nil {
		observer.ObserveInt64(rm.healthStatusGauge, 1)
	}

	return nil
}

// RecordCacheHit records a cache hit event
func (rm *ResourceMetrics) RecordCacheHit(ctx context.Context, cacheType string) {
	if rm.cacheHitCounter != nil {
		rm.cacheHitCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("cache_type", cacheType),
		))
	}
}

// RecordCacheMiss records a cache miss event
func (rm *ResourceMetrics) RecordCacheMiss(ctx context.Context, cacheType string) {
	if rm.cacheMissCounter != nil {
		rm.cacheMissCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("cache_type", cacheType),
		))
	}
}

// RecordProcessingTime records the time spent processing a request
func (rm *ResourceMetrics) RecordProcessingTime(ctx context.Context, operationType string, duration time.Duration) {
	if rm.processingTimeHisto != nil {
		rm.processingTimeHisto.Record(ctx, duration.Seconds(), metric.WithAttributes(
			attribute.String("operation_type", operationType),
		))
	}
}

// RecordAdmissionLatency records the latency of admission request processing
func (rm *ResourceMetrics) RecordAdmissionLatency(ctx context.Context, duration time.Duration, resourceKind string) {
	if rm.admissionLatencyHisto != nil {
		rm.admissionLatencyHisto.Record(ctx, duration.Seconds(), metric.WithAttributes(
			attribute.String("resource_kind", resourceKind),
		))
	}
}

// RecordGCCycle records a garbage collection cycle
func (rm *ResourceMetrics) RecordGCCycle(ctx context.Context) {
	if rm.gcCountCounter != nil {
		rm.gcCountCounter.Add(ctx, 1)
	}
}
