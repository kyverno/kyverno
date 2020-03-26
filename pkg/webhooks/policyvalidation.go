package webhooks

import (
	"encoding/json"
	"fmt"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	policyvalidate "github.com/nirmata/kyverno/pkg/policy"
	v1beta1 "k8s.io/api/admission/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//HandlePolicyValidation performs the validation check on policy resource
func (ws *WebhookServer) handlePolicyValidation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	logger := ws.log.WithValues("action", "policyvalidation", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation)
	var policy *kyverno.ClusterPolicy
	admissionResp := &v1beta1.AdmissionResponse{
		Allowed: true,
	}

	//TODO: can this happen? wont this be picked by OpenAPI spec schema ?
	raw := request.Object.Raw
	if err := json.Unmarshal(raw, &policy); err != nil {
		logger.Error(err, "failed to unmarshal incoming resource to policy type")
		return &v1beta1.AdmissionResponse{Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("Failed to unmarshal policy admission request err %v", err),
			}}
	}
	if err := policyvalidate.Validate(*policy, ws.client, false); err != nil {
		admissionResp = &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}
	if admissionResp.Allowed {
		// if the policy contains mutating & validation rules and it config does not exist we create one
		// queue the request
		ws.resourceWebhookWatcher.RegisterResourceWebhook()
	}
	return admissionResp
}
