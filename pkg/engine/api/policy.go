package api

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GenericPolicy abstracts the policy type (ClusterPolicy/Policy, ValidatingPolicy, ValidatingAdmissionPolicy and MutatingAdmissionPolicy)
// It is intended to be used in EngineResponse
type GenericPolicy interface {
	metav1.Object
	// GetAPIVersion returns policy API version
	GetAPIVersion() string
	// GetKind returns policy kind
	GetKind() string
	// IsNamespaced indicates if the policy is namespace scoped
	IsNamespaced() bool
	// AsObject returns the raw underlying policy
	AsObject() any
	// AsKyvernoPolicy returns the kyverno policy
	AsKyvernoPolicy() kyvernov1.PolicyInterface
	// AsValidatingAdmissionPolicy returns the validating admission policy
	AsValidatingAdmissionPolicy() *admissionregistrationv1.ValidatingAdmissionPolicy
	// AsValidatingPolicy returns the validating policy
	AsValidatingPolicy() *kyvernov2alpha1.ValidatingPolicy
}

type genericPolicy struct {
	metav1.Object
	PolicyInterface           kyvernov1.PolicyInterface
	ValidatingAdmissionPolicy *admissionregistrationv1.ValidatingAdmissionPolicy
	MutatingAdmissionPolicy   *admissionregistrationv1alpha1.MutatingAdmissionPolicy
	ValidatingPolicy          *kyvernov2alpha1.ValidatingPolicy
}

func (p *genericPolicy) AsObject() any {
	return p.Object
}

func (p *genericPolicy) AsKyvernoPolicy() kyvernov1.PolicyInterface {
	return p.PolicyInterface
}

func (p *genericPolicy) AsValidatingAdmissionPolicy() *admissionregistrationv1.ValidatingAdmissionPolicy {
	return p.ValidatingAdmissionPolicy
}

func (p *genericPolicy) AsValidatingPolicy() *kyvernov2alpha1.ValidatingPolicy {
	return p.ValidatingPolicy
}

func (p *genericPolicy) GetAPIVersion() string {
	switch {
	case p.PolicyInterface != nil:
		return kyvernov1.GroupVersion.String()
	case p.ValidatingAdmissionPolicy != nil:
		return admissionregistrationv1.SchemeGroupVersion.String()
	case p.MutatingAdmissionPolicy != nil:
		return admissionregistrationv1alpha1.SchemeGroupVersion.String()
	case p.ValidatingPolicy != nil:
		return kyvernov2alpha1.GroupVersion.String()
	}
	return ""
}

func (p *genericPolicy) GetKind() string {
	switch {
	case p.PolicyInterface != nil:
		return p.PolicyInterface.GetKind()
	case p.ValidatingAdmissionPolicy != nil:
		return "ValidatingAdmissionPolicy"
	case p.MutatingAdmissionPolicy != nil:
		return "MutatingAdmissionPolicy"
	case p.ValidatingPolicy != nil:
		return "ValidatingPolicy"
	}
	return ""
}

func (p *genericPolicy) IsNamespaced() bool {
	switch {
	case p.PolicyInterface != nil:
		return p.PolicyInterface.IsNamespaced()
	}
	return false
}

func NewKyvernoPolicy(pol kyvernov1.PolicyInterface) GenericPolicy {
	return &genericPolicy{
		Object:          pol,
		PolicyInterface: pol,
	}
}

func NewValidatingAdmissionPolicy(pol *admissionregistrationv1.ValidatingAdmissionPolicy) GenericPolicy {
	return &genericPolicy{
		Object:                    pol,
		ValidatingAdmissionPolicy: pol,
	}
}

func NewMutatingAdmissionPolicy(pol *admissionregistrationv1alpha1.MutatingAdmissionPolicy) GenericPolicy {
	return &genericPolicy{
		Object:                  pol,
		MutatingAdmissionPolicy: pol,
	}
}

func NewValidatingPolicy(pol *kyvernov2alpha1.ValidatingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:           pol,
		ValidatingPolicy: pol,
	}
}
