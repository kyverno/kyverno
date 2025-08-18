package v1alpha1

import (
	"context"

	"github.com/kyverno/kyverno/pkg/toggle"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=mutatingpolicies,scope="Cluster",shortName=mpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditionStatus.ready`
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MutatingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MutatingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status MutatingPolicyStatus `json:"status,omitempty"`
}

type MutatingPolicyStatus struct {
	// +optional
	ConditionStatus ConditionStatus `json:"conditionStatus,omitempty"`

	// +optional
	Autogen MutatingPolicyAutogenStatus `json:"autogen,omitempty"`

	// Generated indicates whether a MutatingAdmissionPolicy is generated from the policy or not
	// +optional
	Generated bool `json:"generated"`
}

// MutatingPolicySpec is the specification of the desired behavior of the MutatingPolicy.
type MutatingPolicySpec struct {
	// MatchConstraints specifies what resources this policy is designed to evaluate.
	// The AdmissionPolicy cares about a request if it matches _all_ Constraints.
	// Required.
	MatchConstraints *admissionregistrationv1alpha1.MatchResources `json:"matchConstraints,omitempty"`

	// failurePolicy defines how to handle failures for the admission policy. Failures can
	// occur from CEL expression parse errors, type check errors, runtime errors and invalid
	// or mis-configured policy definitions or bindings.
	//
	// failurePolicy does not define how validations that evaluate to false are handled.
	//
	// When failurePolicy is set to Fail, the validationActions field define how failures are enforced.
	//
	// Allowed values are Ignore or Fail. Defaults to Fail.
	// +optional
	// +kubebuilder:validation:Enum=Ignore;Fail
	FailurePolicy *admissionregistrationv1alpha1.FailurePolicyType `json:"failurePolicy,omitempty"`

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
	MatchConditions []admissionregistrationv1alpha1.MatchCondition `json:"matchConditions,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

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
	Variables []admissionregistrationv1alpha1.Variable `json:"variables,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// AutogenConfiguration defines the configuration for the generation controller.
	// +optional
	AutogenConfiguration *MutatingPolicyAutogenConfiguration `json:"autogen,omitempty"`

	// TargetMatchConstraints specifies what target mutation resources this policy is designed to evaluate.
	// +optional
	TargetMatchConstraints *admissionregistrationv1alpha1.MatchResources `json:"targetMatchConstraints,omitempty"`

	// mutations contain operations to perform on matching objects.
	// mutations may not be empty; a minimum of one mutation is required.
	// mutations are evaluated in order, and are reinvoked according to
	// the reinvocationPolicy.
	// The mutations of a policy are invoked for each binding of this policy
	// and reinvocation of mutations occurs on a per binding basis.
	//
	// +listType=atomic
	// +optional
	Mutations []admissionregistrationv1alpha1.Mutation `json:"mutations,omitempty" protobuf:"bytes,4,rep,name=mutations"`

	// WebhookConfiguration defines the configuration for the webhook.
	// +optional
	WebhookConfiguration *WebhookConfiguration `json:"webhookConfiguration,omitempty"`

	// EvaluationConfiguration defines the configuration for mutating policy evaluation.
	// +optional
	EvaluationConfiguration *MutatingPolicyEvaluationConfiguration `json:"evaluation,omitempty"`

	// reinvocationPolicy indicates whether mutations may be called multiple times per MutatingAdmissionPolicyBinding
	// as part of a single admission evaluation.
	// Allowed values are "Never" and "IfNeeded".
	//
	// Never: These mutations will not be called more than once per binding in a single admission evaluation.
	//
	// IfNeeded: These mutations may be invoked more than once per binding for a single admission request and there is no guarantee of
	// order with respect to other admission plugins, admission webhooks, bindings of this policy and admission policies.  Mutations are only
	// reinvoked when mutations change the object after this mutation is invoked.
	// Required.
	ReinvocationPolicy admissionregistrationv1alpha1.ReinvocationPolicyType `json:"reinvocationPolicy,omitempty" protobuf:"bytes,7,opt,name=reinvocationPolicy,casttype=ReinvocationPolicyType"`
}

func (s *MutatingPolicy) GetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.MatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}

	return s.Spec.GetMatchConstraints()
}

func (s *MutatingPolicy) GetTargetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.TargetMatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}

	return s.Spec.GetTargetMatchConstraints()
}

func (s *MutatingPolicy) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	return s.Spec.GetMatchConditions()
}

func (s *MutatingPolicySpec) GetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.MatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}

	in := s.MatchConstraints
	var out admissionregistrationv1.MatchResources
	out.NamespaceSelector = in.NamespaceSelector
	out.ObjectSelector = in.ObjectSelector
	for _, ex := range in.ExcludeResourceRules {
		out.ExcludeResourceRules = append(out.ExcludeResourceRules, admissionregistrationv1.NamedRuleWithOperations{
			ResourceNames:      ex.ResourceNames,
			RuleWithOperations: ex.RuleWithOperations,
		})
	}
	for _, ex := range in.ResourceRules {
		out.ResourceRules = append(out.ResourceRules, admissionregistrationv1.NamedRuleWithOperations{
			ResourceNames:      ex.ResourceNames,
			RuleWithOperations: ex.RuleWithOperations,
		})
	}
	if in.MatchPolicy != nil {
		mp := admissionregistrationv1.MatchPolicyType(*in.MatchPolicy)
		out.MatchPolicy = &mp
	}
	return out
}

func (s *MutatingPolicySpec) GetTargetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.TargetMatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}

	in := s.TargetMatchConstraints
	var out admissionregistrationv1.MatchResources
	out.NamespaceSelector = in.NamespaceSelector
	out.ObjectSelector = in.ObjectSelector
	for _, ex := range in.ExcludeResourceRules {
		out.ExcludeResourceRules = append(out.ExcludeResourceRules, admissionregistrationv1.NamedRuleWithOperations{
			ResourceNames:      ex.ResourceNames,
			RuleWithOperations: ex.RuleWithOperations,
		})
	}
	for _, ex := range in.ResourceRules {
		out.ResourceRules = append(out.ResourceRules, admissionregistrationv1.NamedRuleWithOperations{
			ResourceNames:      ex.ResourceNames,
			RuleWithOperations: ex.RuleWithOperations,
		})
	}
	if in.MatchPolicy != nil {
		mp := admissionregistrationv1.MatchPolicyType(*in.MatchPolicy)
		out.MatchPolicy = &mp
	}
	return out
}

func (s *MutatingPolicySpec) SetMatchConstraints(in admissionregistrationv1.MatchResources) {
	out := &admissionregistrationv1alpha1.MatchResources{}
	out.NamespaceSelector = in.NamespaceSelector
	out.ObjectSelector = in.ObjectSelector
	for _, ex := range in.ExcludeResourceRules {
		out.ExcludeResourceRules = append(out.ExcludeResourceRules, admissionregistrationv1alpha1.NamedRuleWithOperations{
			ResourceNames:      ex.ResourceNames,
			RuleWithOperations: ex.RuleWithOperations,
		})
	}
	for _, ex := range in.ResourceRules {
		out.ResourceRules = append(out.ResourceRules, admissionregistrationv1alpha1.NamedRuleWithOperations{
			ResourceNames:      ex.ResourceNames,
			RuleWithOperations: ex.RuleWithOperations,
		})
	}
	if in.MatchPolicy != nil {
		mp := admissionregistrationv1alpha1.MatchPolicyType(*in.MatchPolicy)
		out.MatchPolicy = &mp
	}
	s.MatchConstraints = out
}

func (s *MutatingPolicySpec) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	if s.MatchConditions == nil {
		return nil
	}
	in := s.MatchConditions
	out := make([]admissionregistrationv1.MatchCondition, len(in))
	for i := range in {
		out[i] = (admissionregistrationv1.MatchCondition)(in[i])
	}
	return out
}

// GenerateMutatingAdmissionPolicyEnabled checks if mutating admission policy generation is enabled
func (s MutatingPolicySpec) GenerateMutatingAdmissionPolicyEnabled() bool {
	const defaultValue = false
	if s.AutogenConfiguration == nil {
		return defaultValue
	}
	if s.AutogenConfiguration.MutatingAdmissionPolicy == nil {
		return defaultValue
	}
	if s.AutogenConfiguration.MutatingAdmissionPolicy.Enabled == nil {
		return defaultValue
	}
	return *s.AutogenConfiguration.MutatingAdmissionPolicy.Enabled
}

// GetReinvocationPolicy returns the reinvocation policy of the MutatingPolicy
func (s *MutatingPolicySpec) GetReinvocationPolicy() admissionregistrationv1alpha1.ReinvocationPolicyType {
	const defaultValue = admissionregistrationv1alpha1.NeverReinvocationPolicy
	if s.ReinvocationPolicy == "" {
		return defaultValue
	}
	return s.ReinvocationPolicy
}

func (s *MutatingPolicy) GetFailurePolicy() admissionregistrationv1.FailurePolicyType {
	if toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore() {
		return admissionregistrationv1.Ignore
	}
	if s.Spec.FailurePolicy == nil {
		return admissionregistrationv1.Fail
	}
	return admissionregistrationv1.FailurePolicyType(*s.Spec.FailurePolicy)
}

func (s *MutatingPolicy) GetWebhookConfiguration() *WebhookConfiguration {
	return s.Spec.WebhookConfiguration
}

func (s *MutatingPolicy) GetVariables() []admissionregistrationv1.Variable {
	in := s.Spec.Variables
	out := make([]admissionregistrationv1.Variable, len(in))
	for i := range in {
		out[i] = (admissionregistrationv1.Variable)(in[i])
	}
	return out
}

func (s MutatingPolicy) BackgroundEnabled() bool {
	return s.Spec.BackgroundEnabled()
}

func (s MutatingPolicySpec) AdmissionEnabled() bool {
	if s.EvaluationConfiguration == nil || s.EvaluationConfiguration.Admission == nil || s.EvaluationConfiguration.Admission.Enabled == nil {
		return true
	}
	return *s.EvaluationConfiguration.Admission.Enabled
}

func (s MutatingPolicySpec) BackgroundEnabled() bool {
	return true
}

func (s MutatingPolicySpec) MutateExistingEnabled() bool {
	if s.EvaluationConfiguration == nil ||
		s.EvaluationConfiguration.MutateExistingConfiguration == nil ||
		s.EvaluationConfiguration.MutateExistingConfiguration.Enabled == nil {
		return false
	}
	return *s.EvaluationConfiguration.MutateExistingConfiguration.Enabled
}

func (s *MutatingPolicy) GetStatus() *MutatingPolicyStatus {
	return &s.Status
}

func (s *MutatingPolicy) GetKind() string {
	return "MutatingPolicy"
}

func (s *MutatingPolicy) GetSpec() *MutatingPolicySpec {
	return &s.Spec
}

func (status *MutatingPolicyStatus) GetConditionStatus() *ConditionStatus {
	return &status.ConditionStatus
}

type MutatingPolicyEvaluationConfiguration struct {
	// Admission controls policy evaluation during admission.
	// +optional
	Admission *AdmissionConfiguration `json:"admission,omitempty"`

	// MutateExisting controls whether existing resources are mutated.
	// +optional
	MutateExistingConfiguration *MutateExistingConfiguration `json:"mutateExisting,omitempty"`
}

type MutatingPolicyAutogenConfiguration struct {
	// PodControllers specifies whether to generate a pod controllers rules.
	PodControllers *PodControllersGenerationConfiguration `json:"podControllers,omitempty"`
	// MutatingAdmissionPolicy specifies whether to generate a Kubernetes MutatingAdmissionPolicy.
	MutatingAdmissionPolicy *MAPGenerationConfiguration `json:"mutatingAdmissionPolicy,omitempty"`
}

type MAPGenerationConfiguration struct {
	// Enabled specifies whether to generate a Kubernetes MutatingAdmissionPolicy.
	// Optional. Defaults to "false" if not specified.
	Enabled *bool `json:"enabled,omitempty"`
}

type MutateExistingConfiguration struct {
	// Enabled enables mutation of existing resources. Default is false.
	// When spec.targetMatchConstraints is not defined, Kyverno mutates existing resources matched in spec.matchConstraints.
	// +optional
	// +kubebuilder:default=false
	Enabled *bool `json:"enabled,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MutatingPolicyList is a list of MutatingPolicy instances
type MutatingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []MutatingPolicy `json:"items"`
}
