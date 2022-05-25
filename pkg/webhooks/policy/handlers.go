package policy

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/openapi"
	policyvalidate "github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/policymutation"
	"github.com/kyverno/kyverno/pkg/toggle"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/kyverno/pkg/webhooks"
	admissionv1 "k8s.io/api/admission/v1"
)

type handlers struct {
	client            dclient.Interface
	openAPIController *openapi.Controller
}

func NewHandlers(client dclient.Interface, openAPIController *openapi.Controller) webhooks.Handlers {
	return &handlers{
		client:            client,
		openAPIController: openAPIController,
	}
}

func (h *handlers) Validate(logger logr.Logger, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	if request.SubResource != "" {
		logger.V(4).Info("skip policy validation on status update")
		return admissionutils.Response(true)
	}
	policy, _, err := admissionutils.GetPolicies(request)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.ResponseWithMessage(true, fmt.Sprintf("failed to validate policy, check kyverno controller logs for details: %v", err))
	}
	startTime := time.Now()
	logger.V(3).Info("start policy change validation")
	defer logger.V(3).Info("finished policy change validation", "time", time.Since(startTime).String())
	response, err := policyvalidate.Validate(policy, h.client, false, h.openAPIController)
	if err != nil {
		logger.Error(err, "policy validation errors")
		return admissionutils.ResponseWithMessage(false, err.Error())
	}
	if response != nil && len(response.Warnings) != 0 {
		return response
	}
	return admissionutils.Response(true)
}

func (h *handlers) Mutate(logger logr.Logger, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	if toggle.AutogenInternals() {
		return admissionutils.Response(true)
	}
	if request.SubResource != "" {
		logger.V(4).Info("skip policy validation on status update")
		return admissionutils.Response(true)
	}
	policy, _, err := admissionutils.GetPolicies(request)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.ResponseWithMessage(true, fmt.Sprintf("failed to default value, check kyverno controller logs for details: %v", err))
	}
	startTime := time.Now()
	logger.V(3).Info("start policy change mutation")
	defer logger.V(3).Info("finished policy change mutation", "time", time.Since(startTime).String())
	if patches, updateMsgs := policymutation.GenerateJSONPatchesForDefaults(policy, logger); len(patches) != 0 {
		return admissionutils.ResponseWithMessageAndPatch(true, strings.Join(updateMsgs, "'"), patches)
	}
	return admissionutils.Response(true)
}
