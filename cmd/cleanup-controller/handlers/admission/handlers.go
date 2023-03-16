package admission

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	validation "github.com/kyverno/kyverno/pkg/validation/cleanuppolicy"
	admissionv1 "k8s.io/api/admission/v1"
)

type handlers struct {
	client dclient.Interface
}

func New(client dclient.Interface) *handlers {
	return &handlers{
		client: client,
	}
}

func (h *handlers) Validate(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, _ time.Time) *admissionv1.AdmissionResponse {
	policy, _, err := admissionutils.GetCleanupPolicies(request)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.Response(request.UID, err)
	}
	if err := validation.Validate(ctx, logger, h.client, policy); err != nil {
		logger.Error(err, "policy validation errors")
		return admissionutils.Response(request.UID, err)
	}
	return nil
}
