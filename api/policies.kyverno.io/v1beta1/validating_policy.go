package v1beta1

import (
	"context"

	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/toggle"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ValidatingPolicyKind           = "ValidatingPolicy"
	NamespacedValidatingPolicyKind = "NamespacedValidatingPolicy"
)

type (
	WebhookConfiguration                 = v1alpha1.WebhookConfiguration
	ValidatingPolicySpec                 = v1alpha1.ValidatingPolicySpec
	ValidatingPolicyStatus               = v1alpha1.ValidatingPolicyStatus
	VapGenerationConfiguration           = v1alpha1.VapGenerationConfiguration
	ValidatingPolicyAutogenConfiguration = v1alpha1.ValidatingPolicyAutogenConfiguration
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=nvpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditionStatus.ready`
// +kubebuilder:selectablefield:JSONPath=`.spec.evaluation.mode`
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type NamespacedValidatingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ValidatingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status ValidatingPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NamespacedValidatingPolicyList is a list of NamespacedValidatingPolicy instances
type NamespacedValidatingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []NamespacedValidatingPolicy `json:"items"`
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

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=validatingpolicies,scope="Cluster",shortName=vpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditionStatus.ready`
// +kubebuilder:selectablefield:JSONPath=`.spec.evaluation.mode`
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ValidatingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ValidatingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status ValidatingPolicyStatus `json:"status,omitempty"`
}

// BackgroundEnabled checks if background is set to true
func (s ValidatingPolicy) BackgroundEnabled() bool {
	return s.Spec.BackgroundEnabled()
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
	if toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore() {
		return admissionregistrationv1.Ignore
	}
	if s.Spec.FailurePolicy == nil {
		return admissionregistrationv1.Fail
	}
	return *s.Spec.FailurePolicy
}

func (s *ValidatingPolicy) GetTimeoutSeconds() *int32 {
	if s.Spec.WebhookConfiguration == nil {
		return nil
	}

	return s.Spec.WebhookConfiguration.TimeoutSeconds
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
	return ValidatingPolicyKind
}

func (s *ValidatingPolicy) GetValidatingPolicySpec() *ValidatingPolicySpec {
	return &s.Spec
}

func (s *NamespacedValidatingPolicy) GetSpec() *ValidatingPolicySpec {
	return &s.Spec
}

func (s *NamespacedValidatingPolicy) GetStatus() *ValidatingPolicyStatus {
	return &s.Status
}

func (s *NamespacedValidatingPolicy) GetKind() string {
	return NamespacedValidatingPolicyKind
}

func (s *NamespacedValidatingPolicy) GetValidatingPolicySpec() *ValidatingPolicySpec {
	return &s.Spec
}

func (s *NamespacedValidatingPolicy) GetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.MatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}
	return *s.Spec.MatchConstraints
}

func (s *NamespacedValidatingPolicy) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	return s.Spec.MatchConditions
}

func (s *NamespacedValidatingPolicy) GetFailurePolicy() admissionregistrationv1.FailurePolicyType {
	if toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore() {
		return admissionregistrationv1.Ignore
	}
	if s.Spec.FailurePolicy == nil {
		return admissionregistrationv1.Fail
	}
	return *s.Spec.FailurePolicy
}

func (s *NamespacedValidatingPolicy) GetTimeoutSeconds() *int32 {
	if s.Spec.WebhookConfiguration == nil {
		return nil
	}

	return s.Spec.WebhookConfiguration.TimeoutSeconds
}

func (s *NamespacedValidatingPolicy) GetVariables() []admissionregistrationv1.Variable {
	return s.Spec.Variables
}

// BackgroundEnabled checks if background is set to true
func (s NamespacedValidatingPolicy) BackgroundEnabled() bool {
	return s.Spec.BackgroundEnabled()
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ValidatingPolicyList is a list of ValidatingPolicy instances
type ValidatingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ValidatingPolicy `json:"items"`
}
