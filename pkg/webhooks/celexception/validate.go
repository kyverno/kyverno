package celexception

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
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
	polex, _, err := admissionutils.GetCELPolicyExceptions(request.AdmissionRequest)
	if err != nil {
		logger.Error(err, "failed to unmarshal CEL PolicyExceptions from admission request")
		return admissionutils.Response(request.UID, err)
	}
	warnings := validation.ValidateNamespace(ctx, logger, polex.GetNamespace(), h.validationOptions)
	warnings = append(warnings, validation.ValidateCompensatingControls(&polex.Spec)...)
	errs := polex.Validate()
	errs = append(errs, validation.ValidateCompensatingControlExpressions(polex)...)
	env, err := vpolcompiler.NewExceptionEnv(polex.GetNamespace())
	if err != nil {
		logger.Error(err, "failed to build the CEL PolicyException environment")
		return admissionutils.Response(request.UID, err)
	}
	errs = append(errs, compiler.CompilePolicyExceptionMatchConditions(env, polex.Spec.MatchConditions)...)
	return admissionutils.Response(request.UID, errs.ToAggregate(), warnings...)
}
