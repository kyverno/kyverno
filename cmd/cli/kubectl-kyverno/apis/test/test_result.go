package test

import (
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
)

type TestResults struct {
	// Policy mentions the name of the policy.
	Policy string `json:"policy"`
	// Rule mentions the name of the rule in the policy.
	// It's required in case policy is a kyverno policy.
	// +optional
	Rule string `json:"rule,omitempty"`
	// IsValidatingAdmissionPolicy indicates if the policy is a validating admission policy.
	// It's required in case policy is a validating admission policy.
	// +optional
	IsValidatingAdmissionPolicy bool `json:"isValidatingAdmissionPolicy,omitempty"`
	// Result mentions the result that the user is expecting.
	// Possible values are pass, fail and skip.
	Result policyreportv1alpha2.PolicyResult `json:"result"`
	// Status mentions the status that the user is expecting.
	// Possible values are pass, fail and skip.
	Status policyreportv1alpha2.PolicyResult `json:"status,omitempty"`
	// Resource mentions the name of the resource on which the policy is to be applied.
	Resource string `json:"resource,omitempty"`
	// Resources gives us the list of resources on which the policy is going to be applied.
	Resources []string `json:"resources"`
	// Kind mentions the kind of the resource on which the policy is to be applied.
	Kind string `json:"kind"`
	// Namespace mentions the namespace of the policy which has namespace scope.
	Namespace string `json:"namespace,omitempty"`
	// PatchedResource takes a resource configuration file in yaml format from
	// the user to compare it against the Kyverno mutated resource configuration.
	PatchedResource string `json:"patchedResource,omitempty"`
	// GeneratedResource takes a resource configuration file in yaml format from
	// the user to compare it against the Kyverno generated resource configuration.
	GeneratedResource string `json:"generatedResource,omitempty"`
	// CloneSourceResource takes the resource configuration file in yaml format
	// from the user which is meant to be cloned by the generate rule.
	CloneSourceResource string `json:"cloneSourceResource,omitempty"`
}
