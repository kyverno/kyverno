package v1alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:generate=false
type GenericPolicy interface {
	metav1.Object
	GetMatchConstraints() admissionregistrationv1.MatchResources
	GetMatchConditions() []admissionregistrationv1.MatchCondition
	GetFailurePolicy() admissionregistrationv1.FailurePolicyType
	GetWebhookConfiguration() *WebhookConfiguration
	GetVariables() []admissionregistrationv1.Variable
	GetStatus() *PolicyStatus
	GetKind() string
}

// ValidatingPolicyInterface extends Policy interface with validating-specific methods
type ValidatingPolicyInterface interface {
	GenericPolicy
	GetSpec() *ValidatingPolicySpec
	GetAuditAnnotations() []admissionregistrationv1.AuditAnnotation
	GetValidations() []admissionregistrationv1.Validation
	GetValidationActions() []admissionregistrationv1.ValidationAction
}

// MutatingPolicyInterface extends Policy interface with mutating-specific methods
type MutatingPolicyInterface interface {
	GenericPolicy
	GetSpec() *MutatingPolicySpec
	GetMutations() []admissionregistrationv1alpha1.Mutation
	GetReinvocationPolicy() admissionregistrationv1.ReinvocationPolicyType
	GetParamKind() *admissionregistrationv1.ParamKind
}
