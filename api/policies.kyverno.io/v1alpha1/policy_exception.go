package v1alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:deprecatedversion

// PolicyException declares resources to be excluded from specified policies.
type PolicyException struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec declares policy exception behaviors.
	Spec PolicyExceptionSpec `json:"spec"`
}

// PolicyExceptionSpec stores policy exception spec
type PolicyExceptionSpec struct {
	// PolicyRefs identifies the policies to which the exception is applied.
	PolicyRefs []PolicyRef `json:"policyRefs"`

	// MatchConditions is a list of CEL expressions that must be met for a resource to be excluded.
	// +optional
	MatchConditions []admissionregistrationv1.MatchCondition `json:"matchConditions,omitempty"`

	// Images specifies container images to be excluded from policy evaluation.
	// These excluded images can be referenced in CEL expressions via `exceptions.allowedImages`.
	// +optional
	Images []string `json:"images,omitempty"`

	// AllowedValues specifies values that can be used in CEL expressions to bypass policy checks.
	// These values can be referenced in CEL expressions via `exceptions.allowedValues`.
	// +optional
	AllowedValues []string `json:"allowedValues,omitempty"`

	// ReportResult indicates whether the policy exception should be reported in the policy report
	// as a skip result or pass result. Defaults to "skip".
	// +optional
	// +kubebuilder:validation:Enum=skip;pass
	// +kubebuilder:default=skip
	ReportResult string `json:"reportResult,omitempty"`
}

type PolicyRef struct {
	// Name is the name of the policy
	Name string `json:"name"`

	// Kind is the kind of the policy
	Kind string `json:"kind"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyExceptionList is a list of Policy Exceptions
type PolicyExceptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []PolicyException `json:"items"`
}
