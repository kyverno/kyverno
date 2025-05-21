package policy

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	gpolvalidation "github.com/kyverno/kyverno/pkg/cel/policies/gpol"
	vpolvalidation "github.com/kyverno/kyverno/pkg/cel/policies/vpol"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	eval "github.com/kyverno/kyverno/pkg/imageverification/evaluator"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	policyvalidate "github.com/kyverno/kyverno/pkg/validation/policy"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
)

type policyHandlers struct {
	client                       dclient.Interface
	backgroundServiceAccountName string
	reportsServiceAccountName    string
}

func NewHandlers(client dclient.Interface, backgroundSA, reportsSA string) *policyHandlers {
	return &policyHandlers{
		client:                       client,
		backgroundServiceAccountName: backgroundSA,
		reportsServiceAccountName:    reportsSA,
	}
}

func (h *policyHandlers) Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	policy, oldPolicy, err := admissionutils.GetPolicies(request.AdmissionRequest)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.Response(request.UID, err)
	}

	if vpol := policy.AsValidatingPolicy(); vpol != nil {
		warnings, err := vpolvalidation.Validate(vpol)
		if err != nil {
			logger.Error(err, "validating policy validation errors")
		}
		return admissionutils.Response(request.UID, err, warnings...)
	}

	if ivpol := policy.AsImageValidatingPolicy(); ivpol != nil {
		warnings, err := eval.Validate(ivpol, h.client.GetKubeClient().CoreV1().Secrets(""))
		if err != nil {
			logger.Error(err, "validating policy validation errors")
		}
		return admissionutils.Response(request.UID, err, warnings...)
	}

	if gpol := policy.AsGeneratingPolicy(); gpol != nil {
		warnings, err := gpolvalidation.Validate(gpol)
		if err != nil {
			logger.Error(err, "generating policy validation errors")
		}
		return admissionutils.Response(request.UID, err, warnings...)
	}

	if pol := policy.AsKyvernoPolicy(); pol != nil {
		var old kyvernov1.PolicyInterface
		if oldPolicy != nil {
			old = oldPolicy.AsKyvernoPolicy()
		}

		warnings, err := policyvalidate.Validate(policy.AsKyvernoPolicy(), old, h.client, false, h.backgroundServiceAccountName, h.reportsServiceAccountName)
		if err != nil {
			logger.Error(err, "policy validation errors")
		}
		return admissionutils.Response(request.UID, err, warnings...)
	}

	return admissionutils.Response(request.UID, errors.New("failed to convert policy"))
}

func (h *policyHandlers) Mutate(_ context.Context, _ logr.Logger, request handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	return admissionutils.ResponseSuccess(request.UID)
}
