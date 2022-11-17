package admission

import (
	"encoding/json"
	"fmt"

	kyvernov1alpha1 "github.com/kyverno/kyverno/api/kyverno/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
)

func UnmarshalCleanupPolicy(kind string, raw []byte) (kyvernov1alpha1.CleanupPolicyInterface, error) {
	if kind == "CleanupPolicy" {
		var policy *kyvernov1alpha1.CleanupPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return policy, nil
	} else if kind == "ClusterCleanupPolicy" {
		var policy *kyvernov1alpha1.ClusterCleanupPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return policy, nil
	}
	return nil, fmt.Errorf("admission request does not contain a cleanuppolicy")
}

func GetCleanupPolicies(request *admissionv1.AdmissionRequest) (kyvernov1alpha1.CleanupPolicyInterface, kyvernov1alpha1.CleanupPolicyInterface, error) {
	var emptypolicy kyvernov1alpha1.CleanupPolicyInterface
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

func FetchUniqueKinds(polspec kyvernov1alpha1.CleanupPolicySpec) []string {
	var kindlist []string

	kindlist = append(kindlist, polspec.MatchResources.Kinds...)

	for _, all := range polspec.MatchResources.Any {
		kindlist = append(kindlist, all.Kinds...)
	}

	if isMatchResourcesAllValid(polspec) {
		for _, all := range polspec.MatchResources.All {
			kindlist = append(kindlist, all.Kinds...)
		}
	}

	inResult := make(map[string]bool)
	var result []string
	for _, kind := range kindlist {
		if _, ok := inResult[kind]; !ok {
			inResult[kind] = true
			result = append(result, kind)
		}
	}
	return result
}

// check if all slice elements are same
func isMatchResourcesAllValid(polspec kyvernov1alpha1.CleanupPolicySpec) bool {
	var kindlist []string
	for _, all := range polspec.MatchResources.All {
		kindlist = append(kindlist, all.Kinds...)
	}

	if len(kindlist) == 0 {
		return false
	}

	for i := 1; i < len(kindlist); i++ {
		if kindlist[i] != kindlist[0] {
			return false
		}
	}
	return true
}
