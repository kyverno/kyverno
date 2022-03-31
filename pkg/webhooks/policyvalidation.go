package webhooks

import (
	"fmt"
	"time"

	policyvalidate "github.com/kyverno/kyverno/pkg/policy"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	v1beta1 "k8s.io/api/admission/v1beta1"
)

//policyValidation performs the validation check on policy resource
func (ws *WebhookServer) policyValidation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	logger := ws.log.WithValues("action", "policy validation", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation, "gvk", request.Kind.String())
	policy, oldPolicy, err := admissionutils.GetPolicies(request)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.ResponseWithMessage(true, fmt.Sprintf("failed to validate policy, check kyverno controller logs for details: %v", err))
	}
	if oldPolicy != nil && isStatusUpdate(oldPolicy, policy) {
		logger.V(4).Info("skip policy validation on status update")
		return admissionutils.Response(true)
	}
	startTime := time.Now()
	logger.V(3).Info("start policy change validation")
	defer logger.V(3).Info("finished policy change validation", "time", time.Since(startTime).String())
	response, err := policyvalidate.Validate(policy, ws.client, false, ws.openAPIController)
	if err != nil {
		logger.Error(err, "policy validation errors")
		return admissionutils.ResponseWithMessage(true, err.Error())
	}
	if response != nil && len(response.Warnings) != 0 {
		return response
	}
	return admissionutils.Response(true)
}
