package api

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PolicyType represents the type of a policy
type PolicyType string

const (
	// KyvernoPolicy type for kyverno policies
	KyvernoPolicyType PolicyType = "KyvernoPolicy"
	// ValidatingAdmissionPolicy for validating admission policies
	ValidatingAdmissionPolicyType PolicyType = "ValidatingAdmissionPolicy"
)

// GenericPolicy abstracts the policy type (Kyverno policy vs Validating admission policy)
// It is intended to be used in EngineResponse
type GenericPolicy interface {
	// AsKyvernoPolicy returns the kyverno policy
	AsKyvernoPolicy() kyvernov1.PolicyInterface
	// AsValidatingAdmissionPolicy returns the validating admission policy
	AsValidatingAdmissionPolicy() *v1alpha1.ValidatingAdmissionPolicy
	// GetType returns policy type
	GetType() PolicyType
	// GetAPIVersion returns policy API version
	GetAPIVersion() string
	// GetName returns policy name
	GetName() string
	// GetNamespace returns policy namespace
	GetNamespace() string
	// GetKind returns policy kind
	GetKind() string
	// GetResourceVersion returns policy resource version
	GetResourceVersion() string
	// GetAnnotations returns policy annotations
	GetAnnotations() map[string]string
	// IsNamespaced indicates if the policy is namespace scoped
	IsNamespaced() bool
	// MetaObject provides an object compatible with metav1.Object
	MetaObject() metav1.Object
}

type KyvernoPolicy struct {
	policy kyvernov1.PolicyInterface
}

func (p *KyvernoPolicy) AsKyvernoPolicy() kyvernov1.PolicyInterface {
	return p.policy
}

func (p *KyvernoPolicy) AsValidatingAdmissionPolicy() *v1alpha1.ValidatingAdmissionPolicy {
	return nil
}

func (p *KyvernoPolicy) GetType() PolicyType {
	return KyvernoPolicyType
}

func (p *KyvernoPolicy) GetAPIVersion() string {
	return "kyverno.io/v1"
}

func (p *KyvernoPolicy) GetName() string {
	return p.policy.GetName()
}

func (p *KyvernoPolicy) GetNamespace() string {
	return p.policy.GetNamespace()
}

func (p *KyvernoPolicy) GetKind() string {
	return p.policy.GetKind()
}

func (p *KyvernoPolicy) GetResourceVersion() string {
	return p.policy.GetResourceVersion()
}

func (p *KyvernoPolicy) GetAnnotations() map[string]string {
	return p.policy.GetAnnotations()
}

func (p *KyvernoPolicy) IsNamespaced() bool {
	return p.policy.IsNamespaced()
}

func (p *KyvernoPolicy) MetaObject() metav1.Object {
	return p.policy
}

func NewKyvernoPolicy(pol kyvernov1.PolicyInterface) GenericPolicy {
	return &KyvernoPolicy{
		policy: pol,
	}
}

type ValidatingAdmissionPolicy struct {
	policy v1alpha1.ValidatingAdmissionPolicy
}

func (p *ValidatingAdmissionPolicy) AsKyvernoPolicy() kyvernov1.PolicyInterface {
	return nil
}

func (p *ValidatingAdmissionPolicy) AsValidatingAdmissionPolicy() *v1alpha1.ValidatingAdmissionPolicy {
	return &p.policy
}

func (p *ValidatingAdmissionPolicy) GetType() PolicyType {
	return ValidatingAdmissionPolicyType
}

func (p *ValidatingAdmissionPolicy) GetAPIVersion() string {
	return "admissionregistration.k8s.io/v1alpha1"
}

func (p *ValidatingAdmissionPolicy) GetName() string {
	return p.policy.GetName()
}

func (p *ValidatingAdmissionPolicy) GetNamespace() string {
	return p.policy.GetNamespace()
}

func (p *ValidatingAdmissionPolicy) GetKind() string {
	return "ValidatingAdmissionPolicy"
}

func (p *ValidatingAdmissionPolicy) GetResourceVersion() string {
	return p.policy.GetResourceVersion()
}

func (p *ValidatingAdmissionPolicy) GetAnnotations() map[string]string {
	return p.policy.GetAnnotations()
}

func (p *ValidatingAdmissionPolicy) IsNamespaced() bool {
	return false
}

func (p *ValidatingAdmissionPolicy) MetaObject() metav1.Object {
	return &p.policy
}

func NewValidatingAdmissionPolicy(pol v1alpha1.ValidatingAdmissionPolicy) GenericPolicy {
	return &ValidatingAdmissionPolicy{
		policy: pol,
	}
}
