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

// PolicyMetrics provides policy processing performance metrics
type PolicyMetrics struct {
	logger logr.Logger
	meter  metric.Meter

	// Policy processing metrics
	policyProcessingDuration metric.Float64Histogram
	ruleEvaluationDuration   metric.Float64Histogram
	policyEvaluationCounter  metric.Int64Counter

	// Cache metrics
	policyCacheHitCounter  metric.Int64Counter
	policyCacheMissCounter metric.Int64Counter
	policyCacheSizeGauge   metric.Int64ObservableGauge

	// Admission webhook metrics
	admissionRequestDuration metric.Float64Histogram
	admissionRequestCounter  metric.Int64Counter

	// Runtime metrics for system health
	memoryUsageGauge    metric.Int64ObservableGauge
	goroutineCountGauge metric.Int64ObservableGauge
}

// NewPolicyMetrics creates a new PolicyMetrics instance
func NewPolicyMetrics(logger logr.Logger) (*PolicyMetrics, error) {
	meter := otel.GetMeterProvider().Meter(MeterName)

	pm := &PolicyMetrics{
		logger: logger,
		meter:  meter,
	}

	var err error

	// Policy processing duration metrics
	pm.policyProcessingDuration, err = meter.Float64Histogram(
		"kyverno_policy_processing_duration_seconds",
		metric.WithDescription("Time spent processing policies by type and result"),
	)
	if err != nil {
		return nil, err
	}

	pm.ruleEvaluationDuration, err = meter.Float64Histogram(
		"kyverno_rule_evaluation_duration_seconds",
		metric.WithDescription("Time spent evaluating individual rules"),
	)
	if err != nil {
		return nil, err
	}

	pm.policyEvaluationCounter, err = meter.Int64Counter(
		"kyverno_policy_evaluations_total",
		metric.WithDescription("Total number of policy evaluations by type and result"),
	)
	if err != nil {
		return nil, err
	}

	// Cache performance metrics
	pm.policyCacheHitCounter, err = meter.Int64Counter(
		"kyverno_policy_cache_hits_total",
		metric.WithDescription("Total number of policy cache hits"),
	)
	if err != nil {
		return nil, err
	}

	pm.policyCacheMissCounter, err = meter.Int64Counter(
		"kyverno_policy_cache_misses_total",
		metric.WithDescription("Total number of policy cache misses"),
	)
	if err != nil {
		return nil, err
	}

	pm.policyCacheSizeGauge, err = meter.Int64ObservableGauge(
		"kyverno_policy_cache_size",
		metric.WithDescription("Current number of policies in cache"),
	)
	if err != nil {
		return nil, err
	}

	// Admission request metrics
	pm.admissionRequestDuration, err = meter.Float64Histogram(
		"kyverno_admission_request_duration_seconds",
		metric.WithDescription("Duration of admission request processing"),
	)
	if err != nil {
		return nil, err
	}

	pm.admissionRequestCounter, err = meter.Int64Counter(
		"kyverno_admission_requests_total",
		metric.WithDescription("Total number of admission requests processed"),
	)
	if err != nil {
		return nil, err
	}

	// Runtime metrics for system health monitoring
	pm.memoryUsageGauge, err = meter.Int64ObservableGauge(
		"kyverno_memory_usage_bytes",
		metric.WithDescription("Current memory usage in bytes"),
	)
	if err != nil {
		return nil, err
	}

	pm.goroutineCountGauge, err = meter.Int64ObservableGauge(
		"kyverno_goroutine_count",
		metric.WithDescription("Current number of goroutines"),
	)
	if err != nil {
		return nil, err
	}

	// Register observable gauge callbacks
	_, err = meter.RegisterCallback(
		pm.collectRuntimeMetrics,
		pm.memoryUsageGauge,
		pm.goroutineCountGauge,
		pm.policyCacheSizeGauge,
	)
	if err != nil {
		return nil, err
	}

	return pm, nil
}

// safeUint64ToInt64 safely converts uint64 to int64, capping at MaxInt64 to prevent overflow
func safeUint64ToInt64(val uint64) int64 {
	if val > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(val)
}

// collectRuntimeMetrics collects runtime and system metrics
func (pm *PolicyMetrics) collectRuntimeMetrics(ctx context.Context, observer metric.Observer) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Memory metrics - use safe conversion to prevent integer overflow
	observer.ObserveInt64(pm.memoryUsageGauge, safeUint64ToInt64(m.Alloc))

	// Goroutine count
	observer.ObserveInt64(pm.goroutineCountGauge, int64(runtime.NumGoroutine()))

	// Policy cache size - This would be populated by actual cache implementation
	// For now, placeholder value of 0 indicates no cache size tracking implemented yet
	observer.ObserveInt64(pm.policyCacheSizeGauge, 0)

	return nil
}

// RecordPolicyProcessingDuration records the time spent processing a policy
func (pm *PolicyMetrics) RecordPolicyProcessingDuration(ctx context.Context, duration time.Duration, policyType string, ruleType RuleType, result RuleResult) {
	pm.policyProcessingDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(
		attribute.String("policy_type", policyType),
		attribute.String("rule_type", string(ruleType)),
		attribute.String("result", string(result)),
	))
}

// RecordRuleEvaluationDuration records the time spent evaluating a specific rule
func (pm *PolicyMetrics) RecordRuleEvaluationDuration(ctx context.Context, duration time.Duration, policyName, ruleName string, ruleType RuleType) {
	pm.ruleEvaluationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(
		attribute.String("policy_name", policyName),
		attribute.String("rule_name", ruleName),
		attribute.String("rule_type", string(ruleType)),
	))
}

// RecordPolicyEvaluation records a policy evaluation event
func (pm *PolicyMetrics) RecordPolicyEvaluation(ctx context.Context, policyType string, ruleType RuleType, result RuleResult, cause RuleExecutionCause) {
	pm.policyEvaluationCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("policy_type", policyType),
		attribute.String("rule_type", string(ruleType)),
		attribute.String("result", string(result)),
		attribute.String("execution_cause", string(cause)),
	))
}

// RecordPolicyCacheHit records a policy cache hit
func (pm *PolicyMetrics) RecordPolicyCacheHit(ctx context.Context, cacheType string) {
	pm.policyCacheHitCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("cache_type", cacheType),
	))
}

// RecordPolicyCacheMiss records a policy cache miss
func (pm *PolicyMetrics) RecordPolicyCacheMiss(ctx context.Context, cacheType string) {
	pm.policyCacheMissCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("cache_type", cacheType),
	))
}

// RecordAdmissionRequestDuration records the duration of an admission request
func (pm *PolicyMetrics) RecordAdmissionRequestDuration(ctx context.Context, duration time.Duration, operation ResourceRequestOperation, resourceKind string, allowed bool) {
	pm.admissionRequestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(
		attribute.String("operation", string(operation)),
		attribute.String("resource_kind", resourceKind),
		attribute.Bool("allowed", allowed),
	))
}

// RecordAdmissionRequest records an admission request event
func (pm *PolicyMetrics) RecordAdmissionRequest(ctx context.Context, operation ResourceRequestOperation, resourceKind string, allowed bool) {
	pm.admissionRequestCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("operation", string(operation)),
		attribute.String("resource_kind", resourceKind),
		attribute.Bool("allowed", allowed),
	))
}

// SetPolicyCacheSize sets the current policy cache size (to be called by cache implementation)
func (pm *PolicyMetrics) SetPolicyCacheSize(size int64) {
	// This would be implemented by the actual cache to update the gauge
	// The gauge value is observed in the collectRuntimeMetrics callback
	pm.logger.V(6).Info("policy cache size updated", "size", size)
}
