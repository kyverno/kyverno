package v1alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=validatingpolicies,scope="Cluster",shortName=vpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditionStatus.ready`
// +kubebuilder:selectablefield:JSONPath=`.spec.evaluation.mode`
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion

type ValidatingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ValidatingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status ValidatingPolicyStatus `json:"status,omitempty"`
}

// ValidatingPolicyLike captures the common behaviour shared by validating policies regardless of scope.
// +k8s:deepcopy-gen=false
type ValidatingPolicyLike interface {
	metav1.Object
	runtime.Object
	GetSpec() *ValidatingPolicySpec
	GetStatus() *ValidatingPolicyStatus
	GetFailurePolicy() admissionregistrationv1.FailurePolicyType
	GetMatchConstraints() admissionregistrationv1.MatchResources
	GetMatchConditions() []admissionregistrationv1.MatchCondition
	GetVariables() []admissionregistrationv1.Variable
	GetValidatingPolicySpec() *ValidatingPolicySpec
	BackgroundEnabled() bool
	GetKind() string
}

type ValidatingPolicyStatus struct {
	// +optional
	ConditionStatus ConditionStatus `json:"conditionStatus,omitempty"`

	// +optional
	Autogen ValidatingPolicyAutogenStatus `json:"autogen,omitempty"`

	// Generated indicates whether a ValidatingAdmissionPolicy/MutatingAdmissionPolicy is generated from the policy or not
	// +optional
	Generated bool `json:"generated"`
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
	// failurePolicy does not define how validations that evaluate to false are handled.
	//
	// When failurePolicy is set to Fail, the validationActions field define how failures are enforced.
	//
	// Allowed values are Ignore or Fail. Defaults to Fail.
	// +optional
	// +kubebuilder:validation:Enum=Ignore;Fail
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
	AutogenConfiguration *ValidatingPolicyAutogenConfiguration `json:"autogen,omitempty"`

	// ValidationAction specifies the action to be taken when the matched resource violates the policy.
	// If a validation evaluates to false it is always enforced according to these actions.
	//
	// Failures defined by the ValidatingAdmissionPolicy's FailurePolicy are enforced according
	// to these actions only if the FailurePolicy is set to Fail, otherwise the failures are
	// ignored. This includes compilation errors, runtime errors and misconfigurations of the policy.
	//
	// validationActions is declared as a set of action values. Order does
	// not matter. validationActions may not contain duplicates of the same action.
	//
	// The supported actions values are:
	//
	// "Deny" specifies that a validation failure results in a denied request.
	//
	// "Warn" specifies that a validation failure is reported to the request client
	// in HTTP Warning headers, with a warning code of 299. Warnings can be sent
	// both for allowed or denied admission responses.
	//
	// "Audit" specifies that a validation failure is recorded in the created reports.
	//
	// Clients should expect to handle additional values by ignoring
	// any values not recognized.
	//
	// "Deny" and "Warn" may not be used together since this combination
	// needlessly duplicates the validation failure both in the
	// API response body and the HTTP warning headers.
	//
	// Required.
	// +listType=set
	// +kubebuilder:validation:items:Enum=Deny;Audit;Warn
	ValidationAction []admissionregistrationv1.ValidationAction `json:"validationActions,omitempty"`

	// WebhookConfiguration defines the configuration for the webhook.
	// +optional
	WebhookConfiguration *WebhookConfiguration `json:"webhookConfiguration,omitempty"`

	// EvaluationConfiguration defines the configuration for the policy evaluation.
	// +optional
	EvaluationConfiguration *EvaluationConfiguration `json:"evaluation,omitempty"`
}

type ValidatingPolicyAutogenConfiguration struct {
	// PodControllers specifies whether to generate a pod controllers rules.
	PodControllers *PodControllersGenerationConfiguration `json:"podControllers,omitempty"`
	// ValidatingAdmissionPolicy specifies whether to generate a Kubernetes ValidatingAdmissionPolicy.
	ValidatingAdmissionPolicy *VapGenerationConfiguration `json:"validatingAdmissionPolicy,omitempty"`
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
