package v1beta1

import (
	"context"

	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/toggle"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Spec              v1alpha1.ImageValidatingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status v1alpha1.ImageValidatingPolicyStatus `json:"status,omitempty"`
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

func (s *ImageValidatingPolicy) GetWebhookConfiguration() *v1alpha1.WebhookConfiguration {
	return s.Spec.WebhookConfiguration
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

func (s *ImageValidatingPolicy) GetSpec() *v1alpha1.ImageValidatingPolicySpec {
	return &s.Spec
}

func (s *ImageValidatingPolicy) GetStatus() *v1alpha1.ImageValidatingPolicyStatus {
	return &s.Status
}

func (s *ImageValidatingPolicy) GetKind() string {
	return "ImageValidatingPolicy"
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImageValidatingPolicyList is a list of ImageValidatingPolicy instances
type ImageValidatingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ImageValidatingPolicy `json:"items"`
}
