package admission

import (
	"fmt"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

func UnmarshalCleanupPolicy(kind string, raw []byte) (kyvernov2.CleanupPolicyInterface, error) {
	if kind == "CleanupPolicy" {
		var policy *kyvernov2.CleanupPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return policy, nil
	} else if kind == "ClusterCleanupPolicy" {
		var policy *kyvernov2.ClusterCleanupPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return policy, nil
	}
	return nil, fmt.Errorf("admission request does not contain a cleanuppolicy")
}

func GetCleanupPolicies(request admissionv1.AdmissionRequest) (kyvernov2.CleanupPolicyInterface, kyvernov2.CleanupPolicyInterface, error) {
	var emptypolicy kyvernov2.CleanupPolicyInterface
	policy, err := UnmarshalCleanupPolicy(request.Kind.Kind, request.Object.Raw)
	if err != nil {
		return policy, emptypolicy, err
	}
	if request.Operation == admissionv1.Update {
		oldPolicy, err := UnmarshalCleanupPolicy(request.Kind.Kind, request.OldObject.Raw)
		return policy, oldPolicy, err
	}
	return policy, emptypolicy, nil
}
