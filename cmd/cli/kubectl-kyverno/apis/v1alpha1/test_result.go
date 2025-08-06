package v1alpha1

import openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"

// TestResultBase declares a test result base fields
type TestResultBase struct {
	// Policy mentions the name of the policy.
	Policy string `json:"policy"`

	// Rule mentions the name of the rule in the policy.
	// It's required in case policy is a kyverno policy.
	// +optional
	Rule string `json:"rule,omitempty"`

	// IsValidatingAdmissionPolicy indicates if the policy is a validating admission policy.
	// It's required in case the policy is a validating admission policy.
	// +optional
	IsValidatingAdmissionPolicy bool `json:"isValidatingAdmissionPolicy,omitempty"`

	// IsMutatingAdmissionPolicy indicates if the policy is a mutating admission policy.
	// +optional
	IsMutatingAdmissionPolicy bool `json:"isMutatingAdmissionPolicy,omitempty"`

	// IsValidatingPolicy indicates if the policy is a validating policy.
	// It's required in case the policy is a validating policy.
	// +optional
	IsValidatingPolicy bool `json:"isValidatingPolicy,omitempty"`

	// IsDeletingPolicy indicates if the policy is a deleting policy.
	// It's required in case the policy is a deleting policy.
	// +optional
	IsDeletingPolicy bool `json:"isDeletingPolicy,omitempty"`

	// IsImageValidatingPolicy indicates if the policy is an image validating policy.
	// It's required in case the policy is an image validating policy.
	// +optional
	IsImageValidatingPolicy bool `json:"isImageValidatingPolicy,omitempty"`

	// IsGeneratingPolicy indicates if the policy is a generating policy.
	// It's required in case the policy is a generating policy.
	// +optional
	IsGeneratingPolicy bool `json:"isGeneratingPolicy,omitempty"`

	// IsMutatingPolicy indicates if the policy is a mutating policy.
	// It's required in case the policy is a mutating policy.
	// +optional
	IsMutatingPolicy bool `json:"isMutatingPolicy,omitempty"`

	// Result mentions the result that the user is expecting.
	// Possible values are pass, fail and skip.
	Result openreportsv1alpha1.Result `json:"result"`

	// Kind mentions the kind of the resource on which the policy is to be applied.
	Kind string `json:"kind"`

	// PatchedResource takes a resource configuration file in yaml format from
	// the user to compare it against the Kyverno mutated resource configuration.
	// Multiple resources can be passed in the same file
	PatchedResources string `json:"patchedResources,omitempty"`

	// GeneratedResource takes a resource configuration file in yaml format from
	// the user to compare it against the Kyverno generated resource configuration.
	GeneratedResource string `json:"generatedResource,omitempty"`

	// CloneSourceResource takes the resource configuration file in yaml format
	// from the user which is meant to be cloned by the generate rule.
	CloneSourceResource string `json:"cloneSourceResource,omitempty"`
}

// TestResultData declares a test result data
type TestResultData struct {
	// Resources gives us the list of resources on which the policy is going to be applied.
	Resources []string `json:"resources,omitempty"`

	// Resources gives us the list of resources on which the policy is going to be applied.
	ResourceSpecs []TestResourceSpec `json:"resourceSpecs,omitempty"`
}

// TestResult declares a test result
type TestResult struct {
	TestResultBase `json:",inline"`
	TestResultData `json:",inline"`
}
