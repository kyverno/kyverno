package exception

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	validation "github.com/kyverno/kyverno/pkg/validation/exception"
	"github.com/kyverno/kyverno/pkg/webhooks"
	admissionv1 "k8s.io/api/admission/v1"
)

type handlers struct {
	polexOptions webhooks.ExceptionOptions
}

func NewHandlers(po webhooks.ExceptionOptions) webhooks.ExceptionHandlers {
	return &handlers{polexOptions: po}
}

// Validate performs the validation check on policy exception resources
func (h *handlers) Validate(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
	polex, _, err := admissionutils.GetPolicyExceptions(request)
	if err != nil {
		logger.Error(err, "failed to unmarshal policy exceptions from admission request")
		return admissionutils.Response(request.UID, err)
	}
	err, warnings := validation.Validate(ctx, logger, polex, h.polexOptions)
	if err != nil {
		logger.Error(err, "policy exception validation errors")
	}
	return admissionutils.Response(request.UID, err, warnings...)
}
