package v1beta1

import (
	"context"

	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/toggle"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type (
	MutatingPolicySpec                    = v1alpha1.MutatingPolicySpec
	MutatingPolicyStatus                  = v1alpha1.MutatingPolicyStatus
	MutatingPolicyAutogenConfiguration    = v1alpha1.MutatingPolicyAutogenConfiguration
	MAPGenerationConfiguration            = v1alpha1.MAPGenerationConfiguration
	MutatingPolicyEvaluationConfiguration = v1alpha1.MutatingPolicyEvaluationConfiguration
	MutateExistingConfiguration           = v1alpha1.MutateExistingConfiguration
)

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

func (s *MutatingPolicy) GetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.MatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}
	return *s.Spec.MatchConstraints
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
