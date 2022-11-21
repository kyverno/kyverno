package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/tracing"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"go.opentelemetry.io/otel/trace"
	admissionv1 "k8s.io/api/admission/v1"
)

func Verify() AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		return tracing.Span1(
			ctx,
			"webhooks/handlers",
			fmt.Sprintf("VERIFY %s %s", request.Operation, request.Kind),
			func(ctx context.Context, span trace.Span) *admissionv1.AdmissionResponse {
				if request.Name != "kyverno-health" || request.Namespace != config.KyvernoNamespace() {
					return admissionutils.ResponseSuccess()
				}
				patch := jsonutils.NewPatchOperation("/metadata/annotations/"+"kyverno.io~1last-request-time", "replace", time.Now().Format(time.RFC3339))
				bytes, err := patch.ToPatchBytes()
				if err != nil {
					logger.Error(err, "failed to build patch bytes")
					return admissionutils.Response(err)
				}
				return admissionutils.MutationResponse(bytes)
			},
			trace.WithAttributes(admissionRequestAttributes(request)...),
		)
	}
}
