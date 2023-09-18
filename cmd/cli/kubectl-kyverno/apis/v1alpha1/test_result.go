package v1alpha1

import (
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
)

// TestResultBase declares a test result base fields
type TestResultBase struct {
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

	// Kind mentions the kind of the resource on which the policy is to be applied.
	Kind string `json:"kind"`

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

// TestResultBase declares a test result deprecated fields
type TestResultDeprecated struct {
	// Status mentions the status that the user is expecting.
	// Possible values are pass, fail and skip.
	// This is DEPRECATED, use `Result` instead.
	Status policyreportv1alpha2.PolicyResult `json:"status,omitempty"`

	// Resource mentions the name of the resource on which the policy is to be applied.
	// This is DEPRECATED, use `Resources` instead.
	Resource string `json:"resource,omitempty"`

	// Namespace mentions the namespace of the policy which has namespace scope.
	// This is DEPRECATED, use a name in the form `<namespace>/<name>` for policies and/or resources instead.
	Namespace string `json:"namespace,omitempty"`
}

// TestResultBase declares a test result
type TestResult struct {
	TestResultBase       `json:",inline,omitempty"`
	TestResultDeprecated `json:",inline,omitempty"`

	// Resources gives us the list of resources on which the policy is going to be applied.
	Resources []string `json:"resources"`
}
