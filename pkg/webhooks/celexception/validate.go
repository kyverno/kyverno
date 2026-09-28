package celexception

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	validation "github.com/kyverno/kyverno/pkg/validation/exception"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
)

type celExceptionHandlers struct {
	validationOptions validation.ValidationOptions
}

func NewHandlers(validationOptions validation.ValidationOptions) *celExceptionHandlers {
	return &celExceptionHandlers{
		validationOptions: validationOptions,
	}
}

// Validate performs the validation check on CEL PolicyException resources
func (h *celExceptionHandlers) Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ string, startTime time.Time) handlers.AdmissionResponse {
	polex, oldPolex, err := admissionutils.GetCELPolicyExceptions(request.AdmissionRequest)
	if err != nil {
		logger.Error(err, "failed to unmarshal CEL PolicyExceptions from admission request")
		return admissionutils.Response(request.UID, err)
	}
	warnings := validation.ValidateNamespace(ctx, logger, polex.GetNamespace(), h.validationOptions)
	errs := polex.Validate()
	preexistingExpressions := make(map[string]bool)
	if oldPolex != nil {
		for _, condition := range oldPolex.Spec.MatchConditions {
			preexistingExpressions[condition.Expression] = true
		}
	}
	errs = append(errs, compiler.CompilePolicyExceptionMatchConditions(polex.Spec.MatchConditions, preexistingExpressions)...)
	return admissionutils.Response(request.UID, errs.ToAggregate(), warnings...)
}
