package celexception

import (
	"context"
	"time"

	"github.com/go-logr/logr"
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
	var warning string
	if !h.validationOptions.Enabled {
		warning = validation.DisabledPolex
	}
	errs := polex.Validate()
	return admissionutils.Response(request.UID, errs.ToAggregate(), warning)
}
