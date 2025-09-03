package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
)

func (inner AdmissionHandler) WithMetrics(logger logr.Logger, attrs ...attribute.KeyValue) AdmissionHandler {
	return inner.withMetrics(attrs...).WithTrace("METRICS")
}

func (inner AdmissionHandler) withMetrics(attrs ...attribute.KeyValue) AdmissionHandler {
	metrics := metrics.GetAdmissionMetrics()
	if metrics == nil {
		return inner
	}

	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		response := inner(ctx, logger, request, startTime)
		metrics.RecordRequest(ctx, response.Allowed, request.Namespace, request.Operation, request.GroupVersionKind, startTime, attrs...)
		return response
	}
}

func (inner HttpHandler) WithMetrics(logger logr.Logger, attrs ...attribute.KeyValue) HttpHandler {
	return inner.withMetrics(attrs...).WithTrace("METRICS")
}

func (inner HttpHandler) withMetrics(attrs ...attribute.KeyValue) HttpHandler {
	metrics := metrics.GetHTTPMetrics()
	if metrics == nil {
		return inner
	}

	return func(writer http.ResponseWriter, request *http.Request) {
		startTime := time.Now()
		metrics.RecordRequest(request.Context(), request.Method, request.RequestURI, startTime, attrs...)
		inner(writer, request)
	}
}
