package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	// Semantic span names
	PolicyProcessingSpan   = "policy.processing"
	RuleEvaluationSpan     = "rule.evaluation"
	ResourceProcessingSpan = "resource.processing"
	CacheOperationSpan     = "cache.operation"
	AdmissionRequestSpan   = "admission.request"
	ValidationSpan         = "validation.check"
	MutationSpan           = "mutation.apply"
	GenerationSpan         = "generation.create"
)

// SemanticTracer provides enhanced tracing capabilities with semantic conventions
type SemanticTracer struct {
	tracer trace.Tracer
}

// NewSemanticTracer creates a new semantic tracer instance
func NewSemanticTracer() *SemanticTracer {
	tracerProvider := otel.GetTracerProvider()
	if tracerProvider == nil {
		// Return a tracer with no-op tracer
		return &SemanticTracer{
			tracer: nil,
		}
	}

	tracer := tracerProvider.Tracer("kyverno-semantic")
	return &SemanticTracer{
		tracer: tracer,
	}
}

// PolicySpanOptions contains options for policy tracing spans
type PolicySpanOptions struct {
	PolicyName      string
	PolicyNamespace string
	PolicyType      string
	RuleName        string
	Operation       string
}

// TracePolicy creates a span for policy processing with enhanced semantic attributes
func (st *SemanticTracer) TracePolicy(ctx context.Context, opts PolicySpanOptions, fn func(context.Context, trace.Span)) {
	if st.tracer == nil {
		// No tracer available, call function with original context and no-op span
		fn(ctx, trace.SpanFromContext(ctx))
		return
	}

	spanName := fmt.Sprintf("%s.%s", PolicyProcessingSpan, opts.Operation)

	attributes := []attribute.KeyValue{
		PolicyNameKey.String(opts.PolicyName),
		PolicyNamespaceKey.String(opts.PolicyNamespace),
		attribute.String("policy.type", opts.PolicyType),
	}

	if opts.RuleName != "" {
		attributes = append(attributes, RuleNameKey.String(opts.RuleName))
	}

	ctx, span := st.tracer.Start(ctx, spanName, trace.WithAttributes(attributes...))
	defer span.End()

	fn(ctx, span)
}

// TraceRule creates a span for rule evaluation with detailed context
func (st *SemanticTracer) TraceRule(ctx context.Context, ruleName, ruleType string, fn func(context.Context, trace.Span)) {
	if st.tracer == nil {
		fn(ctx, trace.SpanFromContext(ctx))
		return
	}

	spanName := fmt.Sprintf("%s.%s", RuleEvaluationSpan, ruleType)

	attributes := []attribute.KeyValue{
		RuleNameKey.String(ruleName),
		attribute.String("rule.type", ruleType),
	}

	ctx, span := st.tracer.Start(ctx, spanName, trace.WithAttributes(attributes...))
	defer span.End()

	fn(ctx, span)
}

// TraceResource creates a span for resource processing
func (st *SemanticTracer) TraceResource(ctx context.Context, resourceKind, resourceName, resourceNamespace string, fn func(context.Context, trace.Span)) {
	if st.tracer == nil {
		fn(ctx, trace.SpanFromContext(ctx))
		return
	}

	attributes := []attribute.KeyValue{
		attribute.String("resource.kind", resourceKind),
		attribute.String("resource.name", resourceName),
		attribute.String("resource.namespace", resourceNamespace),
	}

	ctx, span := st.tracer.Start(ctx, ResourceProcessingSpan, trace.WithAttributes(attributes...))
	defer span.End()

	fn(ctx, span)
}

// TraceCacheOperation creates a span for cache operations with detailed metrics
func (st *SemanticTracer) TraceCacheOperation(ctx context.Context, operation, cacheType, key string, hit bool, fn func(context.Context, trace.Span)) {
	if st.tracer == nil {
		fn(ctx, trace.SpanFromContext(ctx))
		return
	}

	spanName := fmt.Sprintf("%s.%s", CacheOperationSpan, operation)

	attributes := []attribute.KeyValue{
		attribute.String("cache.type", cacheType),
		attribute.String("cache.key", key),
		attribute.String("cache.operation", operation),
		attribute.Bool("cache.hit", hit),
	}

	ctx, span := st.tracer.Start(ctx, spanName, trace.WithAttributes(attributes...))
	defer span.End()

	fn(ctx, span)
}

// TraceAdmissionRequest creates a span for admission request processing
func (st *SemanticTracer) TraceAdmissionRequest(ctx context.Context, requestUID, operation, resourceKind string, fn func(context.Context, trace.Span)) {
	if st.tracer == nil {
		fn(ctx, trace.SpanFromContext(ctx))
		return
	}

	attributes := []attribute.KeyValue{
		RequestUidKey.String(requestUID),
		RequestOperationKey.String(operation),
		attribute.String("request.resource.kind", resourceKind),
	}

	ctx, span := st.tracer.Start(ctx, AdmissionRequestSpan, trace.WithAttributes(attributes...))
	defer span.End()

	fn(ctx, span)
}

// TraceValidation creates a span for validation operations
func (st *SemanticTracer) TraceValidation(ctx context.Context, validationType string, fn func(context.Context, trace.Span) error) error {
	if st.tracer == nil {
		return fn(ctx, trace.SpanFromContext(ctx))
	}

	spanName := fmt.Sprintf("%s.%s", ValidationSpan, validationType)

	attributes := []attribute.KeyValue{
		attribute.String("validation.type", validationType),
	}

	ctx, span := st.tracer.Start(ctx, spanName, trace.WithAttributes(attributes...))
	defer span.End()

	err := fn(ctx, span)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "validation successful")
	}

	return err
}

// TraceMutation creates a span for mutation operations
func (st *SemanticTracer) TraceMutation(ctx context.Context, mutationType string, fn func(context.Context, trace.Span) error) error {
	if st.tracer == nil {
		return fn(ctx, trace.SpanFromContext(ctx))
	}

	spanName := fmt.Sprintf("%s.%s", MutationSpan, mutationType)

	attributes := []attribute.KeyValue{
		attribute.String("mutation.type", mutationType),
	}

	ctx, span := st.tracer.Start(ctx, spanName, trace.WithAttributes(attributes...))
	defer span.End()

	err := fn(ctx, span)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "mutation successful")
	}

	return err
}

// TraceGeneration creates a span for resource generation operations
func (st *SemanticTracer) TraceGeneration(ctx context.Context, targetKind, targetName string, fn func(context.Context, trace.Span) error) error {
	if st.tracer == nil {
		return fn(ctx, trace.SpanFromContext(ctx))
	}

	attributes := []attribute.KeyValue{
		attribute.String("generation.target.kind", targetKind),
		attribute.String("generation.target.name", targetName),
	}

	ctx, span := st.tracer.Start(ctx, GenerationSpan, trace.WithAttributes(attributes...))
	defer span.End()

	err := fn(ctx, span)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "generation successful")
	}

	return err
}

// AddEventToSpan adds a structured event to the current span
func (st *SemanticTracer) AddEventToSpan(ctx context.Context, eventName string, attributes ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(eventName, trace.WithAttributes(attributes...))
	}
}

// SetSpanAttribute sets an attribute on the current span
func (st *SemanticTracer) SetSpanAttribute(ctx context.Context, key string, value interface{}) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		switch v := value.(type) {
		case string:
			span.SetAttributes(attribute.String(key, v))
		case int:
			span.SetAttributes(attribute.Int(key, v))
		case int64:
			span.SetAttributes(attribute.Int64(key, v))
		case bool:
			span.SetAttributes(attribute.Bool(key, v))
		case float64:
			span.SetAttributes(attribute.Float64(key, v))
		default:
			span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", v)))
		}
	}
}

// RecordEvent records a structured event with attributes
func (st *SemanticTracer) RecordEvent(ctx context.Context, eventType, message string, attributes map[string]interface{}) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		attrs := make([]attribute.KeyValue, 0, len(attributes))
		for k, v := range attributes {
			switch val := v.(type) {
			case string:
				attrs = append(attrs, attribute.String(k, val))
			case int:
				attrs = append(attrs, attribute.Int(k, val))
			case int64:
				attrs = append(attrs, attribute.Int64(k, val))
			case bool:
				attrs = append(attrs, attribute.Bool(k, val))
			case float64:
				attrs = append(attrs, attribute.Float64(k, val))
			default:
				attrs = append(attrs, attribute.String(k, fmt.Sprintf("%v", val)))
			}
		}

		attrs = append(attrs,
			attribute.String("event.type", eventType),
			attribute.String("event.message", message),
		)

		span.AddEvent("kyverno.event", trace.WithAttributes(attrs...))
	}
}
