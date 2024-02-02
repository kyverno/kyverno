package admission

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

func UnmarshalPolicy(kind string, raw []byte) (kyvernov1.PolicyInterface, error) {
	if kind == "ClusterPolicy" {
		var policy *kyvernov1.ClusterPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return policy, nil
	} else if kind == "Policy" {
		var policy *kyvernov1.Policy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return policy, nil
	}
	return nil, fmt.Errorf("admission request does not contain a policy")
}

func GetPolicy(request admissionv1.AdmissionRequest) (kyvernov1.PolicyInterface, error) {
	return UnmarshalPolicy(request.Kind.Kind, request.Object.Raw)
}

func GetPolicies(request admissionv1.AdmissionRequest) (kyvernov1.PolicyInterface, kyvernov1.PolicyInterface, error) {
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
