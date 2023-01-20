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

// ExceptionOptions holds flags to enable PolicyExceptions and in which namespace
type ExceptionOptions struct {
	// EnablePolicyException enables/disables PolicyExceptions
	EnablePolicyException bool
	// Namespace is the defined namespace
	Namespace string
}

type handlers struct {
	polexOptions ExceptionOptions
}

func NewHandlers(po ExceptionOptions) webhooks.ExceptionHandlers {
	return &handlers{polexOptions: po}
}

// Validate performs the validation check on policy exception resources
func (h *handlers) Validate(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
	polex, _, err := admissionutils.GetPolicyExceptions(request)
	if err != nil {
		logger.Error(err, "failed to unmarshal policy exceptions from admission request")
		return admissionutils.Response(request.UID, err)
	}
	err, warnings := validation.Validate(ctx, logger, polex, h.polexOptions.EnablePolicyException, h.polexOptions.Namespace)
	if err != nil {
		logger.Error(err, "policy exception validation errors")
	}
	return admissionutils.Response(request.UID, err, warnings...)
}
