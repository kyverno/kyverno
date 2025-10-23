package v1alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:generate=false
type GenericPolicy interface {
	metav1.Object
	GetMatchConstraints() admissionregistrationv1.MatchResources
	GetMatchConditions() []admissionregistrationv1.MatchCondition
	GetFailurePolicy() admissionregistrationv1.FailurePolicyType
	GetTimeoutSeconds() *int32
	GetVariables() []admissionregistrationv1.Variable
}
