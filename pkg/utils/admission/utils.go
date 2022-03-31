package admission

import (
	"encoding/json"
	"fmt"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	v1beta1 "k8s.io/api/admission/v1beta1"
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

func GetPolicy(request *v1beta1.AdmissionRequest) (kyverno.PolicyInterface, error) {
	return UnmarshalPolicy(request.Kind.Kind, request.Object.Raw)
}

func GetPolicies(request *v1beta1.AdmissionRequest) (kyverno.PolicyInterface, kyverno.PolicyInterface, error) {
	policy, err := UnmarshalPolicy(request.Kind.Kind, request.Object.Raw)
	if err != nil {
		return policy, nil, err
	}
	if request.Operation == v1beta1.Update {
		oldPolicy, err := UnmarshalPolicy(request.Kind.Kind, request.OldObject.Raw)
		return policy, oldPolicy, err
	}
	return policy, nil, nil
}

func Response(allowed bool) *v1beta1.AdmissionResponse {
	r := &v1beta1.AdmissionResponse{
		Allowed: allowed,
	}
	return r
}

func ResponseWithMessage(allowed bool, msg string) *v1beta1.AdmissionResponse {
	r := Response(allowed)
	r.Result = &metav1.Status{
		Message: msg,
	}
	return r
}

func ResponseWithMessageAndPatch(allowed bool, msg string, patch []byte) *v1beta1.AdmissionResponse {
	r := ResponseWithMessage(allowed, msg)
	r.Patch = patch
	return r
}
