package admission

import (
	"encoding/json"
	"fmt"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UnmarshalPolicy(kind string, raw []byte) (kyverno.PolicyInterface, error) {
	if kind == "ClusterPolicy" {
		var policy *kyverno.ClusterPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return policy, nil
	} else if kind == "Policy" {
		var policy *kyverno.Policy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return policy, nil
	}
	return nil, fmt.Errorf("admission request does not contain a policy")
}

func GetPolicy(request *admissionv1.AdmissionRequest) (kyverno.PolicyInterface, error) {
	return UnmarshalPolicy(request.Kind.Kind, request.Object.Raw)
}

func GetPolicies(request *admissionv1.AdmissionRequest) (kyverno.PolicyInterface, kyverno.PolicyInterface, error) {
	policy, err := UnmarshalPolicy(request.Kind.Kind, request.Object.Raw)
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

func ResponseFailure(allowed bool, msg string) *admissionv1.AdmissionResponse {
	return ResponseStatus(allowed, metav1.StatusFailure, msg)
}

func ResponseSuccess(allowed bool, msg string) *admissionv1.AdmissionResponse {
	return ResponseStatus(allowed, metav1.StatusSuccess, msg)
}

func ResponseSuccessWithPatch(allowed bool, msg string, patch []byte) *admissionv1.AdmissionResponse {
	r := ResponseSuccess(allowed, msg)
	if len(patch) > 0 {
		r.Patch = patch
	}
	return r
}

func GetResourceName(request *admissionv1.AdmissionRequest) string {
	resourceName := request.Kind.Kind + "/" + request.Name
	if request.Namespace != "" {
		resourceName = request.Namespace + "/" + resourceName
	}
	return resourceName
}
