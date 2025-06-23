package globalcontext

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	validation "github.com/kyverno/kyverno/pkg/validation/globalcontext"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
)

type gctxHandlers struct{}

func NewHandlers() *gctxHandlers {
	return &gctxHandlers{}
}

// Validate performs the validation check on global context entries
func (h *gctxHandlers) Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ string, startTime time.Time) handlers.AdmissionResponse {
	gctx, _, err := admissionutils.GetGlobalContextEntry(request.AdmissionRequest)
	if err != nil {
		logger.Error(err, "failed to unmarshal global context entry from admission request")
		return admissionutils.Response(request.UID, err)
	}
	warnings, err := validation.Validate(ctx, logger, gctx)
	if err != nil {
		logger.Error(err, "global context entry validation errors")
	}
	return admissionutils.Response(request.UID, err, warnings...)
}
