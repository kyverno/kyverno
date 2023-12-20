package tracing

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

func SetSpanStatus(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
}

func SetStatus(ctx context.Context, err error) {
	SetSpanStatus(trace.SpanFromContext(ctx), err)
}

func SetHttpStatus(ctx context.Context, err error, code int) {
	span := trace.SpanFromContext(ctx)
	if err != nil {
		span.RecordError(err)
	}
	span.SetAttributes(semconv.HTTPStatusCodeKey.Int(code))
	if code >= 400 {
		span.SetStatus(codes.Error, http.StatusText(code))
	} else {
		span.SetStatus(codes.Ok, http.StatusText(code))
	}
}

func IsInSpan(ctx context.Context) bool {
	span := trace.SpanFromContext(ctx)
	return span.IsRecording()
}

func CurrentSpan(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

func SetAttributes(ctx context.Context, kv ...attribute.KeyValue) {
	CurrentSpan(ctx).SetAttributes(kv...)
}
