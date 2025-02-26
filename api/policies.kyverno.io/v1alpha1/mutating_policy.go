package v1alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=mutatingpolicies,scope="Cluster",shortName=mpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MutatingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MutatingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status PolicyStatus `json:"status,omitempty"`
}

func (s *MutatingPolicy) GetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.MatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}
	return *s.Spec.MatchConstraints
}

func (s *MutatingPolicy) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	return s.Spec.MatchConditions
}

func (s *MutatingPolicy) GetFailurePolicy() admissionregistrationv1.FailurePolicyType {
	if s.Spec.FailurePolicy == nil {
		return admissionregistrationv1.Fail
	}
	return *s.Spec.FailurePolicy
}

func (s *MutatingPolicy) GetWebhookConfiguration() *WebhookConfiguration {
	return s.Spec.WebhookConfiguration
}

func (s *MutatingPolicy) GetVariables() []admissionregistrationv1.Variable {
	return s.Spec.Variables
}

func (s *MutatingPolicy) GetStatus() *PolicyStatus {
	return &s.Status
}

func (s *MutatingPolicy) GetKind() string {
	return "MutatingPolicy"
}

func (s *MutatingPolicy) GetSpec() *MutatingPolicySpec {
	return &s.Spec
}

func (s *MutatingPolicy) GetMutations() []admissionregistrationv1alpha1.Mutation {
	return s.Spec.Mutations
}

func (s *MutatingPolicy) GetReinvocationPolicy() admissionregistrationv1.ReinvocationPolicyType {
	return s.Spec.ReinvocationPolicy
}

func (s *MutatingPolicy) GetParamKind() *admissionregistrationv1.ParamKind {
	return s.Spec.ParamKind
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MutatingPolicyList is a list of MutatingPolicy instances
type MutatingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []MutatingPolicy `json:"items"`
}
