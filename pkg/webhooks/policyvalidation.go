package webhooks

import (
	"encoding/json"
	"fmt"

	"github.com/golang/glog"
	policyv1 "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/utils"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//HandlePolicyValidation performs the validation check on policy resource
func (ws *WebhookServer) HandlePolicyValidation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	var policy *policyv1.Policy
	json.Unmarshal(request.Object.Raw, &policy)

	admissionResp := ws.validateUniqueRuleName(policy)

	if admissionResp.Allowed {
		ws.registerWebhookConfigurations(*policy)
	}
	return admissionResp
}

// Verify if the Rule names are unique within a policy
func (ws *WebhookServer) validateUniqueRuleName(policy *policyv1.Policy) *v1beta1.AdmissionResponse {
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
