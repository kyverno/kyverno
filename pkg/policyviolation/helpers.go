package policyviolation

import (
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
)

//BuildPolicyViolation returns an value of type PolicyViolation
func BuildPolicyViolation(policy string, resource kyverno.ResourceSpec, fRules []kyverno.ViolatedRule) kyverno.PolicyViolation {
	pv := kyverno.PolicyViolation{
		Spec: kyverno.PolicyViolationSpec{
			Policy:        policy,
			ResourceSpec:  resource,
			ViolatedRules: fRules,
		},
	}
	//TODO: check if this can be removed or use unstructured?
	// pv.Kind = "PolicyViolation"
	return pv
}
