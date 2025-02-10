package exception

import (
	"context"
	"time"

	"github.com/go-logr/logr"
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
	polex, _, err := admissionutils.GetPolicyExceptions(request.AdmissionRequest)
	if err != nil {
		logger.Error(err, "failed to unmarshal policy exceptions from admission request")
		return admissionutils.Response(request.UID, err)
	}
	warnings := validation.ValidateNamespace(ctx, logger, polex.GetNamespace(), h.validationOptions)
	errs := polex.Validate()
	return admissionutils.Response(request.UID, errs.ToAggregate(), warnings...)
}
