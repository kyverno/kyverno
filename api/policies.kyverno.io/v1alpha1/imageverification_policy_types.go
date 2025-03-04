package v1alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=imageverificationpolicies,scope="Cluster",shortName=ivpol,categories=kyverno
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ImageVerificationPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ImageVerificationPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status PolicyStatus `json:"status,omitempty"`
}

func (s *ImageVerificationPolicy) GetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.MatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}
	return *s.Spec.MatchConstraints
}

func (s *ImageVerificationPolicy) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	return s.Spec.MatchConditions
}

func (s *ImageVerificationPolicy) GetWebhookConfiguration() *WebhookConfiguration {
	return s.Spec.WebhookConfiguration
}

func (s *ImageVerificationPolicy) GetFailurePolicy() admissionregistrationv1.FailurePolicyType {
	if s.Spec.FailurePolicy == nil {
		return admissionregistrationv1.Fail
	}
	return *s.Spec.FailurePolicy
}

func (s *ImageVerificationPolicy) GetVariables() []admissionregistrationv1.Variable {
	return s.Spec.Variables
}

func (s *ImageVerificationPolicy) GetSpec() *ImageVerificationPolicySpec {
	return &s.Spec
}

func (s *ImageVerificationPolicy) GetStatus() *PolicyStatus {
	return &s.Status
}

func (s *ImageVerificationPolicy) GetKind() string {
	return "ImageVerificationPolicy"
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImageVerificationPolicyList is a list of ImageVerificationPolicy instances
type ImageVerificationPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ImageVerificationPolicy `json:"items"`
}
