package admission

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	validation "github.com/kyverno/kyverno/pkg/validation/cleanuppolicy"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
)

type cleanupHandlers struct {
	client dclient.Interface
}

func New(client dclient.Interface) *cleanupHandlers {
	return &cleanupHandlers{
		client: client,
	}
}

func (h *cleanupHandlers) Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ time.Time) handlers.AdmissionResponse {
	policy, _, err := admissionutils.GetCleanupPolicies(request.AdmissionRequest)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.Response(request.UID, err)
	}
	if err := validation.Validate(ctx, logger, h.client, policy); err != nil {
		logger.Error(err, "policy validation errors")
		return admissionutils.Response(request.UID, err)
	}
	// ttlLabel, err := admissionutils.GetTtlLabel(request.AdmissionRequest.Object.Raw)
	// if err != nil {
	// 	logger.Error(err, "failed to get the ttl label")
	// 	// return admissionutils.Response(request.UID, err)
	// }

	// if err := admissionutils.ValidateTTL(ttlLabel); err != nil {
	// 	logger.Error(err, "failed to unmarshal the ttl label value")
	// 	return admissionutils.Response(request.UID, err)
	// }
	return admissionutils.ResponseSuccess(request.UID)
}
