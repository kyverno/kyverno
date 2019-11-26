package webhooks

import (
	"encoding/json"
	"fmt"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	policyvalidate "github.com/nirmata/kyverno/pkg/engine/policy"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//HandlePolicyValidation performs the validation check on policy resource
func (ws *WebhookServer) handlePolicyValidation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	var policy *kyverno.ClusterPolicy
	admissionResp := &v1beta1.AdmissionResponse{
		Allowed: true,
	}

	//TODO: can this happen? wont this be picked by OpenAPI spec schema ?
	raw := request.Object.Raw
	if err := json.Unmarshal(raw, &policy); err != nil {
		glog.Errorf("Failed to unmarshal policy admission request, err %v\n", err)
		return &v1beta1.AdmissionResponse{Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("Failed to unmarshal policy admission request err %v", err),
			}}
	}
	if err := policyvalidate.Validate(*policy); err != nil {
		admissionResp = &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	if admissionResp.Allowed {
		// create mutating resource mutatingwebhookconfiguration if not present
		if err := ws.webhookRegistrationClient.CreateResourceMutatingWebhookConfigurationIfRequired(*policy); err != nil {
			glog.Error("failed to created resource mutating webhook configuration, policies wont be applied on the resource")
		}
	}
	return admissionResp
}

func failResponseWithMsg(msg string) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Message: msg,
		},
	}
}
