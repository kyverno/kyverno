package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/trace"
	admissionv1 "k8s.io/api/admission/v1"
)

func (h AdmissionHandler) WithFilter(configuration config.Configuration) AdmissionHandler {
	return withFilter(configuration, h)
}

func withFilter(c config.Configuration, inner AdmissionHandler) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		return tracing.Span1(
			ctx,
			"webhooks/handlers",
			fmt.Sprintf("FILTER %s %s", request.Operation, request.Kind),
			func(ctx context.Context, span trace.Span) *admissionv1.AdmissionResponse {
				if c.ToFilter(request.Kind.Kind, request.Namespace, request.Name) {
					return nil
				}
				return inner(ctx, logger, request, startTime)
			},
			trace.WithAttributes(admissionRequestAttributes(request)...),
		)
	}
}
