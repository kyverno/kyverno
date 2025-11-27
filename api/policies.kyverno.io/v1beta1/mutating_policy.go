package v1beta1

import (
	"context"

	"github.com/kyverno/kyverno/pkg/toggle"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=mutatingpolicies,scope="Cluster",shortName=mpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditionStatus.ready`
// +kubebuilder:selectablefield:JSONPath=`.spec.evaluation.mode`
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MutatingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MutatingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status MutatingPolicyStatus `json:"status,omitempty"`
}

// BackgroundEnabled checks if background is set to true
func (s MutatingPolicy) BackgroundEnabled() bool {
	return s.Spec.BackgroundEnabled()
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=nmpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditionStatus.ready`
// +kubebuilder:selectablefield:JSONPath=`.spec.evaluation.mode`
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion

type NamespacedMutatingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MutatingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status MutatingPolicyStatus `json:"status,omitempty"`
}

// BackgroundEnabled checks if background is set to true
func (s NamespacedMutatingPolicy) BackgroundEnabled() bool {
	return s.Spec.BackgroundEnabled()
}

func (s *NamespacedMutatingPolicy) GetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.MatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}
	return *s.Spec.MatchConstraints
}

func (s *NamespacedMutatingPolicy) GetTargetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.TargetMatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}
	return *s.Spec.TargetMatchConstraints
}

func (s *NamespacedMutatingPolicy) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	return s.Spec.MatchConditions
}

func (s *NamespacedMutatingPolicy) GetFailurePolicy() admissionregistrationv1.FailurePolicyType {
	if toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore() {
		return admissionregistrationv1.Ignore
	}
	if s.Spec.FailurePolicy == nil {
		return admissionregistrationv1.Fail
	}
	return *s.Spec.FailurePolicy
}

func (s *NamespacedMutatingPolicy) GetWebhookConfiguration() *WebhookConfiguration {
	return s.Spec.WebhookConfiguration
}

func (s *NamespacedMutatingPolicy) GetTimeoutSeconds() *int32 {
	if s.Spec.WebhookConfiguration == nil {
		return nil
	}
	return s.Spec.WebhookConfiguration.TimeoutSeconds
}

func (s *NamespacedMutatingPolicy) GetVariables() []admissionregistrationv1.Variable {
	return s.Spec.Variables
}

func (s *NamespacedMutatingPolicy) GetSpec() *MutatingPolicySpec {
	return &s.Spec
}

func (s *NamespacedMutatingPolicy) GetStatus() *MutatingPolicyStatus {
	return &s.Status
}

func (s *NamespacedMutatingPolicy) GetKind() string {
	return "NamespacedMutatingPolicy"
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

func (s *MutatingPolicy) GetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.MatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}
	return *s.Spec.MatchConstraints
}

func (s *MutatingPolicySpec) SetMatchConstraints(in admissionregistrationv1.MatchResources) {
	out := &admissionregistrationv1.MatchResources{}
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
		out.MatchPolicy = in.MatchPolicy
	}
	s.MatchConstraints = out
}

func (s *MutatingPolicy) GetTargetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.TargetMatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}
	return *s.Spec.TargetMatchConstraints
}

func (s *MutatingPolicy) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	return s.Spec.MatchConditions
}

func (s *MutatingPolicy) GetFailurePolicy() admissionregistrationv1.FailurePolicyType {
	if toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore() {
		return admissionregistrationv1.Ignore
	}
	if s.Spec.FailurePolicy == nil {
		return admissionregistrationv1.Fail
	}
	return *s.Spec.FailurePolicy
}

func (s *MutatingPolicy) GetWebhookConfiguration() *WebhookConfiguration {
	return s.Spec.WebhookConfiguration
}

func (s *MutatingPolicy) GetTimeoutSeconds() *int32 {
	if s.Spec.WebhookConfiguration == nil {
		return nil
	}
	return s.Spec.WebhookConfiguration.TimeoutSeconds
}

func (s *MutatingPolicy) GetVariables() []admissionregistrationv1.Variable {
	return s.Spec.Variables
}

func (s *MutatingPolicy) GetSpec() *MutatingPolicySpec {
	return &s.Spec
}

func (s *MutatingPolicy) GetStatus() *MutatingPolicyStatus {
	return &s.Status
}

func (s *MutatingPolicy) GetKind() string {
	return "MutatingPolicy"
}

func (status *MutatingPolicyStatus) GetConditionStatus() *ConditionStatus {
	return &status.ConditionStatus
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MutatingPolicyList is a list of MutatingPolicy instances
type MutatingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []MutatingPolicy `json:"items"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NamespacedMutatingPolicyList is a list of NamespacedMutatingPolicy instances
type NamespacedMutatingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []NamespacedMutatingPolicy `json:"items"`
}

// MutatingPolicyLike captures the common behaviour shared by mutating policies regardless of scope.
// +k8s:deepcopy-gen=false
type MutatingPolicyLike interface {
	metav1.Object
	runtime.Object
	GetSpec() *MutatingPolicySpec
	GetStatus() *MutatingPolicyStatus
	GetFailurePolicy() admissionregistrationv1.FailurePolicyType
	GetMatchConstraints() admissionregistrationv1.MatchResources
	GetTargetMatchConstraints() admissionregistrationv1.MatchResources
	GetMatchConditions() []admissionregistrationv1.MatchCondition
	GetVariables() []admissionregistrationv1.Variable
	GetWebhookConfiguration() *WebhookConfiguration
	BackgroundEnabled() bool
	GetKind() string
}

// MutatingPolicySpec is the specification of the desired behavior of the MutatingPolicy.
type MutatingPolicySpec struct {
	// MatchConstraints specifies what resources this policy is designed to evaluate.
	// The AdmissionPolicy cares about a request if it matches _all_ Constraints.
	// Required.
	MatchConstraints *admissionregistrationv1.MatchResources `json:"matchConstraints,omitempty"`

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
	FailurePolicy *admissionregistrationv1.FailurePolicyType `json:"failurePolicy,omitempty"`

	// MatchConditions is a list of conditions that must be met for a request to be validated.
	// Match conditions filter requests that have already been matched by the rules,
	// namespaceSelector, and objectSelector. An empty list of matchConditions matches all requests.
	// There are a maximum of 64 match conditions allowed.
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

	// AutogenConfiguration defines the configuration for the generation controller.
	// +optional
	AutogenConfiguration *MutatingPolicyAutogenConfiguration `json:"autogen,omitempty"`

	// TargetMatchConstraints specifies what target mutation resources this policy is designed to evaluate.
	// +optional
	TargetMatchConstraints *admissionregistrationv1.MatchResources `json:"targetMatchConstraints,omitempty"`

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
	ReinvocationPolicy admissionregistrationv1.ReinvocationPolicyType `json:"reinvocationPolicy,omitempty" protobuf:"bytes,7,opt,name=reinvocationPolicy,casttype=ReinvocationPolicyType"`
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

// AdmissionEnabled checks if admission is set to true
func (s MutatingPolicySpec) AdmissionEnabled() bool {
	const defaultValue = true
	if s.EvaluationConfiguration == nil || s.EvaluationConfiguration.Admission == nil || s.EvaluationConfiguration.Admission.Enabled == nil {
		return defaultValue
	}
	return *s.EvaluationConfiguration.Admission.Enabled
}

// BackgroundEnabled checks if background is set to true
func (s MutatingPolicySpec) BackgroundEnabled() bool {
	const defaultValue = true
	if s.EvaluationConfiguration == nil || s.EvaluationConfiguration.Background == nil || s.EvaluationConfiguration.Background.Enabled == nil {
		return defaultValue
	}
	return *s.EvaluationConfiguration.Background.Enabled
}

// EvaluationMode returns the evaluation mode of the policy.
func (s MutatingPolicySpec) EvaluationMode() EvaluationMode {
	const defaultValue = EvaluationModeKubernetes
	if s.EvaluationConfiguration == nil || s.EvaluationConfiguration.Mode == "" {
		return defaultValue
	}
	return s.EvaluationConfiguration.Mode
}

// GetReinvocationPolicy returns the reinvocation policy of the MutatingPolicy
func (s *MutatingPolicySpec) GetReinvocationPolicy() admissionregistrationv1.ReinvocationPolicyType {
	const defaultValue = admissionregistrationv1.NeverReinvocationPolicy
	if s.ReinvocationPolicy == "" {
		return defaultValue
	}
	return s.ReinvocationPolicy
}

// MutateExistingEnabled checks if mutate existing is set to true
func (s MutatingPolicySpec) MutateExistingEnabled() bool {
	if s.EvaluationConfiguration == nil ||
		s.EvaluationConfiguration.MutateExistingConfiguration == nil ||
		s.EvaluationConfiguration.MutateExistingConfiguration.Enabled == nil {
		return false
	}
	return *s.EvaluationConfiguration.MutateExistingConfiguration.Enabled
}

type MutatingPolicyEvaluationConfiguration struct {
	// Mode is the mode of policy evaluation.
	// Allowed values are "Kubernetes" or "JSON".
	// Optional. Default value is "Kubernetes".
	// +optional
	Mode EvaluationMode `json:"mode,omitempty"`

	// Admission controls policy evaluation during admission.
	// +optional
	Admission *AdmissionConfiguration `json:"admission,omitempty"`

	// Background controls policy evaluation during background scan.
	// +optional
	Background *BackgroundConfiguration `json:"background,omitempty"`

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

// MutationTarget specifies the target of the mutation.
type MutationTarget struct {
	// Group specifies the API group of the target resource.
	// +optional
	Group string `json:"group,omitempty"`

	// Version specifies the API version of the target resource.
	// +optional
	Version string `json:"version,omitempty"`

	// Resource specifies the resource name of the target resource.
	// +optional
	Resource string `json:"resource,omitempty"`

	// Kind specifies the kind of the target resource.
	// +optional
	Kind string `json:"kind,omitempty"`
}
