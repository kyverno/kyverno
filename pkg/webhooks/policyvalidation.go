package webhooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/utils"
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

	if err := ws.validatePolicy(policy); err != nil {
		admissionResp = &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	// helper function to evaluate if policy has validtion or mutation rules defined
	hasMutateOrValidate := func() bool {
		for _, rule := range policy.Spec.Rules {
			if rule.HasMutate() || rule.HasValidate() {
				return true
			}
		}
		return false
	}

	if admissionResp.Allowed {
		if hasMutateOrValidate() {
			// create mutating resource mutatingwebhookconfiguration if not present
			if err := ws.webhookRegistrationClient.CreateResourceMutatingWebhookConfiguration(); err != nil {
				glog.Error("failed to created resource mutating webhook configuration, policies wont be applied on the resource")
			}
		}
	}
	return admissionResp
}

func (ws *WebhookServer) validatePolicy(policy *kyverno.ClusterPolicy) error {
	// validate only one type of rule defined per rule
	if err := validateRuleType(policy); err != nil {
		return err
	}

	// validate resource description block

	// validate ^() can only be used on array

	// check for uniqueness of rule names while CREATE/DELET
	if err := validateUniqueRuleName(policy); err != nil {
		return err
	}

	if err := validateOverlayPattern(policy); err != nil {
		return err
	}

	return nil
}

// validateRuleType checks only one type of rule is defined per rule
func validateRuleType(policy *kyverno.ClusterPolicy) error {
	for _, rule := range policy.Spec.Rules {
		mutate := rule.HasMutate()
		validate := rule.HasValidate()
		generate := rule.HasGenerate()

		if !mutate && !validate && !generate {
			return fmt.Errorf("No rule defined in '%s'", rule.Name)
		}

		if (mutate && !validate && !generate) ||
			(!mutate && validate && !generate) ||
			(!mutate && !validate && generate) {
			return nil
		}

		return fmt.Errorf("Multiple types of rule defined in rule '%s', only one type of rule is allowed per rule", rule.Name)
	}

	return nil
}

// Verify if the Rule names are unique within a policy
func validateUniqueRuleName(policy *kyverno.ClusterPolicy) error {
	var ruleNames []string

	for _, rule := range policy.Spec.Rules {
		if utils.ContainsString(ruleNames, rule.Name) {
			msg := fmt.Sprintf(`The policy "%s" is invalid: duplicate rule name: "%s"`, policy.Name, rule.Name)
			glog.Errorln(msg)
			return errors.New(msg)
		}
		ruleNames = append(ruleNames, rule.Name)
	}

	glog.V(4).Infof("Policy validation passed")
	return nil
}

// validateOverlayPattern checks one of pattern/anyPattern must exist
func validateOverlayPattern(policy *kyverno.ClusterPolicy) error {
	for _, rule := range policy.Spec.Rules {
		if reflect.DeepEqual(rule.Validation, kyverno.Validation{}) {
			continue
		}

		if rule.Validation.Pattern == nil && len(rule.Validation.AnyPattern) == 0 {
			return errors.New(fmt.Sprintf("Invalid policy, neither pattern nor anyPattern found in validate rule %s", rule.Name))
		}

		if rule.Validation.Pattern != nil && len(rule.Validation.AnyPattern) != 0 {
			return errors.New(fmt.Sprintf("Invalid policy, either pattern or anyPattern is allowed in validate rule %s", rule.Name))
		}
	}

	return nil
}

func failResponseWithMsg(msg string) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Message: msg,
		},
	}
}
