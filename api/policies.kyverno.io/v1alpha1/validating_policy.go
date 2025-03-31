package v1alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EvaluationMode string

const (
	EvaluationModeKubernetes EvaluationMode = "Kubernetes"
	EvaluationModeJSON       EvaluationMode = "JSON"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=validatingpolicies,scope="Cluster",shortName=vpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditionStatus.ready`
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ValidatingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ValidatingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status ValidatingPolicyStatus `json:"status,omitempty"`
}

type ValidatingPolicyStatus struct {
	// +optional
	ConditionStatus ConditionStatus `json:"conditionStatus,omitempty"`

	// +optional
	Autogen AutogenStatus `json:"autogen"`

	// Generated indicates whether a ValidatingAdmissionPolicy/MutatingAdmissionPolicy is generated from the policy or not
	// +optional
	Generated bool `json:"generated"`
}

// AutogenStatus contains autogen status information.
type AutogenStatus struct {
	// Rules is a list of Rule instances. It contains auto generated rules added for pod controllers
	Rules []AutogenRule `json:"rules,omitempty"`
}

type AutogenRule struct {
	MatchConstraints *admissionregistrationv1.MatchResources   `json:"matchConstraints,omitempty"`
	MatchConditions  []admissionregistrationv1.MatchCondition  `json:"matchConditions,omitempty"`
	Validations      []admissionregistrationv1.Validation      `json:"validations,omitempty"`
	AuditAnnotation  []admissionregistrationv1.AuditAnnotation `json:"auditAnnotations,omitempty"`
	Variables        []admissionregistrationv1.Variable        `json:"variables,omitempty"`
}

func (s *ValidatingPolicy) GetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.MatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}
	return *s.Spec.MatchConstraints
}

func (s *ValidatingPolicy) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	return s.Spec.MatchConditions
}

func (s *ValidatingPolicy) GetFailurePolicy() admissionregistrationv1.FailurePolicyType {
	if s.Spec.FailurePolicy == nil {
		return admissionregistrationv1.Fail
	}
	return *s.Spec.FailurePolicy
}

func (s *ValidatingPolicy) GetWebhookConfiguration() *WebhookConfiguration {
	return s.Spec.WebhookConfiguration
}

func (s *ValidatingPolicy) GetVariables() []admissionregistrationv1.Variable {
	return s.Spec.Variables
}

func (s *ValidatingPolicy) GetSpec() *ValidatingPolicySpec {
	return &s.Spec
}

func (s *ValidatingPolicy) GetStatus() *ValidatingPolicyStatus {
	return &s.Status
}

func (s *ValidatingPolicy) GetKind() string {
	return "ValidatingPolicy"
}

func (status *ValidatingPolicyStatus) GetConditionStatus() *ConditionStatus {
	return &status.ConditionStatus
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ValidatingPolicyList is a list of ValidatingPolicy instances
type ValidatingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ValidatingPolicy `json:"items"`
}

// ValidatingPolicySpec is the specification of the desired behavior of the ValidatingPolicy.
type ValidatingPolicySpec struct {
	// MatchConstraints specifies what resources this policy is designed to validate.
	// The AdmissionPolicy cares about a request if it matches _all_ Constraints.
	// However, in order to prevent clusters from being put into an unstable state that cannot be recovered from via the API
	// ValidatingAdmissionPolicy cannot match ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding.
	// Required.
	MatchConstraints *admissionregistrationv1.MatchResources `json:"matchConstraints,omitempty"`

	// Validations contain CEL expressions which is used to apply the validation.
	// Validations and AuditAnnotations may not both be empty; a minimum of one Validations or AuditAnnotations is
	// required.
	// +listType=atomic
	// +optional
	Validations []admissionregistrationv1.Validation `json:"validations,omitempty"`

	// failurePolicy defines how to handle failures for the admission policy. Failures can
	// occur from CEL expression parse errors, type check errors, runtime errors and invalid
	// or mis-configured policy definitions or bindings.
	//
	// A policy is invalid if spec.paramKind refers to a non-existent Kind.
	// A binding is invalid if spec.paramRef.name refers to a non-existent resource.
	//
	// failurePolicy does not define how validations that evaluate to false are handled.
	//
	// When failurePolicy is set to Fail, ValidatingAdmissionPolicyBinding validationActions
	// define how failures are enforced.
	//
	// Allowed values are Ignore or Fail. Defaults to Fail.
	// +optional
	FailurePolicy *admissionregistrationv1.FailurePolicyType `json:"failurePolicy,omitempty"`

	// auditAnnotations contains CEL expressions which are used to produce audit
	// annotations for the audit event of the API request.
	// validations and auditAnnotations may not both be empty; a least one of validations or auditAnnotations is
	// required.
	// +listType=atomic
	// +optional
	AuditAnnotations []admissionregistrationv1.AuditAnnotation `json:"auditAnnotations,omitempty"`

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

	// GenerationConfiguration defines the configuration for the generation controller.
	// +optional
	GenerationConfiguration *GenerationConfiguration `json:"generation,omitempty"`

	// ValidationAction specifies the action to be taken when the matched resource violates the policy.
	// Required.
	// +listType=set
	ValidationAction []admissionregistrationv1.ValidationAction `json:"validationActions,omitempty"`

	// WebhookConfiguration defines the configuration for the webhook.
	// +optional
	WebhookConfiguration *WebhookConfiguration `json:"webhookConfiguration,omitempty"`

	// EvaluationConfiguration defines the configuration for the policy evaluation.
	// +optional
	EvaluationConfiguration *EvaluationConfiguration `json:"evaluation,omitempty"`
}

// GenerateValidatingAdmissionPolicyEnabled checks if validating admission policy generation is enabled
func (s ValidatingPolicySpec) GenerateValidatingAdmissionPolicyEnabled() bool {
	const defaultValue = false
	if s.GenerationConfiguration == nil {
		return defaultValue
	}
	if s.GenerationConfiguration.ValidatingAdmissionPolicy == nil {
		return defaultValue
	}
	if s.GenerationConfiguration.ValidatingAdmissionPolicy.Enabled == nil {
		return defaultValue
	}
	return *s.GenerationConfiguration.ValidatingAdmissionPolicy.Enabled
}

// AdmissionEnabled checks if admission is set to true
func (s ValidatingPolicySpec) AdmissionEnabled() bool {
	const defaultValue = true
	if s.EvaluationConfiguration == nil || s.EvaluationConfiguration.Admission == nil || s.EvaluationConfiguration.Admission.Enabled == nil {
		return defaultValue
	}
	return *s.EvaluationConfiguration.Admission.Enabled
}

// BackgroundEnabled checks if background is set to true
func (s ValidatingPolicySpec) BackgroundEnabled() bool {
	const defaultValue = true
	if s.EvaluationConfiguration == nil || s.EvaluationConfiguration.Background == nil || s.EvaluationConfiguration.Background.Enabled == nil {
		return defaultValue
	}
	return *s.EvaluationConfiguration.Background.Enabled
}

// EvaluationMode returns the evaluation mode of the policy.
func (s ValidatingPolicySpec) EvaluationMode() EvaluationMode {
	const defaultValue = EvaluationModeKubernetes
	if s.EvaluationConfiguration == nil || s.EvaluationConfiguration.Mode == "" {
		return defaultValue
	}
	return s.EvaluationConfiguration.Mode
}

type GenerationConfiguration struct {
	// PodControllers specifies whether to generate a pod controllers rules.
	PodControllers *PodControllersGenerationConfiguration `json:"podControllers,omitempty"`
	// ValidatingAdmissionPolicy specifies whether to generate a Kubernetes ValidatingAdmissionPolicy.
	ValidatingAdmissionPolicy *VapGenerationConfiguration `json:"validatingAdmissionPolicy,omitempty"`
}

type PodControllersGenerationConfiguration struct {
	// TODO: shall we use GVK/GVR instead of string ?
	Controllers []string `json:"controllers,omitempty"`
}

type VapGenerationConfiguration struct {
	// Enabled specifies whether to generate a Kubernetes ValidatingAdmissionPolicy.
	// Optional. Defaults to "false" if not specified.
	Enabled *bool `json:"enabled,omitempty"`
}

type WebhookConfiguration struct {
	// TimeoutSeconds specifies the maximum time in seconds allowed to apply this policy.
	// After the configured time expires, the admission request may fail, or may simply ignore the policy results,
	// based on the failure policy. The default timeout is 10s, the value must be between 1 and 30 seconds.
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
}

type EvaluationConfiguration struct {
	// Mode is the mode of policy evaluation.
	// Allowed values are "Kubernetes" or "JSON".
	// Optional. Default value is "Kubernetes".
	// +optional
	Mode EvaluationMode `json:"mode,omitempty"`

	// Admission controls policy evaluation during admission.
	// +optional
	Admission *AdmissionConfiguration `json:"admission,omitempty"`

	// Background  controls policy evaluation during background scan.
	// +optional
	Background *BackgroundConfiguration `json:"background,omitempty"`
}

type AdmissionConfiguration struct {
	// Enabled controls if rules are applied during admission.
	// Optional. Default value is "true".
	// +optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`
}

type BackgroundConfiguration struct {
	// Enabled controls if rules are applied to existing resources during a background scan.
	// Optional. Default value is "true". The value must be set to "false" if the policy rule
	// uses variables that are only available in the admission review request (e.g. user name).
	// +optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`
}
