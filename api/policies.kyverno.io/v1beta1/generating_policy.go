package v1beta1

import (
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	GeneratingPolicyKind           = "GeneratingPolicy"
	NamespacedGeneratingPolicyKind = "NamespacedGeneratingPolicy"
)

type (
	GeneratingPolicySpec                        = v1alpha1.GeneratingPolicySpec
	GeneratingPolicyStatus                      = v1alpha1.GeneratingPolicyStatus
	GeneratingPolicyEvaluationConfiguration     = v1alpha1.GeneratingPolicyEvaluationConfiguration
	OrphanDownstreamOnPolicyDeleteConfiguration = v1alpha1.OrphanDownstreamOnPolicyDeleteConfiguration
	GenerateExistingConfiguration               = v1alpha1.GenerateExistingConfiguration
	SynchronizationConfiguration                = v1alpha1.SynchronizationConfiguration
	Generation                                  = v1alpha1.Generation
)

// GeneratingPolicyLike captures the common behaviour shared by generating policies regardless of scope.
// +k8s:deepcopy-gen=false
type GeneratingPolicyLike interface {
	metav1.Object
	runtime.Object
	GetSpec() *GeneratingPolicySpec
	GetStatus() *GeneratingPolicyStatus
	GetMatchConstraints() admissionregistrationv1.MatchResources
	GetMatchConditions() []admissionregistrationv1.MatchCondition
	GetVariables() []admissionregistrationv1.Variable
	GetKind() string
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=generatingpolicies,scope="Cluster",shortName=gpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion

type GeneratingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              GeneratingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status GeneratingPolicyStatus `json:"status,omitempty"`
}

func (s *GeneratingPolicy) GetKind() string {
	return GeneratingPolicyKind
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

func (s *GeneratingPolicy) GetTimeoutSeconds() *int32 {
	if s.Spec.WebhookConfiguration == nil {
		return nil
	}

	return s.Spec.WebhookConfiguration.TimeoutSeconds
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

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=ngpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//
// NamespacedGeneratingPolicy is the namespaced CEL-based generating policy.
type NamespacedGeneratingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              GeneratingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status GeneratingPolicyStatus `json:"status,omitempty"`
}

func (s *NamespacedGeneratingPolicy) GetKind() string {
	return NamespacedGeneratingPolicyKind
}

func (s *NamespacedGeneratingPolicy) GetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.MatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}
	return *s.Spec.MatchConstraints
}

func (s *NamespacedGeneratingPolicy) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	return s.Spec.MatchConditions
}

func (s *NamespacedGeneratingPolicy) GetFailurePolicy() admissionregistrationv1.FailurePolicyType {
	return admissionregistrationv1.Ignore
}

func (s *NamespacedGeneratingPolicy) GetTimeoutSeconds() *int32 {
	if s.Spec.WebhookConfiguration == nil {
		return nil
	}

	return s.Spec.WebhookConfiguration.TimeoutSeconds
}

func (s *NamespacedGeneratingPolicy) GetVariables() []admissionregistrationv1.Variable {
	return s.Spec.Variables
}

func (s *NamespacedGeneratingPolicy) GetSpec() *GeneratingPolicySpec {
	return &s.Spec
}

func (s *NamespacedGeneratingPolicy) GetStatus() *GeneratingPolicyStatus {
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

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//
// NamespacedGeneratingPolicyList is a list of NamespacedGeneratingPolicy instances
type NamespacedGeneratingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []NamespacedGeneratingPolicy `json:"items"`
}
