package v2alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

// +kubebuilder:object:generate=false
type GenericPolicy interface {
	GetMatchConstraints() admissionregistrationv1.MatchResources
	GetMatchConditions() []admissionregistrationv1.MatchCondition
	GetFailurePolicy() admissionregistrationv1.FailurePolicyType
	GetVariables() []admissionregistrationv1.Variable
}
