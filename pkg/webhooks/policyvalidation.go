package webhooks

import (
	"encoding/json"
	"fmt"
	"time"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	policyvalidate "github.com/kyverno/kyverno/pkg/policy"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//HandlePolicyValidation performs the validation check on policy resource
func (ws *WebhookServer) policyValidation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	logger := ws.log.WithValues("action", "policy validation", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation, "gvk", request.Kind.String())
	var policy *kyverno.ClusterPolicy

	if err := json.Unmarshal(request.Object.Raw, &policy); err != nil {
		logger.Error(err, "failed to unmarshal policy admission request")
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to validate policy, check kyverno controller logs for details: %v", err),
			},
		}
	}

	if request.Operation == v1beta1.Update {
		admissionResponse := hasPolicyChanged(policy, request.OldObject.Raw, logger)
		if admissionResponse != nil {
			logger.V(4).Info("skip policy validation on status update")
			return admissionResponse
		}
	}

	startTime := time.Now()
	logger.V(3).Info("start policy change validation")
	defer logger.V(3).Info("finished policy change validation", "time", time.Since(startTime).String())

	if err := policyvalidate.Validate(policy, ws.client, false, ws.openAPIController); err != nil {
		logger.Error(err, "policy validation errors")
		return &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
}
