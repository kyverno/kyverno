package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// StartSpan creates a span from a context with `operationName` name
func StartSpan(
	ctx context.Context,
	tracerName string,
	operationName string,
	opts ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, operationName, opts...)
}

// Span executes function doFn inside new span with `operationName` name and hooking as child to a span found within given context if any.
func Span(
	ctx context.Context,
	tracerName string,
	operationName string,
	doFn func(context.Context, trace.Span),
	opts ...trace.SpanStartOption,
) {
	ctx, span := StartSpan(ctx, tracerName, operationName, opts...)
	defer span.End()
	doFn(ctx, span)
}

// Span executes function doFn inside new span with `operationName` name and hooking as child to a span found within given context if any.
func Span1[T1 any](
	ctx context.Context,
	tracerName string,
	operationName string,
	doFn func(context.Context, trace.Span) T1,
	opts ...trace.SpanStartOption,
) T1 {
	ctx, span := StartSpan(ctx, tracerName, operationName, opts...)
	defer span.End()
	return doFn(ctx, span)
}

// Span executes function doFn inside new span with `operationName` name and hooking as child to a span found within given context if any.
func Span2[T1 any, T2 any](
	ctx context.Context,
	tracerName string,
	operationName string,
	doFn func(context.Context, trace.Span) (T1, T2),
	opts ...trace.SpanStartOption,
) (T1, T2) {
	ctx, span := StartSpan(ctx, tracerName, operationName, opts...)
	defer span.End()
	return doFn(ctx, span)
}

// Span executes function doFn inside new span with `operationName` name and hooking as child to a span found within given context if any.
func Span3[T1 any, T2 any, T3 any](
	ctx context.Context,
	tracerName string,
	operationName string,
	doFn func(context.Context, trace.Span) (T1, T2, T3),
	opts ...trace.SpanStartOption,
) (T1, T2, T3) {
	ctx, span := StartSpan(ctx, tracerName, operationName, opts...)
	defer span.End()
	return doFn(ctx, span)
}
