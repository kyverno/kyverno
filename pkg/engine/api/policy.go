package api

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PolicyType represents the type of a policy
type PolicyType string

const (
	// KyvernoPolicy type for kyverno policies
	KyvernoPolicyType PolicyType = "KyvernoPolicy"
	// ValidatingAdmissionPolicy for Kubernetes ValidatingAdmission policies
	ValidatingAdmissionPolicyType PolicyType = "ValidatingAdmissionPolicy"
	// MutatingAdmissionPolicy for Kubernetes MutatingAdmissionPolicies
	MutatingAdmissionPolicyType PolicyType = "MutatingAdmissionPolicy"
	// ValidatingPolicy type for validating policies
	ValidatingPolicyType PolicyType = "ValidatingPolicy"
)

// GenericPolicy abstracts the policy type (Kyverno policy vs Validating admission policy)
// It is intended to be used in EngineResponse
type GenericPolicy interface {
	// AsKyvernoPolicy returns the kyverno policy
	AsKyvernoPolicy() kyvernov1.PolicyInterface
	// AsValidatingAdmissionPolicy returns the validating admission policy
	AsValidatingAdmissionPolicy() *admissionregistrationv1beta1.ValidatingAdmissionPolicy
	// GetType returns policy type
	GetType() PolicyType
	// GetAPIVersion returns policy API version
	GetAPIVersion() string
	// GetKind returns policy kind
	GetKind() string
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

func (p *KyvernoPolicy) AsValidatingAdmissionPolicy() *admissionregistrationv1beta1.ValidatingAdmissionPolicy {
	return nil
}

func (p *KyvernoPolicy) GetType() PolicyType {
	return KyvernoPolicyType
}

func (p *KyvernoPolicy) GetAPIVersion() string {
	return "kyverno.io/v1"
}

func (p *KyvernoPolicy) GetKind() string {
	return p.policy.GetKind()
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
	policy admissionregistrationv1beta1.ValidatingAdmissionPolicy
}

func (p *ValidatingAdmissionPolicy) AsKyvernoPolicy() kyvernov1.PolicyInterface {
	return nil
}

func (p *ValidatingAdmissionPolicy) AsValidatingAdmissionPolicy() *admissionregistrationv1beta1.ValidatingAdmissionPolicy {
	return &p.policy
}

func (p *ValidatingAdmissionPolicy) GetType() PolicyType {
	return ValidatingAdmissionPolicyType
}

func (p *ValidatingAdmissionPolicy) GetAPIVersion() string {
	return "admissionregistration.k8s.io/v1beta1"
}

func (p *ValidatingAdmissionPolicy) GetKind() string {
	return "ValidatingAdmissionPolicy"
}

func (p *ValidatingAdmissionPolicy) IsNamespaced() bool {
	return false
}

func (p *ValidatingAdmissionPolicy) MetaObject() metav1.Object {
	return &p.policy
}

func NewValidatingAdmissionPolicy(pol admissionregistrationv1beta1.ValidatingAdmissionPolicy) GenericPolicy {
	return &ValidatingAdmissionPolicy{
		policy: pol,
	}
}

type MutatingAdmissionPolicy struct {
	policy admissionregistrationv1alpha1.MutatingAdmissionPolicy
}

func (p *MutatingAdmissionPolicy) AsKyvernoPolicy() kyvernov1.PolicyInterface {
	return nil
}

func (p *MutatingAdmissionPolicy) AsValidatingAdmissionPolicy() *admissionregistrationv1beta1.ValidatingAdmissionPolicy {
	return nil
}

func (p *MutatingAdmissionPolicy) GetType() PolicyType {
	return MutatingAdmissionPolicyType
}

func (p *MutatingAdmissionPolicy) GetAPIVersion() string {
	return "admissionregistration.k8s.io/v1alpha1"
}

func (p *MutatingAdmissionPolicy) GetKind() string {
	return "MutatingAdmissionPolicy"
}

func (p *MutatingAdmissionPolicy) IsNamespaced() bool {
	return false
}

func (p *MutatingAdmissionPolicy) MetaObject() metav1.Object {
	return &p.policy
}

func NewMutatingAdmissionPolicy(pol admissionregistrationv1alpha1.MutatingAdmissionPolicy) GenericPolicy {
	return &MutatingAdmissionPolicy{
		policy: pol,
	}
}

type ValidatingPolicy struct {
	policy kyvernov2alpha1.ValidatingPolicy
}

func (p *ValidatingPolicy) AsKyvernoPolicy() kyvernov1.PolicyInterface {
	return nil
}

func (p *ValidatingPolicy) AsValidatingAdmissionPolicy() *admissionregistrationv1beta1.ValidatingAdmissionPolicy {
	return nil
}

func (p *ValidatingPolicy) GetType() PolicyType {
	return ValidatingPolicyType
}

func (p *ValidatingPolicy) GetAPIVersion() string {
	return kyvernov2alpha1.GroupVersion.String()
}

func (p *ValidatingPolicy) GetKind() string {
	return "ValidatingPolicy"
}

func (p *ValidatingPolicy) IsNamespaced() bool {
	return false
}

func (p *ValidatingPolicy) MetaObject() metav1.Object {
	return &p.policy
}

func NewValidatingPolicy(pol kyvernov2alpha1.ValidatingPolicy) GenericPolicy {
	return &ValidatingPolicy{
		policy: pol,
	}
}
