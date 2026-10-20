package exception

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	validation "github.com/kyverno/kyverno/pkg/validation/exception"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
)

type exceptionHandlers struct {
	validationOptions validation.ValidationOptions
}

func NewHandlers(validationOptions validation.ValidationOptions) *exceptionHandlers {
	return &exceptionHandlers{
		validationOptions: validationOptions,
	}
}

// Validate performs the validation check on policy exception resources
func (h *exceptionHandlers) Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ string, startTime time.Time) handlers.AdmissionResponse {
	// Subresource requests never touch spec, so there is nothing here to validate or warn
	// about; short-circuit before validation and deprecation warnings, not just the
	// legacy-policy block. Kubernetes guarantees a status-subresource write cannot change
	// spec, see:
	// https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#status-subresource
	if request.SubResource != "" {
		return admissionutils.ResponseSuccess(request.UID)
	}

	polex, _, err := admissionutils.GetPolicyExceptions(request.AdmissionRequest)
	if err != nil {
		logger.Error(err, "failed to unmarshal policy exceptions from admission request")
		return admissionutils.Response(request.UID, err)
	}
	// The legacy write denial (create/update of this kind) now happens one layer up, at the
	// route-level handlers.WithLegacyPolicyDenial() decorator (see
	// pkg/deprecations.DenyLegacyWrite), so this method only runs on requests it already
	// allowed through.
	warnings := validation.ValidateNamespace(ctx, logger, polex.GetNamespace(), h.validationOptions)
	if warning, ok := deprecations.BuildKindWarning(request.Kind.Group, request.Kind.Version, request.Kind.Kind); ok {
		warnings = append(warnings, warning.Message)
		if deprecatedMetric := metrics.GetDeprecatedAPIRequestMetrics(); deprecatedMetric != nil {
			deprecatedMetric.Record(ctx, request.Namespace, warning.Group, warning.Version, warning.Kind, "")
		}
	}
	errs := polex.Validate()
	return admissionutils.Response(request.UID, errs.ToAggregate(), warnings...)
}
