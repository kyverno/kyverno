package admission

import (
	"encoding/json"
	"fmt"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionv1 "k8s.io/api/admission/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UnmarshalPolicy(kind string, raw []byte) (kyvernov2beta1.PolicyInterface, error) {
	if kind == "ClusterPolicy" {
		var policy *kyvernov2beta1.ClusterPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return policy, nil
	} else if kind == "Policy" {
		var policy *kyvernov2beta1.Policy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return policy, nil
	}
	return nil, fmt.Errorf("admission request does not contain a policy")
}
func UnmarshalV1Policy(kind string, raw []byte) (kyverno.Policy, kyverno.ClusterPolicy, error) {

	var p kyverno.Policy
	var cp kyverno.ClusterPolicy
	if kind == "ClusterPolicy" {
		if err := json.Unmarshal(raw, &cp); err != nil {
			return p, cp, err
		}
		return p, cp, nil
	} else if kind == "Policy" {

		if err := json.Unmarshal(raw, &p); err != nil {
			return p, cp, err
		}
		return p, cp, nil
	}
	return p, cp, fmt.Errorf("admission request does not contain a policy")
}
func GetPolicy(request *admissionv1.AdmissionRequest) (kyvernov2beta1.PolicyInterface, error) {
	return UnmarshalPolicy(request.Kind.Kind, request.Object.Raw)
}
func conversionPolicy(request *admissionv1.AdmissionRequest) ([]byte, error) {

	_, cpol, err := UnmarshalV1Policy(request.Kind.Kind, request.Object.Raw)
	if err != nil {
		return nil, err
	}
	var policyCopy = cpol
	var matchResources kyverno.MatchResources
	var preconditions kyvernov2beta1.AnyAllConditions
	var deny kyvernov2beta1.AnyAllConditions

	for i, rule := range cpol.GetSpec().Rules {
		if rule.MatchResources.Any == nil && rule.MatchResources.All == nil {
			matchResources = kyverno.MatchResources{
				Any: []kyverno.ResourceFilter{
					{
						ResourceDescription: *rule.MatchResources.ResourceDescription.DeepCopy(),
					},
				},
			}
			policyCopy.Spec.Rules[i].MatchResources = *matchResources.DeepCopy()
		}

		if rule.RawAnyAllConditions != nil {
			kyvernoAnyAllConditions, _ := utils.ApiextensionsJsonToKyvernoConditions(rule.RawAnyAllConditions)
			switch typedAnyAllConditions := kyvernoAnyAllConditions.(type) {
			case []kyvernov2beta1.Condition:
				preconditions = kyvernov2beta1.AnyAllConditions{
					AnyConditions: typedAnyAllConditions,
				}
			}
			test1, _ := json.Marshal(preconditions)
			policyCopy.Spec.Rules[i].RawAnyAllConditions = &apiextv1.JSON{Raw: test1}
		}

		if rule.Validation.Deny != nil {
			if target := rule.Validation.Deny.GetAnyAllConditions(); target != nil {
				kyvernoConditions, err := utils.ApiextensionsJsonToKyvernoConditions(target)
				if err != nil {
					return nil, err
				}
				switch typedConditions := kyvernoConditions.(type) {
				case []kyvernov2beta1.Condition:
					deny = kyvernov2beta1.AnyAllConditions{
						AnyConditions: typedConditions,
					}

				}
			}
			test2, _ := json.Marshal(deny)
			policyCopy.Spec.Rules[i].Validation.Deny.RawAnyAllConditions = &apiextv1.JSON{Raw: test2}
		}

	}

	policyBytes, err := json.Marshal(policyCopy)
	if err != nil {
		return nil, err
	}

	return policyBytes, nil
}

func GetPolicies(request *admissionv1.AdmissionRequest) (kyvernov2beta1.PolicyInterface, kyvernov2beta1.PolicyInterface, error) {

	obj := request.Object.Raw
	convertedPolicy, err := conversionPolicy(request)
	if err != nil {
		return nil, nil, err
	}

	if convertedPolicy != nil {
		obj = convertedPolicy
	}

	policy, err := UnmarshalPolicy(request.Kind.Kind, obj)
	if err != nil {
		return policy, nil, err
	}
	if request.Operation == admissionv1.Update {
		oldPolicy, err := UnmarshalPolicy(request.Kind.Kind, request.OldObject.Raw)
		return policy, oldPolicy, err
	}
	return policy, nil, nil
}

func Response(allowed bool) *admissionv1.AdmissionResponse {
	r := &admissionv1.AdmissionResponse{
		Allowed: allowed,
	}
	return r
}

func ResponseWithMessage(allowed bool, msg string) *admissionv1.AdmissionResponse {
	r := Response(allowed)
	r.Result = &metav1.Status{
		Message: msg,
	}
	return r
}

func ResponseWithMessageAndPatch(allowed bool, msg string, patch []byte) *admissionv1.AdmissionResponse {
	r := ResponseWithMessage(allowed, msg)
	r.Patch = patch
	return r
}

func ResponseStatus(allowed bool, status, msg string) *admissionv1.AdmissionResponse {
	r := Response(allowed)
	r.Result = &metav1.Status{
		Status:  status,
		Message: msg,
	}
	return r
}

func ResponseFailure(msg string) *admissionv1.AdmissionResponse {
	return ResponseStatus(false, metav1.StatusFailure, msg)
}

func ResponseSuccess() *admissionv1.AdmissionResponse {
	return Response(true)
}

func ResponseSuccessWithWarnings(warnings []string) *admissionv1.AdmissionResponse {
	r := Response(true)
	r.Warnings = warnings
	return r
}

func ResponseSuccessWithPatch(patch []byte) *admissionv1.AdmissionResponse {
	r := Response(true)
	if len(patch) > 0 {
		r.Patch = patch
	}
	return r
}

func ResponseSuccessWithPatchAndWarnings(patch []byte, warnings []string) *admissionv1.AdmissionResponse {
	r := Response(true)
	if len(patch) > 0 {
		r.Patch = patch
	}

	r.Warnings = warnings
	return r
}

func GetResourceName(request *admissionv1.AdmissionRequest) string {
	resourceName := request.Kind.Kind + "/" + request.Name
	if request.Namespace != "" {
		resourceName = request.Namespace + "/" + resourceName
	}
	return resourceName
}
