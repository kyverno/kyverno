package api

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Everything someone might need to validate a single ValidatingAdmissionPolicy
// against all of its registered bindings.
type ValidatingAdmissionPolicyData struct {
	definition *admissionregistrationv1.ValidatingAdmissionPolicy
	bindings   []admissionregistrationv1.ValidatingAdmissionPolicyBinding
}

func (p *ValidatingAdmissionPolicyData) AddBinding(binding admissionregistrationv1.ValidatingAdmissionPolicyBinding) {
	p.bindings = append(p.bindings, binding)
}

func (p *ValidatingAdmissionPolicyData) GetDefinition() *admissionregistrationv1.ValidatingAdmissionPolicy {
	return p.definition
}

func (p *ValidatingAdmissionPolicyData) GetBindings() []admissionregistrationv1.ValidatingAdmissionPolicyBinding {
	return p.bindings
}

func NewValidatingAdmissionPolicyData(
	policy *admissionregistrationv1.ValidatingAdmissionPolicy,
	bindings ...admissionregistrationv1.ValidatingAdmissionPolicyBinding,
) *ValidatingAdmissionPolicyData {
	return &ValidatingAdmissionPolicyData{
		definition: policy,
		bindings:   bindings,
	}
}

// MutatingPolicyData holds a MAP and its associated MAPBs
type MutatingAdmissionPolicyData struct {
	definition *admissionregistrationv1alpha1.MutatingAdmissionPolicy
	bindings   []admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding
}

// AddBinding appends a MAPB to the policy data
func (m *MutatingAdmissionPolicyData) AddBinding(b admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) {
	m.bindings = append(m.bindings, b)
}

func (p *MutatingAdmissionPolicyData) GetDefinition() *admissionregistrationv1alpha1.MutatingAdmissionPolicy {
	return p.definition
}

func (p *MutatingAdmissionPolicyData) GetBindings() []admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding {
	return p.bindings
}

// NewMutatingPolicyData initializes a MAP wrapper with no bindings
func NewMutatingAdmissionPolicyData(
	policy *admissionregistrationv1alpha1.MutatingAdmissionPolicy,
	bindings ...admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding,
) *MutatingAdmissionPolicyData {
	return &MutatingAdmissionPolicyData{
		definition: policy,
		bindings:   bindings,
	}
}

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
	// AsValidatingAdmissionPolicy returns the validating admission policy with its bindings
	AsValidatingAdmissionPolicy() *ValidatingAdmissionPolicyData
	// AsValidatingPolicy returns the validating policy
	AsValidatingPolicy() *policiesv1alpha1.ValidatingPolicy
	// AsImageValidatingPolicy returns the imageverificationpolicy
	AsImageValidatingPolicy() *policiesv1alpha1.ImageValidatingPolicy
	// AsMutatingAdmissionPolicy returns the mutatingadmission policy
	AsMutatingAdmissionPolicy() *MutatingAdmissionPolicyData
	// AsMutatingPolicy returns the mutating policy
	AsMutatingPolicy() *policiesv1alpha1.MutatingPolicy
	// AsGeneratingPolicy returns the generating policy
	AsGeneratingPolicy() *policiesv1alpha1.GeneratingPolicy
	// AsDeletingPolicy returns the deleting policy
	AsDeletingPolicy() *policiesv1alpha1.DeletingPolicy
}
type genericPolicy struct {
	metav1.Object
	PolicyInterface           kyvernov1.PolicyInterface
	ValidatingAdmissionPolicy *ValidatingAdmissionPolicyData
	MutatingAdmissionPolicy   *MutatingAdmissionPolicyData
	ValidatingPolicy          *policiesv1alpha1.ValidatingPolicy
	ImageValidatingPolicy     *policiesv1alpha1.ImageValidatingPolicy
	MutatingPolicy            *policiesv1alpha1.MutatingPolicy
	GeneratingPolicy          *policiesv1alpha1.GeneratingPolicy
	DeletingPolicy            *policiesv1alpha1.DeletingPolicy
}

func (p *genericPolicy) AsObject() any {
	return p.Object
}

func (p *genericPolicy) AsKyvernoPolicy() kyvernov1.PolicyInterface {
	return p.PolicyInterface
}

func (p *genericPolicy) AsValidatingAdmissionPolicy() *ValidatingAdmissionPolicyData {
	return p.ValidatingAdmissionPolicy
}

func (p *genericPolicy) AsMutatingAdmissionPolicy() *MutatingAdmissionPolicyData {
	return p.MutatingAdmissionPolicy
}

func (p *genericPolicy) AsValidatingPolicy() *policiesv1alpha1.ValidatingPolicy {
	return p.ValidatingPolicy
}

func (p *genericPolicy) AsImageValidatingPolicy() *policiesv1alpha1.ImageValidatingPolicy {
	return p.ImageValidatingPolicy
}

func (p *genericPolicy) AsMutatingPolicy() *policiesv1alpha1.MutatingPolicy {
	return p.MutatingPolicy
}

func (p *genericPolicy) AsGeneratingPolicy() *policiesv1alpha1.GeneratingPolicy {
	return p.GeneratingPolicy
}

func (p *genericPolicy) AsDeletingPolicy() *policiesv1alpha1.DeletingPolicy {
	return p.DeletingPolicy
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
	case p.MutatingPolicy != nil:
		return policiesv1alpha1.GroupVersion.String()
	case p.GeneratingPolicy != nil:
		return policiesv1alpha1.GroupVersion.String()
	case p.DeletingPolicy != nil:
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
	case p.MutatingPolicy != nil:
		return "MutatingPolicy"
	case p.GeneratingPolicy != nil:
		return "GeneratingPolicy"
	case p.DeletingPolicy != nil:
		return "DeletingPolicy"
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
		ValidatingAdmissionPolicy: NewValidatingAdmissionPolicyData(pol),
	}
}

func NewValidatingAdmissionPolicyWithBindings(pol *admissionregistrationv1.ValidatingAdmissionPolicy, bindings ...admissionregistrationv1.ValidatingAdmissionPolicyBinding) GenericPolicy {
	return &genericPolicy{
		Object:                    pol,
		ValidatingAdmissionPolicy: NewValidatingAdmissionPolicyData(pol, bindings...),
	}
}

func NewMutatingAdmissionPolicy(pol *admissionregistrationv1alpha1.MutatingAdmissionPolicy) GenericPolicy {
	return &genericPolicy{
		Object:                  pol,
		MutatingAdmissionPolicy: NewMutatingAdmissionPolicyData(pol),
	}
}

func NewMutatingAdmissionPolicyWithBindings(pol *admissionregistrationv1alpha1.MutatingAdmissionPolicy, bindings ...admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) GenericPolicy {
	return &genericPolicy{
		Object:                  pol,
		MutatingAdmissionPolicy: NewMutatingAdmissionPolicyData(pol, bindings...),
	}
}

func NewValidatingPolicy(pol *policiesv1alpha1.ValidatingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:           pol,
		ValidatingPolicy: pol,
	}
}

func NewImageValidatingPolicy(pol *policiesv1alpha1.ImageValidatingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:                pol,
		ImageValidatingPolicy: pol,
	}
}

func NewMutatingPolicy(pol *policiesv1alpha1.MutatingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:         pol,
		MutatingPolicy: pol,
	}
}

func NewGeneratingPolicy(pol *policiesv1alpha1.GeneratingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:           pol,
		GeneratingPolicy: pol,
	}
}

func NewDeletingPolicy(pol *policiesv1alpha1.DeletingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:         pol,
		DeletingPolicy: pol,
	}
}
