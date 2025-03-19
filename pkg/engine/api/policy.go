package api

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
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
	AsValidatingPolicy() *policiesv1alpha1.ValidatingPolicy
	// AsImageVerificationPolicy returns the imageverificationpolicy
	AsImageVerificationPolicy() *policiesv1alpha1.ImageValidatingPolicy
}

type genericPolicy struct {
	metav1.Object
	PolicyInterface           kyvernov1.PolicyInterface
	ValidatingAdmissionPolicy *admissionregistrationv1.ValidatingAdmissionPolicy
	MutatingAdmissionPolicy   *admissionregistrationv1alpha1.MutatingAdmissionPolicy
	ValidatingPolicy          *policiesv1alpha1.ValidatingPolicy
	ImageValidatingPolicy     *policiesv1alpha1.ImageValidatingPolicy
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

func (p *genericPolicy) AsValidatingPolicy() *policiesv1alpha1.ValidatingPolicy {
	return p.ValidatingPolicy
}

func (p *genericPolicy) AsImageVerificationPolicy() *policiesv1alpha1.ImageValidatingPolicy {
	return p.ImageValidatingPolicy
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
		return policiesv1alpha1.GroupVersion.String()
	case p.ImageValidatingPolicy != nil:
		return policiesv1alpha1.GroupVersion.String()
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
	case p.ImageValidatingPolicy != nil:
		return "ImageValidatingPolicy"
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

func NewValidatingPolicy(pol *policiesv1alpha1.ValidatingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:           pol,
		ValidatingPolicy: pol,
	}
}

func NewImageVerificationPolicy(pol *policiesv1alpha1.ImageValidatingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:                pol,
		ImageValidatingPolicy: pol,
	}
}
