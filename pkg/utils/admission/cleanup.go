package admission

import (
	"encoding/json"
	"fmt"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	admissionv1 "k8s.io/api/admission/v1"
)

func UnmarshalCleanupPolicy(kind string, raw []byte) (kyvernov2alpha1.CleanupPolicyInterface, error) {
	if kind == "CleanupPolicy" {
		var policy *kyvernov2alpha1.CleanupPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return policy, nil
	} else if kind == "ClusterCleanupPolicy" {
		var policy *kyvernov2alpha1.ClusterCleanupPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return policy, nil
	}
	return nil, fmt.Errorf("admission request does not contain a cleanuppolicy")
}

func GetCleanupPolicies(request admissionv1.AdmissionRequest) (kyvernov2alpha1.CleanupPolicyInterface, kyvernov2alpha1.CleanupPolicyInterface, error) {
	var emptypolicy kyvernov2alpha1.CleanupPolicyInterface
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

// UnmarshalTTLLabel extracts the cleanup.kyverno.io/ttl label value from the raw admission request.
func GetTtlLabel(raw []byte) (string, error) {
	var resourceObj map[string]interface{}
	if err := json.Unmarshal(raw, &resourceObj); err != nil {
		return "", err
	}

	metadata, found := resourceObj["metadata"].(map[string]interface{})
	if !found {
		return "", fmt.Errorf("resource has no metadata field")
	}

	labels, found := metadata["labels"].(map[string]interface{})
	if !found {
		return "", fmt.Errorf("resource has no labels field")
	}

	ttlValue, found := labels[kyverno.LabelCleanupTtl].(string)
	if !found {
		return "", fmt.Errorf("resource has no %s label", kyverno.LabelCleanupTtl)
	}

	return ttlValue, nil
}
