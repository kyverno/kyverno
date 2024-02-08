package policy

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov2alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2alpha1"
	kyvernov2alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	policyvalidate "github.com/kyverno/kyverno/pkg/validation/policy"
	"github.com/kyverno/kyverno/pkg/webhooks"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
)

type policyHandlers struct {
	client                       dclient.Interface
	gctxentryLister              kyvernov2alpha1listers.GlobalContextEntryLister
	backgroundServiceAccountName string
}

func NewHandlers(client dclient.Interface, gctxentryInformer kyvernov2alpha1informers.GlobalContextEntryInformer, serviceaccount string) webhooks.PolicyHandlers {
	return &policyHandlers{
		client:                       client,
		gctxentryLister:              gctxentryInformer.Lister(),
		backgroundServiceAccountName: serviceaccount,
	}
}

func (h *policyHandlers) Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ time.Time) handlers.AdmissionResponse {
	policy, oldPolicy, err := admissionutils.GetPolicies(request.AdmissionRequest)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.Response(request.UID, err)
	}
	warnings, err := policyvalidate.Validate(policy, oldPolicy, h.client, h.gctxentryLister, false, h.backgroundServiceAccountName)
	if err != nil {
		logger.Error(err, "policy validation errors")
	}
	return admissionutils.Response(request.UID, err, warnings...)
}

func (h *policyHandlers) Mutate(_ context.Context, _ logr.Logger, request handlers.AdmissionRequest, _ time.Time) handlers.AdmissionResponse {
	return admissionutils.ResponseSuccess(request.UID)
}
