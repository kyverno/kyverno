package policy

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	policyvalidate "github.com/kyverno/kyverno/pkg/validation/policy"
	"github.com/kyverno/kyverno/pkg/webhooks"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
)

type policyHandlers struct {
	client                       dclient.Interface
	kyvernoClient                versioned.Interface
	backgroundServiceAccountName string
}

func NewHandlers(client dclient.Interface, kyvernoClient versioned.Interface, serviceaccount string) webhooks.PolicyHandlers {
	return &policyHandlers{
		client:                       client,
		kyvernoClient:                kyvernoClient,
		backgroundServiceAccountName: serviceaccount,
	}
}

func (h *policyHandlers) Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ time.Time) handlers.AdmissionResponse {
	policy, oldPolicy, err := admissionutils.GetPolicies(request.AdmissionRequest)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.Response(request.UID, err)
	}
	warnings, err := policyvalidate.Validate(policy, oldPolicy, h.client, h.kyvernoClient, false, h.backgroundServiceAccountName)
	if err != nil {
		logger.Error(err, "policy validation errors")
	}
	return admissionutils.Response(request.UID, err, warnings...)
}

func (h *policyHandlers) Mutate(_ context.Context, _ logr.Logger, request handlers.AdmissionRequest, _ time.Time) handlers.AdmissionResponse {
	return admissionutils.ResponseSuccess(request.UID)
}
