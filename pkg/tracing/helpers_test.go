package tracing

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func TestSetHttpStatusSuccess(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test-span")

	SetHttpStatus(ctx, nil, 200)
	span.End()

	spans := exporter.GetSpans()
	assert.Len(t, spans, 1)
	assert.Equal(t, codes.Ok, spans[0].Status.Code)

	foundAttr := false
	for _, attr := range spans[0].Attributes {
		if attr.Key == semconv.HTTPStatusCodeKey {
			assert.Equal(t, int64(200), attr.Value.AsInt64())
			foundAttr = true
			break
		}
	}
	assert.True(t, foundAttr, "HTTP status code attribute not found")
}

func TestSetHttpStatusError(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test-span")

	testErr := errors.New("test error")
	SetHttpStatus(ctx, testErr, 500)
	span.End()

	spans := exporter.GetSpans()
	assert.Len(t, spans, 1)
	assert.Equal(t, codes.Error, spans[0].Status.Code)
	assert.Equal(t, "Internal Server Error", spans[0].Status.Description)

	foundAttr := false
	for _, attr := range spans[0].Attributes {
		if attr.Key == semconv.HTTPStatusCodeKey {
			assert.Equal(t, int64(500), attr.Value.AsInt64())
			foundAttr = true
			break
		}
	}
	assert.True(t, foundAttr, "HTTP status code attribute not found")

	assert.Len(t, spans[0].Events, 1)
	assert.Equal(t, "exception", spans[0].Events[0].Name)
}

func TestSetSpanStatusWithError(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	_, span := tracer.Start(context.Background(), "test-span")

	testErr := errors.New("test error")
	SetSpanStatus(span, testErr)
	span.End()

	spans := exporter.GetSpans()
	assert.Len(t, spans, 1)
	assert.Equal(t, codes.Error, spans[0].Status.Code)
	assert.Equal(t, "test error", spans[0].Status.Description)
	assert.Len(t, spans[0].Events, 1)
}

func TestSetSpanStatusWithoutError(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	_, span := tracer.Start(context.Background(), "test-span")

	SetSpanStatus(span, nil)
	span.End()

	spans := exporter.GetSpans()
	assert.Len(t, spans, 1)
	assert.Equal(t, codes.Ok, spans[0].Status.Code)
	assert.Equal(t, "", spans[0].Status.Description)
	assert.Len(t, spans[0].Events, 0)
}

func TestSetStatus(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test-span")

	testErr := errors.New("context error")
	SetStatus(ctx, testErr)
	span.End()

	spans := exporter.GetSpans()
	assert.Len(t, spans, 1)
	assert.Equal(t, codes.Error, spans[0].Status.Code)
	assert.Equal(t, "context error", spans[0].Status.Description)
}

func TestIsInSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	assert.True(t, IsInSpan(ctx))
	assert.False(t, IsInSpan(context.Background()))
}

func TestCurrentSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	currentSpan := CurrentSpan(ctx)
	assert.NotNil(t, currentSpan)
	assert.True(t, currentSpan.IsRecording())
}

func TestSetAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test-span")

	SetAttributes(ctx, PolicyNameKey.String("test-policy"), RuleNameKey.String("test-rule"))
	span.End()

	spans := exporter.GetSpans()
	assert.Len(t, spans, 1)

	attrs := spans[0].Attributes
	foundPolicy := false
	foundRule := false
	for _, attr := range attrs {
		if attr.Key == PolicyNameKey {
			assert.Equal(t, "test-policy", attr.Value.AsString())
			foundPolicy = true
		}
		if attr.Key == RuleNameKey {
			assert.Equal(t, "test-rule", attr.Value.AsString())
			foundRule = true
		}
	}
	assert.True(t, foundPolicy, "policy name attribute not found")
	assert.True(t, foundRule, "rule name attribute not found")
}
