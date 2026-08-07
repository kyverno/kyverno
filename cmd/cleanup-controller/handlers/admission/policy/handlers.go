package policy

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/deprecations"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	validation "github.com/kyverno/kyverno/pkg/validation/cleanuppolicy"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
)

type validationHandlers struct {
	client dclient.Interface
}

func New(client dclient.Interface) *validationHandlers {
	return &validationHandlers{
		client: client,
	}
}

func (h *validationHandlers) Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ time.Time) handlers.AdmissionResponse {
	policy, _, err := admissionutils.GetCleanupPolicies(request.AdmissionRequest)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.Response(request.UID, err)
	}
	if err := validation.Validate(ctx, logger, h.client, policy); err != nil {
		logger.Error(err, "policy validation errors")
		return admissionutils.Response(request.UID, err)
	}
	if warning := deprecations.Warning(request.Kind.Kind); warning != "" {
		logger.V(2).Info(warning, "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name)
		return admissionutils.Response(request.UID, nil, warning)
	}
	return admissionutils.ResponseSuccess(request.UID)
}
