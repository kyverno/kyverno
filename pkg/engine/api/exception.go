package api

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GenericException abstracts the exception type (PolicyException, PolicyException)
type GenericException interface {
	metav1.Object
	// GetAPIVersion returns policy API version
	GetAPIVersion() string
	// GetKind returns policy kind
	GetKind() string
	// AsException returns the policy exception
	AsException() *kyvernov2.PolicyException
	// AsCELException returns the CEL policy exception
	AsCELException() *policiesv1beta1.PolicyException
}

type genericException struct {
	metav1.Object
	PolicyException    *kyvernov2.PolicyException
	CELPolicyException *policiesv1beta1.PolicyException
}

func (p *genericException) AsException() *kyvernov2.PolicyException {
	return p.PolicyException
}

func (p *genericException) AsCELException() *policiesv1beta1.PolicyException {
	return p.CELPolicyException
}

func (p *genericException) GetAPIVersion() string {
	switch {
	case p.PolicyException != nil:
		return kyvernov2.GroupVersion.String()
	case p.CELPolicyException != nil:
		return policiesv1beta1.GroupVersion.String()
	}
	return ""
}

func (p *genericException) GetKind() string {
	switch {
	case p.PolicyException != nil:
		return "PolicyException"
	case p.CELPolicyException != nil:
		return "CELPolicyException"
	}
	return ""
}

func NewPolicyException(polex *kyvernov2.PolicyException) GenericException {
	return &genericException{
		Object:          polex,
		PolicyException: polex,
	}
}

func NewCELPolicyException(polex *policiesv1beta1.PolicyException) GenericException {
	return &genericException{
		Object:             polex,
		CELPolicyException: polex,
	}
}
