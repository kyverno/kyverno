package webhooks

import (
	"encoding/json"
	"fmt"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/utils"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//HandlePolicyValidation performs the validation check on policy resource
func (ws *WebhookServer) HandlePolicyValidation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	var policy *kyverno.Policy
	admissionResp := &v1beta1.AdmissionResponse{
		Allowed: true,
	}
	// nothing to do on DELETE
	if request.Operation == v1beta1.Delete {
		return admissionResp
	}

	raw := request.Object.Raw
	if err := json.Unmarshal(raw, &policy); err != nil {
		glog.Errorf("Failed to unmarshal policy admission request, err %v\n", err)
		return &v1beta1.AdmissionResponse{Allowed: false}
	}
	// check for uniqueness of rule names while CREATE/DELET
	admissionResp = ws.validateUniqueRuleName(policy)

	if admissionResp.Allowed {
		ws.manageWebhookConfigurations(*policy, request.Operation)
	}

	return admissionResp
}

// Verify if the Rule names are unique within a policy
func (ws *WebhookServer) validateUniqueRuleName(policy *kyverno.Policy) *v1beta1.AdmissionResponse {
	// =======
	// func (ws *WebhookServer) validateUniqueRuleName(rawPolicy []byte) *v1beta1.AdmissionResponse {
	// 	var policy *kyverno.Policy
	// >>>>>>> policyViolation
	var ruleNames []string

	for _, rule := range policy.Spec.Rules {
		if utils.Contains(ruleNames, rule.Name) {
			msg := fmt.Sprintf(`The policy "%s" is invalid: duplicate rule name: "%s"`, policy.Name, rule.Name)
			glog.Errorln(msg)

			return &v1beta1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: msg,
				},
			}
		}
		ruleNames = append(ruleNames, rule.Name)
	}

	glog.V(3).Infof("Policy validation passed")
	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
}
