package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// EnhancedPolicyProcessor demonstrates advanced usage of OpenTelemetry features
// in policy processing workflows
type EnhancedPolicyProcessor struct {
	logger          logr.Logger
	resourceMetrics *metrics.ResourceMetrics
	semanticTracer  *tracing.SemanticTracer
}

// PolicyRequest represents a policy processing request
type PolicyRequest struct {
	PolicyName        string
	PolicyNamespace   string
	PolicyType        string
	ResourceKind      string
	ResourceName      string
	ResourceNamespace string
	Operation         string
	RequestUID        string
}

// PolicyResult represents the result of policy processing
type PolicyResult struct {
	Allowed    bool
	Message    string
	Violations []string
	Applied    bool
	Duration   time.Duration
}

// NewEnhancedPolicyProcessor creates a new enhanced policy processor
func NewEnhancedPolicyProcessor(logger logr.Logger, resourceMetrics *metrics.ResourceMetrics) *EnhancedPolicyProcessor {
	return &EnhancedPolicyProcessor{
		logger:          logger,
		resourceMetrics: resourceMetrics,
		semanticTracer:  tracing.NewSemanticTracer(),
	}
}

// ProcessPolicy processes a policy with comprehensive observability
func (epp *EnhancedPolicyProcessor) ProcessPolicy(ctx context.Context, request PolicyRequest) (*PolicyResult, error) {
	startTime := time.Now()

	// Create admission request span
	var result *PolicyResult
	var err error

	epp.semanticTracer.TraceAdmissionRequest(ctx, request.RequestUID, request.Operation, request.ResourceKind, func(ctx context.Context, span trace.Span) {
		// Record admission latency
		defer func() {
			duration := time.Since(startTime)
			epp.resourceMetrics.RecordAdmissionLatency(ctx, duration, request.ResourceKind)
			epp.resourceMetrics.RecordProcessingTime(ctx, "policy_processing", duration)
		}()

		// Process policy with enhanced tracing
		result, err = epp.processWithTracing(ctx, request, startTime)
	})

	return result, err
}

// processWithTracing handles the actual policy processing with detailed tracing
func (epp *EnhancedPolicyProcessor) processWithTracing(ctx context.Context, request PolicyRequest, startTime time.Time) (*PolicyResult, error) {
	result := &PolicyResult{
		Allowed:    true,
		Violations: make([]string, 0),
	}

	// Trace the entire policy processing
	opts := tracing.PolicySpanOptions{
		PolicyName:      request.PolicyName,
		PolicyNamespace: request.PolicyNamespace,
		PolicyType:      request.PolicyType,
		Operation:       request.Operation,
	}

	epp.semanticTracer.TracePolicy(ctx, opts, func(ctx context.Context, span trace.Span) {
		// Add resource information to the span
		span.SetAttributes(
			attribute.String("resource.kind", request.ResourceKind),
			attribute.String("resource.name", request.ResourceName),
			attribute.String("resource.namespace", request.ResourceNamespace),
		)

		// Record event for policy processing start
		epp.semanticTracer.RecordEvent(ctx, "policy.processing.start", "Starting policy evaluation", map[string]interface{}{
			"policy_name":   request.PolicyName,
			"resource_kind": request.ResourceKind,
		})

		// Simulate policy cache lookup
		epp.lookupPolicyCache(ctx, request)

		// Process validation rules
		if err := epp.processValidationRules(ctx, request, result); err != nil {
			span.RecordError(err)
			result.Message = fmt.Sprintf("Validation failed: %v", err)
			result.Allowed = false
			return
		}

		// Process mutation rules
		if err := epp.processMutationRules(ctx, request, result); err != nil {
			span.RecordError(err)
			result.Message = fmt.Sprintf("Mutation failed: %v", err)
			result.Allowed = false
			return
		}

		// Process generation rules
		if err := epp.processGenerationRules(ctx, request, result); err != nil {
			span.RecordError(err)
			result.Message = fmt.Sprintf("Generation failed: %v", err)
		}

		// Calculate final duration
		result.Duration = time.Since(startTime)

		// Record successful completion
		epp.semanticTracer.RecordEvent(ctx, "policy.processing.complete", "Policy processing completed", map[string]interface{}{
			"allowed":          result.Allowed,
			"duration_ms":      result.Duration.Milliseconds(),
			"violations_count": len(result.Violations),
		})
	})

	return result, nil
}

// lookupPolicyCache simulates policy cache operations with detailed tracing
func (epp *EnhancedPolicyProcessor) lookupPolicyCache(ctx context.Context, request PolicyRequest) {
	cacheKey := fmt.Sprintf("%s/%s", request.PolicyNamespace, request.PolicyName)

	// Simulate cache lookup with tracing
	epp.semanticTracer.TraceCacheOperation(ctx, "get", "policy", cacheKey, true, func(ctx context.Context, span trace.Span) {
		// Simulate cache hit
		epp.resourceMetrics.RecordCacheHit(ctx, "policy")

		epp.semanticTracer.AddEventToSpan(ctx, "cache.hit",
			attribute.String("cache.key", cacheKey),
			attribute.String("cache.type", "policy"),
		)
	})
}

// processValidationRules processes validation rules with enhanced tracing
func (epp *EnhancedPolicyProcessor) processValidationRules(ctx context.Context, request PolicyRequest, result *PolicyResult) error {
	return epp.semanticTracer.TraceValidation(ctx, "admission", func(ctx context.Context, span trace.Span) error {
		// Simulate validation rule processing
		epp.semanticTracer.TraceRule(ctx, "validate-required-labels", "validation", func(ctx context.Context, span trace.Span) {
			// Simulate validation logic
			span.SetAttributes(
				attribute.String("validation.rule", "validate-required-labels"),
				attribute.Bool("validation.passed", true),
			)
		})

		// Simulate another validation rule
		epp.semanticTracer.TraceRule(ctx, "validate-resource-limits", "validation", func(ctx context.Context, span trace.Span) {
			// Simulate validation that finds a violation
			violation := "Resource limits not specified"
			result.Violations = append(result.Violations, violation)

			span.SetAttributes(
				attribute.String("validation.rule", "validate-resource-limits"),
				attribute.Bool("validation.passed", false),
				attribute.String("validation.violation", violation),
			)
		})

		return nil
	})
}

// processMutationRules processes mutation rules with enhanced tracing
func (epp *EnhancedPolicyProcessor) processMutationRules(ctx context.Context, request PolicyRequest, result *PolicyResult) error {
	return epp.semanticTracer.TraceMutation(ctx, "admission", func(ctx context.Context, span trace.Span) error {
		// Simulate mutation rule processing
		epp.semanticTracer.TraceRule(ctx, "add-default-labels", "mutation", func(ctx context.Context, span trace.Span) {
			span.SetAttributes(
				attribute.String("mutation.rule", "add-default-labels"),
				attribute.Bool("mutation.applied", true),
				attribute.Int("mutation.patches", 2),
			)
			result.Applied = true
		})

		return nil
	})
}

// processGenerationRules processes generation rules with enhanced tracing
func (epp *EnhancedPolicyProcessor) processGenerationRules(ctx context.Context, request PolicyRequest, result *PolicyResult) error {
	if request.ResourceKind == "Namespace" {
		return epp.semanticTracer.TraceGeneration(ctx, "NetworkPolicy", "default-deny", func(ctx context.Context, span trace.Span) error {
			// Simulate generation rule processing
			span.SetAttributes(
				attribute.String("generation.rule", "generate-network-policy"),
				attribute.String("generation.target.kind", "NetworkPolicy"),
				attribute.String("generation.target.name", "default-deny"),
				attribute.Bool("generation.created", true),
			)

			epp.semanticTracer.AddEventToSpan(ctx, "resource.generated",
				attribute.String("generated.kind", "NetworkPolicy"),
				attribute.String("generated.name", "default-deny"),
			)

			return nil
		})
	}

	return nil
}

// GetMetrics returns the resource metrics instance for external access
func (epp *EnhancedPolicyProcessor) GetMetrics() *metrics.ResourceMetrics {
	return epp.resourceMetrics
}

// GetTracer returns the semantic tracer instance for external access
func (epp *EnhancedPolicyProcessor) GetTracer() *tracing.SemanticTracer {
	return epp.semanticTracer
}

// ProcessBatch processes multiple policies in batch with aggregated metrics
func (epp *EnhancedPolicyProcessor) ProcessBatch(ctx context.Context, requests []PolicyRequest) ([]*PolicyResult, error) {
	startTime := time.Now()
	results := make([]*PolicyResult, len(requests))

	// Track batch processing
	epp.semanticTracer.AddEventToSpan(ctx, "batch.processing.start",
		attribute.Int("batch.size", len(requests)),
	)

	for i, request := range requests {
		result, err := epp.ProcessPolicy(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to process policy %s: %w", request.PolicyName, err)
		}
		results[i] = result
	}

	// Record batch completion metrics
	batchDuration := time.Since(startTime)
	epp.resourceMetrics.RecordProcessingTime(ctx, "batch_processing", batchDuration)

	epp.semanticTracer.AddEventToSpan(ctx, "batch.processing.complete",
		attribute.Int("batch.size", len(requests)),
		attribute.Int64("batch.duration_ms", batchDuration.Milliseconds()),
	)

	return results, nil
}
