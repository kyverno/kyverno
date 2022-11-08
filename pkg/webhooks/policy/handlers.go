package policy

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/openapi"
	policyvalidate "github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/policymutation"
	"github.com/kyverno/kyverno/pkg/toggle"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/kyverno/pkg/webhooks"
	admissionv1 "k8s.io/api/admission/v1"
)

type handlers struct {
	client         dclient.Interface
	openApiManager openapi.Manager
}

func NewHandlers(client dclient.Interface, openApiManager openapi.Manager) webhooks.PolicyHandlers {
	return &handlers{
		client:         client,
		openApiManager: openApiManager,
	}
}

func (h *handlers) Validate(logger logr.Logger, request *admissionv1.AdmissionRequest, _ time.Time) *admissionv1.AdmissionResponse {
	if request.SubResource != "" {
		logger.V(4).Info("skip policy validation on status update")
		return admissionutils.ResponseSuccess()
	}
	policy, _, err := admissionutils.GetPolicies(request)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.Response(err)
	}
	warnings, err := policyvalidate.Validate(policy, h.client, false, h.openApiManager)
	if err != nil {
		logger.Error(err, "policy validation errors")
		return admissionutils.Response(err, warnings...)
	}
	return admissionutils.ResponseSuccess(warnings...)
}

func (h *handlers) Mutate(logger logr.Logger, request *admissionv1.AdmissionRequest, _ time.Time) *admissionv1.AdmissionResponse {
	if toggle.AutogenInternals.Enabled() {
		return admissionutils.ResponseSuccess()
	}
	if request.SubResource != "" {
		logger.V(4).Info("skip policy validation on status update")
		return admissionutils.ResponseSuccess()
	}
	policy, _, err := admissionutils.GetPolicies(request)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.Response(fmt.Errorf("failed to default value, check kyverno controller logs for details: %v", err))
	}
	if patches, updateMsgs := policymutation.GenerateJSONPatchesForDefaults(policy, logger); len(patches) != 0 {
		return admissionutils.MutationResponse(patches, updateMsgs...)
	}
	return admissionutils.ResponseSuccess()
}
