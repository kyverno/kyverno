package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/kyverno/kyverno/pkg/tracing"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

func (h HttpHandler) WithTrace() HttpHandler {
	return withTrace(h)
}

func withTrace(inner HttpHandler) HttpHandler {
	return func(writer http.ResponseWriter, request *http.Request) {
		tracing.Span(
			request.Context(),
			"webhooks/handlers",
			fmt.Sprintf("HTTP %s %s", request.Method, request.URL.Path),
			func(ctx context.Context, span trace.Span) {
				inner(writer, request.WithContext(ctx))
			},
			trace.WithAttributes(
				semconv.HTTPRequestContentLengthKey.Int64(request.ContentLength),
				semconv.HTTPHostKey.String(request.Host),
				semconv.HTTPMethodKey.String(request.Method),
				semconv.HTTPURLKey.String(request.RequestURI),
			),
			trace.WithSpanKind(trace.SpanKindServer),
		)
	}
}
