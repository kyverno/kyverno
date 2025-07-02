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
	// Status contains policy runtime data.
	// +optional
	Status GeneratingPolicyStatus `json:"status,omitempty"`
}

func (s *GeneratingPolicy) GetKind() string {
	return "GeneratingPolicy"
}

func (s *GeneratingPolicy) GetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.MatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}
	return *s.Spec.MatchConstraints
}

func (s *GeneratingPolicy) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	return s.Spec.MatchConditions
}

func (s *GeneratingPolicy) GetFailurePolicy() admissionregistrationv1.FailurePolicyType {
	return admissionregistrationv1.Ignore
}

func (s *GeneratingPolicy) GetWebhookConfiguration() *WebhookConfiguration {
	return s.Spec.WebhookConfiguration
}

func (s *GeneratingPolicy) GetVariables() []admissionregistrationv1.Variable {
	return s.Spec.Variables
}

func (s *GeneratingPolicy) GetSpec() *GeneratingPolicySpec {
	return &s.Spec
}

func (s *GeneratingPolicy) GetStatus() *GeneratingPolicyStatus {
	return &s.Status
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

	// EvaluationConfiguration defines the configuration for the policy evaluation.
	// +optional
	EvaluationConfiguration *GeneratingPolicyEvaluationConfiguration `json:"evaluation,omitempty"`

	// WebhookConfiguration defines the configuration for the webhook.
	// +optional
	WebhookConfiguration *WebhookConfiguration `json:"webhookConfiguration,omitempty"`

	// Generation defines a set of CEL expressions that will be evaluated to generate resources.
	// Required.
	// +kubebuilder:validation:MinItems=1
	Generation []Generation `json:"generate"`
}

func (s GeneratingPolicySpec) OrphanDownstreamOnPolicyDeleteEnabled() bool {
	const defaultValue = false
	if s.EvaluationConfiguration == nil {
		return defaultValue
	}
	if s.EvaluationConfiguration.OrphanDownstreamOnPolicyDelete == nil {
		return defaultValue
	}
	if s.EvaluationConfiguration.OrphanDownstreamOnPolicyDelete.Enabled == nil {
		return defaultValue
	}
	return *s.EvaluationConfiguration.OrphanDownstreamOnPolicyDelete.Enabled
}

func (s GeneratingPolicySpec) GenerateExistingEnabled() bool {
	const defaultValue = false
	if s.EvaluationConfiguration == nil {
		return defaultValue
	}
	if s.EvaluationConfiguration.GenerateExistingConfiguration == nil {
		return defaultValue
	}
	if s.EvaluationConfiguration.GenerateExistingConfiguration.Enabled == nil {
		return defaultValue
	}
	return *s.EvaluationConfiguration.GenerateExistingConfiguration.Enabled
}

func (s GeneratingPolicySpec) SynchronizationEnabled() bool {
	const defaultValue = false
	if s.EvaluationConfiguration == nil {
		return defaultValue
	}
	if s.EvaluationConfiguration.SynchronizationConfiguration == nil {
		return defaultValue
	}
	if s.EvaluationConfiguration.SynchronizationConfiguration.Enabled == nil {
		return defaultValue
	}
	return *s.EvaluationConfiguration.SynchronizationConfiguration.Enabled
}

func (s GeneratingPolicySpec) AdmissionEnabled() bool {
	if s.EvaluationConfiguration == nil || s.EvaluationConfiguration.Admission == nil || s.EvaluationConfiguration.Admission.Enabled == nil {
		return true
	}
	return *s.EvaluationConfiguration.Admission.Enabled
}

type GeneratingPolicyEvaluationConfiguration struct {
	// Admission controls policy evaluation during admission.
	// +optional
	Admission *AdmissionConfiguration `json:"admission,omitempty"`

	// GenerateExisting defines the configuration for generating resources for existing triggeres.
	// +optional
	GenerateExistingConfiguration *GenerateExistingConfiguration `json:"generateExisting,omitempty"`

	// Synchronization defines the configuration for the synchronization of generated resources.
	// +optional
	SynchronizationConfiguration *SynchronizationConfiguration `json:"synchronize,omitempty"`

	// OrphanDownstreamOnPolicyDelete defines the configuration for orphaning downstream resources on policy delete.
	OrphanDownstreamOnPolicyDelete *OrphanDownstreamOnPolicyDeleteConfiguration `json:"orphanDownstreamOnPolicyDelete,omitempty"`
}

// GenerateExistingConfiguration defines the configuration for generating resources for existing triggers.
type GenerateExistingConfiguration struct {
	// Enabled controls whether to trigger the policy for existing resources
	// If is set to "true" the policy will be triggered and applied to existing matched resources.
	// Optional. Defaults to "false" if not specified.
	// +optional
	// +kubebuilder:default=false
	Enabled *bool `json:"enabled,omitempty"`
}

// SynchronizationConfiguration defines the configuration for the synchronization of generated resources.
type SynchronizationConfiguration struct {
	// Enabled controls if generated resources should be kept in-sync with their source resource.
	// If Synchronize is set to "true" changes to generated resources will be overwritten with resource
	// data from Data or the resource specified in the Clone declaration.
	// Optional. Defaults to "false" if not specified.
	// +optional
	// +kubebuilder:default=false
	Enabled *bool `json:"enabled,omitempty"`
}

// OrphanDownstreamOnPolicyDeleteConfiguration defines the configuration for orphaning downstream resources on policy delete.
type OrphanDownstreamOnPolicyDeleteConfiguration struct {
	// Enabled controls whether generated resources should be deleted when the policy that generated
	// them is deleted with synchronization enabled. This option is only applicable to generate rules of the data type.
	// Optional. Defaults to "false" if not specified.
	// +optional
	// +kubebuilder:default=false
	Enabled *bool `json:"enabled,omitempty"`
}

// Generation defines the configuration for the generation of resources.
type Generation struct {
	// Expression is a CEL expression that takes a list of resources to be generated.
	Expression string `json:"expression,omitempty"`
}

type GeneratingPolicyStatus struct {
	// +optional
	ConditionStatus ConditionStatus `json:"conditionStatus,omitempty"`
}
