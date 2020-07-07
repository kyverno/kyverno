package webhooks

import (
	policyvalidate "github.com/nirmata/kyverno/pkg/policy"

	v1beta1 "k8s.io/api/admission/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//HandlePolicyValidation performs the validation check on policy resource
func (ws *WebhookServer) policyValidation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	logger := ws.log.WithValues("action", "policyvalidation", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation)

	//TODO: can this happen? wont this be picked by OpenAPI spec schema ?
	if err := policyvalidate.Validate(request.Object.Raw, ws.client, false, ws.openAPIController); err != nil {
		logger.Error(err, "faield to validate policy")
		return &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	// if the policy contains mutating & validation rules and it config does not exist we create one
	// queue the request
	ws.resourceWebhookWatcher.RegisterResourceWebhook()
	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
}
