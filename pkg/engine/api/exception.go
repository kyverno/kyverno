package api

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GenericException abstracts the exception type (CEL PolicyException)
type GenericException interface {
	metav1.Object
	// GetAPIVersion returns policy API version
	GetAPIVersion() string
	// GetKind returns policy kind
	GetKind() string
	// AsCELException returns the CEL policy exception
	AsCELException() *policiesv1beta1.PolicyException
}

type genericException struct {
	metav1.Object
	CELPolicyException *policiesv1beta1.PolicyException
}

func (p *genericException) AsCELException() *policiesv1beta1.PolicyException {
	return p.CELPolicyException
}

func (p *genericException) GetAPIVersion() string {
	switch {
	case p.CELPolicyException != nil:
		return policiesv1beta1.GroupVersion.String()
	}
	return ""
}

func (p *genericException) GetKind() string {
	switch {
	case p.CELPolicyException != nil:
		return "CELPolicyException"
	}
	return ""
}

func NewCELPolicyException(polex *policiesv1beta1.PolicyException) GenericException {
	return &genericException{
		Object:             polex,
		CELPolicyException: polex,
	}
}
