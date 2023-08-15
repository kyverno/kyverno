package api

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/api/admissionregistration/v1alpha1"
)

// PolicyType represents the type of a policy
type PolicyType string

const (
	// KyvernoPolicy type for kyverno policies
	KyvernoPolicyType PolicyType = "KyvernoPolicy"
	// ValidatingAdmissionPolicy for validating admission policies
	ValidatingAdmissionPolicyType PolicyType = "ValidatingAdmissionPolicy"
)

type Policy interface {
	// GetPolicy returns either kyverno policy or validating admission policy
	GetPolicy() interface{}
	// GetType returns policy type
	GetType() PolicyType
	// GetName returns policy name
	GetName() string
	// GetNamespace returns policy namespace
	GetNamespace() string
	// GetAnnotations returns policy annotations
	GetAnnotations() map[string]string
}

type KyvernoPolicy struct {
	policy kyvernov1.PolicyInterface
}

func (p KyvernoPolicy) GetPolicy() interface{} {
	return p.policy
}

func (p KyvernoPolicy) GetType() PolicyType {
	return KyvernoPolicyType
}

func (p KyvernoPolicy) GetName() string {
	return p.policy.GetName()
}

func (p KyvernoPolicy) GetNamespace() string {
	return p.policy.GetNamespace()
}

func (p KyvernoPolicy) GetAnnotations() map[string]string {
	return p.policy.GetAnnotations()
}

func NewKyvernoPolicy(pol kyvernov1.PolicyInterface) KyvernoPolicy {
	return KyvernoPolicy{
		policy: pol,
	}
}

type ValidatingAdmissionPolicy struct {
	policy v1alpha1.ValidatingAdmissionPolicy
}

func (p ValidatingAdmissionPolicy) GetPolicy() interface{} {
	return p.policy
}

func (p ValidatingAdmissionPolicy) GetType() PolicyType {
	return ValidatingAdmissionPolicyType
}

func (p ValidatingAdmissionPolicy) GetName() string {
	return p.policy.GetName()
}

func (p ValidatingAdmissionPolicy) GetNamespace() string {
	return p.policy.GetNamespace()
}

func (p ValidatingAdmissionPolicy) GetAnnotations() map[string]string {
	return p.policy.GetAnnotations()
}

func NewValidatingAdmissionPolicy(pol v1alpha1.ValidatingAdmissionPolicy) ValidatingAdmissionPolicy {
	return ValidatingAdmissionPolicy{
		policy: pol,
	}
}
