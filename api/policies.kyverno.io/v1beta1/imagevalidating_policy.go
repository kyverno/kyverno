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
	ImageValidatingPolicyKind           = "ImageValidatingPolicy"
	NamespacedImageValidatingPolicyKind = "NamespacedImageValidatingPolicy"
)

type (
	ImageValidatingPolicySpec                 = v1alpha1.ImageValidatingPolicySpec
	ImageValidatingPolicyStatus               = v1alpha1.ImageValidatingPolicyStatus
	ImageValidatingPolicyAutogenConfiguration = v1alpha1.ImageValidatingPolicyAutogenConfiguration
	MatchImageReference                       = v1alpha1.MatchImageReference
	ImageExtractor                            = v1alpha1.ImageExtractor
	StringOrExpression                        = v1alpha1.StringOrExpression
	Attestation                               = v1alpha1.Attestation
	InToto                                    = v1alpha1.InToto
	Referrer                                  = v1alpha1.Referrer
	Attestor                                  = v1alpha1.Attestor
	Cosign                                    = v1alpha1.Cosign
	Notary                                    = v1alpha1.Notary
	Credentials                               = v1alpha1.Credentials
	Certificate                               = v1alpha1.Certificate
	Key                                       = v1alpha1.Key
	TUF                                       = v1alpha1.TUF
	Source                                    = v1alpha1.Source
	CTLog                                     = v1alpha1.CTLog
	Keyless                                   = v1alpha1.Keyless
	Identity                                  = v1alpha1.Identity
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=imagevalidatingpolicies,scope="Cluster",shortName=ivpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditionStatus.ready`
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ImageValidatingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ImageValidatingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status ImageValidatingPolicyStatus `json:"status,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=nivpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditionStatus.ready`
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type NamespacedImageValidatingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ImageValidatingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status ImageValidatingPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImageValidatingPolicyList is a list of ImageValidatingPolicy instances
type ImageValidatingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ImageValidatingPolicy `json:"items"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NamespacedImageValidatingPolicyList is a list of NamespacedImageValidatingPolicy instances
type NamespacedImageValidatingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []NamespacedImageValidatingPolicy `json:"items"`
}

// ImageValidatingPolicyLike captures the common behaviour shared by image validating policies regardless of scope.
// +k8s:deepcopy-gen=false
type ImageValidatingPolicyLike interface {
	metav1.Object
	runtime.Object
	GetSpec() *ImageValidatingPolicySpec
	GetStatus() *ImageValidatingPolicyStatus
	GetFailurePolicy() admissionregistrationv1.FailurePolicyType
	GetMatchConstraints() admissionregistrationv1.MatchResources
	GetMatchConditions() []admissionregistrationv1.MatchCondition
	GetVariables() []admissionregistrationv1.Variable
	GetWebhookConfiguration() *WebhookConfiguration
	BackgroundEnabled() bool
	GetKind() string
}

// BackgroundEnabled checks if background is set to true
func (s ImageValidatingPolicy) BackgroundEnabled() bool {
	return s.Spec.BackgroundEnabled()
}

func (s *ImageValidatingPolicy) GetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.MatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}
	return *s.Spec.MatchConstraints
}

func (s *ImageValidatingPolicy) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	return s.Spec.MatchConditions
}

func (s *ImageValidatingPolicy) GetTimeoutSeconds() *int32 {
	if s.Spec.WebhookConfiguration == nil {
		return nil
	}

	return s.Spec.WebhookConfiguration.TimeoutSeconds
}

func (s *ImageValidatingPolicy) GetFailurePolicy() admissionregistrationv1.FailurePolicyType {
	if toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore() {
		return admissionregistrationv1.Ignore
	}
	if s.Spec.FailurePolicy == nil {
		return admissionregistrationv1.Fail
	}
	return *s.Spec.FailurePolicy
}

func (s *ImageValidatingPolicy) GetVariables() []admissionregistrationv1.Variable {
	return s.Spec.Variables
}

func (s *ImageValidatingPolicy) GetWebhookConfiguration() *WebhookConfiguration {
	return s.Spec.WebhookConfiguration
}

func (s *ImageValidatingPolicy) GetSpec() *ImageValidatingPolicySpec {
	return &s.Spec
}

func (s *ImageValidatingPolicy) GetStatus() *ImageValidatingPolicyStatus {
	return &s.Status
}

func (s *ImageValidatingPolicy) GetKind() string {
	return ImageValidatingPolicyKind
}

// NamespacedImageValidatingPolicy methods

func (s *NamespacedImageValidatingPolicy) GetSpec() *ImageValidatingPolicySpec {
	return &s.Spec
}

func (s *NamespacedImageValidatingPolicy) GetStatus() *ImageValidatingPolicyStatus {
	return &s.Status
}

func (s *NamespacedImageValidatingPolicy) GetKind() string {
	return NamespacedImageValidatingPolicyKind
}

func (s *NamespacedImageValidatingPolicy) GetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.MatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}
	return *s.Spec.MatchConstraints
}

func (s *NamespacedImageValidatingPolicy) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	return s.Spec.MatchConditions
}

func (s *NamespacedImageValidatingPolicy) GetTimeoutSeconds() *int32 {
	if s.Spec.WebhookConfiguration == nil {
		return nil
	}

	return s.Spec.WebhookConfiguration.TimeoutSeconds
}

func (s *NamespacedImageValidatingPolicy) GetWebhookConfiguration() *WebhookConfiguration {
	return s.Spec.WebhookConfiguration
}

func (s *NamespacedImageValidatingPolicy) GetFailurePolicy() admissionregistrationv1.FailurePolicyType {
	if toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore() {
		return admissionregistrationv1.Ignore
	}
	if s.Spec.FailurePolicy == nil {
		return admissionregistrationv1.Fail
	}
	return *s.Spec.FailurePolicy
}

func (s *NamespacedImageValidatingPolicy) GetVariables() []admissionregistrationv1.Variable {
	return s.Spec.Variables
}

// BackgroundEnabled checks if background is set to true
func (s NamespacedImageValidatingPolicy) BackgroundEnabled() bool {
	return s.Spec.BackgroundEnabled()
}
