package v1alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=generatingpolicies,scope="Cluster",shortName=gpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GeneratingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              GeneratingPolicySpec `json:"spec"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GeneratingPolicyList is a list of GeneratingPolicy instances
type GeneratingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []GeneratingPolicy `json:"items"`
}

// GeneratingPolicySpec is the specification of the desired behavior of the GeneratingPolicy.
type GeneratingPolicySpec struct {
	// MatchConstraints specifies what resources will trigger this policy.
	// The AdmissionPolicy cares about a request if it matches _all_ Constraints.
	// Required.
	MatchConstraints *admissionregistrationv1.MatchResources `json:"matchConstraints,omitempty"`

	// MatchConditions is a list of conditions that must be met for a request to be validated.
	// Match conditions filter requests that have already been matched by the rules,
	// namespaceSelector, and objectSelector. An empty list of matchConditions matches all requests.
	// There are a maximum of 64 match conditions allowed.
	//
	// If a parameter object is provided, it can be accessed via the `params` handle in the same
	// manner as validation expressions.
	//
	// The exact matching logic is (in order):
	//   1. If ANY matchCondition evaluates to FALSE, the policy is skipped.
	//   2. If ALL matchConditions evaluate to TRUE, the policy is evaluated.
	//   3. If any matchCondition evaluates to an error (but none are FALSE):
	//      - If failurePolicy=Fail, reject the request
	//      - If failurePolicy=Ignore, the policy is skipped
	//
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=name
	// +optional
	MatchConditions []admissionregistrationv1.MatchCondition `json:"matchConditions,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// Variables contain definitions of variables that can be used in composition of other expressions.
	// Each variable is defined as a named CEL expression.
	// The variables defined here will be available under `variables` in other expressions of the policy
	// except MatchConditions because MatchConditions are evaluated before the rest of the policy.
	//
	// The expression of a variable can refer to other variables defined earlier in the list but not those after.
	// Thus, Variables must be sorted by the order of first appearance and acyclic.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=name
	// +optional
	Variables []admissionregistrationv1.Variable `json:"variables,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// GenerateConfiguration defines the configuration for the policy evaluation.
	// +optional
	GenerateConfiguration *GenerateConfiguration `json:"evaluation,omitempty"`

	// Generation defines a set of CEL expressions that will be evaluated to generate resources.
	// Required.
	// +kubebuilder:validation:MinItems=1
	Generation []Generation `json:"generate"`
}

type GenerateConfiguration struct {
	// GenerateExisting controls whether to trigger the policy for existing resources
	// If is set to "true" the policy will be triggered and applied to existing matched resources.
	// +optional
	GenerateExisting *bool `json:"generateExisting,omitempty"`

	// Synchronize controls if generated resources should be kept in-sync with their source resource.
	// If Synchronize is set to "true" changes to generated resources will be overwritten with resource
	// data from Data or the resource specified in the Clone declaration.
	// Optional. Defaults to "false" if not specified.
	// +optional
	Synchronize *bool `json:"synchronize,omitempty"`

	// OrphanDownstreamOnPolicyDelete controls whether generated resources should be deleted when the policy that generated
	// them is deleted with synchronization enabled. This option is only applicable to generate rules of the data type.
	// See https://kyverno.io/docs/writing-policies/generate/#data-examples.
	// Defaults to "false" if not specified.
	// +optional
	OrphanDownstreamOnPolicyDelete *bool `json:"orphanDownstreamOnPolicyDelete,omitempty"`
}

// Generation defines the configuration for the generation of resources.
type Generation struct {
	// Expression is a CEL expression that takes a list of resources to be generated.
	Expression string `json:"expression,omitempty"`
}
