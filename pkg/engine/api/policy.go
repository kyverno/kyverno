package api

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Everything someone might need to validate a single ValidatingAdmissionPolicy
// against all of its registered bindings.
type ValidatingAdmissionPolicyData struct {
	definition *admissionregistrationv1.ValidatingAdmissionPolicy
	bindings   []admissionregistrationv1.ValidatingAdmissionPolicyBinding
	params     []runtime.Object
}

func (p *ValidatingAdmissionPolicyData) AddBinding(binding admissionregistrationv1.ValidatingAdmissionPolicyBinding) {
	p.bindings = append(p.bindings, binding)
}

func (p *ValidatingAdmissionPolicyData) AddParam(param runtime.Object) {
	p.params = append(p.params, param)
}

func (p *ValidatingAdmissionPolicyData) GetDefinition() *admissionregistrationv1.ValidatingAdmissionPolicy {
	return p.definition
}

func (p *ValidatingAdmissionPolicyData) GetBindings() []admissionregistrationv1.ValidatingAdmissionPolicyBinding {
	return p.bindings
}

func (p *ValidatingAdmissionPolicyData) GetParams() []runtime.Object {
	return p.params
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
	definition *admissionregistrationv1beta1.MutatingAdmissionPolicy
	bindings   []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding
	params     []runtime.Object
}

// AddBinding appends a MAPB to the policy data
func (m *MutatingAdmissionPolicyData) AddBinding(b admissionregistrationv1beta1.MutatingAdmissionPolicyBinding) {
	m.bindings = append(m.bindings, b)
}

func (m *MutatingAdmissionPolicyData) AddParam(p runtime.Object) {
	m.params = append(m.params, p)
}

func (p *MutatingAdmissionPolicyData) GetDefinition() *admissionregistrationv1beta1.MutatingAdmissionPolicy {
	return p.definition
}

func (p *MutatingAdmissionPolicyData) GetBindings() []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding {
	return p.bindings
}

func (p *MutatingAdmissionPolicyData) GetParams() []runtime.Object {
	return p.params
}

// NewMutatingPolicyData initializes a MAP wrapper with no bindings
func NewMutatingAdmissionPolicyData(
	policy *admissionregistrationv1beta1.MutatingAdmissionPolicy,
	bindings ...admissionregistrationv1beta1.MutatingAdmissionPolicyBinding,
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
	// AsValidatingPolicyLike returns the validating policy
	AsValidatingPolicyLike() policiesv1beta1.ValidatingPolicyLike
	// AsValidatingPolicy returns the validating policy
	AsValidatingPolicy() *policiesv1beta1.ValidatingPolicy
	// AsNamespacedValidatingPolicy returns the namespaced validating policy
	AsNamespacedValidatingPolicy() *policiesv1beta1.NamespacedValidatingPolicy
	// AsImageValidatingPolicyLike returns the imageverificationpolicy
	AsImageValidatingPolicyLike() policiesv1beta1.ImageValidatingPolicyLike
	// AsImageValidatingPolicy returns the imageverificationpolicy
	AsImageValidatingPolicy() *policiesv1beta1.ImageValidatingPolicy
	// AsNamespacedImageValidatingPolicy returns the namespaced imageverificationpolicy
	AsNamespacedImageValidatingPolicy() *policiesv1beta1.NamespacedImageValidatingPolicy
	// AsMutatingAdmissionPolicy returns the mutatingadmission policy
	AsMutatingAdmissionPolicy() *MutatingAdmissionPolicyData
	// AsMutatingPolicyLike returns the mutating policy
	AsMutatingPolicyLike() policiesv1beta1.MutatingPolicyLike
	// AsMutatingPolicy returns the mutating policy
	AsMutatingPolicy() *policiesv1beta1.MutatingPolicy
	// AsNamespacedMutatingPolicy returns the namespaced mutating policy
	AsNamespacedMutatingPolicy() *policiesv1beta1.NamespacedMutatingPolicy
	// AsGeneratingPolicyLike returns the generating policy
	AsGeneratingPolicyLike() policiesv1beta1.GeneratingPolicyLike
	// AsGeneratingPolicy returns the generating policy
	AsGeneratingPolicy() *policiesv1beta1.GeneratingPolicy
	// AsNamespacedGeneratingPolicy returns the namespaced generating policy
	AsNamespacedGeneratingPolicy() *policiesv1beta1.NamespacedGeneratingPolicy
	// AsDeletingPolicy returns the deleting policy
	AsDeletingPolicy() policiesv1beta1.DeletingPolicyLike
}
type genericPolicy struct {
	metav1.Object
	PolicyInterface                 kyvernov1.PolicyInterface
	ValidatingAdmissionPolicy       *ValidatingAdmissionPolicyData
	MutatingAdmissionPolicy         *MutatingAdmissionPolicyData
	ValidatingPolicy                *policiesv1beta1.ValidatingPolicy
	NamespacedValidatingPolicy      *policiesv1beta1.NamespacedValidatingPolicy
	ImageValidatingPolicy           *policiesv1beta1.ImageValidatingPolicy
	NamespacedImageValidatingPolicy *policiesv1beta1.NamespacedImageValidatingPolicy
	MutatingPolicy                  *policiesv1beta1.MutatingPolicy
	NamespacedMutatingPolicy        *policiesv1beta1.NamespacedMutatingPolicy
	GeneratingPolicy                *policiesv1beta1.GeneratingPolicy
	NamespacedGeneratingPolicy      *policiesv1beta1.NamespacedGeneratingPolicy
	DeletingPolicy                  policiesv1beta1.DeletingPolicyLike
	// originalAPIVersion tracks the original API version for converted policies
	originalAPIVersion string
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

func (p *genericPolicy) AsValidatingPolicyLike() policiesv1beta1.ValidatingPolicyLike {
	if v := p.AsValidatingPolicy(); v != nil {
		return v
	}
	if v := p.AsNamespacedValidatingPolicy(); v != nil {
		return v
	}
	return nil
}

func (p *genericPolicy) AsValidatingPolicy() *policiesv1beta1.ValidatingPolicy {
	return p.ValidatingPolicy
}

func (p *genericPolicy) AsNamespacedValidatingPolicy() *policiesv1beta1.NamespacedValidatingPolicy {
	return p.NamespacedValidatingPolicy
}

func (p *genericPolicy) AsImageValidatingPolicyLike() policiesv1beta1.ImageValidatingPolicyLike {
	if v := p.AsImageValidatingPolicy(); v != nil {
		return v
	}
	if v := p.AsNamespacedImageValidatingPolicy(); v != nil {
		return v
	}
	return nil
}

func (p *genericPolicy) AsImageValidatingPolicy() *policiesv1beta1.ImageValidatingPolicy {
	return p.ImageValidatingPolicy
}

func (p *genericPolicy) AsNamespacedImageValidatingPolicy() *policiesv1beta1.NamespacedImageValidatingPolicy {
	return p.NamespacedImageValidatingPolicy
}

func (p *genericPolicy) AsMutatingPolicyLike() policiesv1beta1.MutatingPolicyLike {
	if m := p.AsMutatingPolicy(); m != nil {
		return m
	}
	if m := p.AsNamespacedMutatingPolicy(); m != nil {
		return m
	}
	return nil
}

func (p *genericPolicy) AsMutatingPolicy() *policiesv1beta1.MutatingPolicy {
	return p.MutatingPolicy
}

func (p *genericPolicy) AsNamespacedMutatingPolicy() *policiesv1beta1.NamespacedMutatingPolicy {
	return p.NamespacedMutatingPolicy
}

func (p *genericPolicy) AsGeneratingPolicyLike() policiesv1beta1.GeneratingPolicyLike {
	if g := p.AsGeneratingPolicy(); g != nil {
		return g
	}
	if g := p.AsNamespacedGeneratingPolicy(); g != nil {
		return g
	}
	return nil
}

func (p *genericPolicy) AsGeneratingPolicy() *policiesv1beta1.GeneratingPolicy {
	return p.GeneratingPolicy
}

func (p *genericPolicy) AsNamespacedGeneratingPolicy() *policiesv1beta1.NamespacedGeneratingPolicy {
	return p.NamespacedGeneratingPolicy
}

func (p *genericPolicy) AsDeletingPolicy() policiesv1beta1.DeletingPolicyLike {
	return p.DeletingPolicy
}

func (p *genericPolicy) GetAPIVersion() string {
	switch {
	case p.PolicyInterface != nil:
		return kyvernov1.GroupVersion.String()
	case p.ValidatingAdmissionPolicy != nil:
		return admissionregistrationv1.SchemeGroupVersion.String()
	case p.MutatingAdmissionPolicy != nil:
		if p.originalAPIVersion != "" {
			return p.originalAPIVersion
		}
		return admissionregistrationv1beta1.SchemeGroupVersion.String()
	case p.ValidatingPolicy != nil:
		if apiVersion := p.ValidatingPolicy.APIVersion; apiVersion != "" {
			return apiVersion
		}
		return policiesv1beta1.GroupVersion.String()
	case p.NamespacedValidatingPolicy != nil:
		if apiVersion := p.NamespacedValidatingPolicy.APIVersion; apiVersion != "" {
			return apiVersion
		}
		return policiesv1beta1.GroupVersion.String()
	case p.ImageValidatingPolicy != nil:
		return policiesv1beta1.GroupVersion.String()
	case p.NamespacedImageValidatingPolicy != nil:
		return policiesv1beta1.GroupVersion.String()
	case p.MutatingPolicy != nil:
		if apiVersion := p.MutatingPolicy.APIVersion; apiVersion != "" {
			return apiVersion
		}
		return policiesv1beta1.GroupVersion.String()
	case p.NamespacedMutatingPolicy != nil:
		if apiVersion := p.NamespacedMutatingPolicy.APIVersion; apiVersion != "" {
			return apiVersion
		}
		return policiesv1beta1.GroupVersion.String()
	case p.GeneratingPolicy != nil:
		return policiesv1beta1.GroupVersion.String()
	case p.DeletingPolicy != nil:
		return policiesv1beta1.GroupVersion.String()
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
		return p.ValidatingPolicy.GetKind()
	case p.NamespacedValidatingPolicy != nil:
		return p.NamespacedValidatingPolicy.GetKind()
	case p.ImageValidatingPolicy != nil:
		return p.ImageValidatingPolicy.GetKind()
	case p.NamespacedImageValidatingPolicy != nil:
		return p.NamespacedImageValidatingPolicy.GetKind()
	case p.MutatingPolicy != nil:
		return p.MutatingPolicy.GetKind()
	case p.NamespacedMutatingPolicy != nil:
		return p.NamespacedMutatingPolicy.GetKind()
	case p.GeneratingPolicy != nil:
		return "GeneratingPolicy"
	case p.DeletingPolicy != nil:
		return p.DeletingPolicy.GetKind()
	}
	return ""
}

func (p *genericPolicy) IsNamespaced() bool {
	switch {
	case p.PolicyInterface != nil:
		return p.PolicyInterface.IsNamespaced()
	case p.NamespacedValidatingPolicy != nil:
		return true
	case p.NamespacedImageValidatingPolicy != nil:
		return true
	case p.NamespacedMutatingPolicy != nil:
		return true
	case p.NamespacedGeneratingPolicy != nil:
		return true
	case p.DeletingPolicy != nil:
		return p.DeletingPolicy.GetNamespace() != ""
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

func NewMutatingAdmissionPolicy(pol *admissionregistrationv1beta1.MutatingAdmissionPolicy) GenericPolicy {
	return &genericPolicy{
		Object:                  pol,
		MutatingAdmissionPolicy: NewMutatingAdmissionPolicyData(pol),
	}
}

func NewMutatingAdmissionPolicyAlpha(pol *admissionregistrationv1alpha1.MutatingAdmissionPolicy) GenericPolicy {
	v1beta1Pol := &admissionregistrationv1beta1.MutatingAdmissionPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: admissionregistrationv1beta1.SchemeGroupVersion.String(),
			Kind:       "MutatingAdmissionPolicy",
		},
		ObjectMeta: pol.ObjectMeta,
		Spec: admissionregistrationv1beta1.MutatingAdmissionPolicySpec{
			ParamKind:          convertParamKind(pol.Spec.ParamKind),
			MatchConstraints:   convertMatchConstraints(pol.Spec.MatchConstraints),
			Variables:          convertVariables(pol.Spec.Variables),
			Mutations:          convertMutations(pol.Spec.Mutations),
			FailurePolicy:      (*admissionregistrationv1beta1.FailurePolicyType)(pol.Spec.FailurePolicy),
			MatchConditions:    convertMatchConditions(pol.Spec.MatchConditions),
			ReinvocationPolicy: pol.Spec.ReinvocationPolicy,
		},
	}
	return &genericPolicy{
		Object:                  v1beta1Pol,
		MutatingAdmissionPolicy: NewMutatingAdmissionPolicyData(v1beta1Pol),
		originalAPIVersion:      admissionregistrationv1alpha1.SchemeGroupVersion.String(),
	}
}

func convertParamKind(paramKind *admissionregistrationv1alpha1.ParamKind) *admissionregistrationv1beta1.ParamKind {
	if paramKind == nil {
		return nil
	}
	return &admissionregistrationv1beta1.ParamKind{
		APIVersion: paramKind.APIVersion,
		Kind:       paramKind.Kind,
	}
}

func convertMatchConstraints(matchConstraints *admissionregistrationv1alpha1.MatchResources) *admissionregistrationv1beta1.MatchResources {
	if matchConstraints == nil {
		return nil
	}
	return &admissionregistrationv1beta1.MatchResources{
		ResourceRules:        convertResourceRules(matchConstraints.ResourceRules),
		ExcludeResourceRules: convertResourceRules(matchConstraints.ExcludeResourceRules),
		MatchPolicy:          (*admissionregistrationv1beta1.MatchPolicyType)(matchConstraints.MatchPolicy),
	}
}

func convertResourceRules(rules []admissionregistrationv1alpha1.NamedRuleWithOperations) []admissionregistrationv1beta1.NamedRuleWithOperations {
	result := make([]admissionregistrationv1beta1.NamedRuleWithOperations, len(rules))
	for i, r := range rules {
		result[i] = admissionregistrationv1beta1.NamedRuleWithOperations{
			ResourceNames: r.ResourceNames,
			RuleWithOperations: admissionregistrationv1beta1.RuleWithOperations{
				Operations: convertOperations(r.Operations),
				Rule: admissionregistrationv1beta1.Rule{
					APIGroups:   r.APIGroups,
					APIVersions: r.APIVersions,
					Resources:   r.Resources,
					Scope:       r.Scope,
				},
			},
		}
	}
	return result
}

func convertOperations(ops []admissionregistrationv1alpha1.OperationType) []admissionregistrationv1beta1.OperationType {
	result := make([]admissionregistrationv1beta1.OperationType, len(ops))
	copy(result, ops)
	return result
}

func convertVariables(vars []admissionregistrationv1alpha1.Variable) []admissionregistrationv1beta1.Variable {
	result := make([]admissionregistrationv1beta1.Variable, len(vars))
	for i, v := range vars {
		result[i] = admissionregistrationv1beta1.Variable{
			Name:       v.Name,
			Expression: v.Expression,
		}
	}
	return result
}

func convertMutations(mutations []admissionregistrationv1alpha1.Mutation) []admissionregistrationv1beta1.Mutation {
	result := make([]admissionregistrationv1beta1.Mutation, len(mutations))
	for i, m := range mutations {
		result[i] = admissionregistrationv1beta1.Mutation{
			PatchType:          admissionregistrationv1beta1.PatchType(m.PatchType),
			ApplyConfiguration: convertApplyConfiguration(m.ApplyConfiguration),
			JSONPatch:          convertJSONPatch(m.JSONPatch),
		}
	}
	return result
}

func convertApplyConfiguration(applyConfig *admissionregistrationv1alpha1.ApplyConfiguration) *admissionregistrationv1beta1.ApplyConfiguration {
	if applyConfig == nil {
		return nil
	}
	return &admissionregistrationv1beta1.ApplyConfiguration{
		Expression: applyConfig.Expression,
	}
}

func convertJSONPatch(jsonPatch *admissionregistrationv1alpha1.JSONPatch) *admissionregistrationv1beta1.JSONPatch {
	if jsonPatch == nil {
		return nil
	}
	return &admissionregistrationv1beta1.JSONPatch{
		Expression: jsonPatch.Expression,
	}
}

func convertMatchConditions(conditions []admissionregistrationv1alpha1.MatchCondition) []admissionregistrationv1beta1.MatchCondition {
	result := make([]admissionregistrationv1beta1.MatchCondition, len(conditions))
	for i, c := range conditions {
		result[i] = admissionregistrationv1beta1.MatchCondition{
			Name:       c.Name,
			Expression: c.Expression,
		}
	}
	return result
}

func ConvertMutatingAdmissionPolicyBindingsAlpha(bindings []admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding {
	result := make([]admissionregistrationv1beta1.MutatingAdmissionPolicyBinding, len(bindings))
	for i, binding := range bindings {
		result[i] = admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
			TypeMeta: metav1.TypeMeta{
				APIVersion: admissionregistrationv1beta1.SchemeGroupVersion.String(),
				Kind:       "MutatingAdmissionPolicyBinding",
			},
			ObjectMeta: binding.ObjectMeta,
			Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
				PolicyName:     binding.Spec.PolicyName,
				ParamRef:       convertParamRef(binding.Spec.ParamRef),
				MatchResources: convertMatchResourcesForBinding(binding.Spec.MatchResources),
			},
		}
	}
	return result
}

func convertParamRef(paramRef *admissionregistrationv1alpha1.ParamRef) *admissionregistrationv1beta1.ParamRef {
	if paramRef == nil {
		return nil
	}
	return &admissionregistrationv1beta1.ParamRef{
		Name:                    paramRef.Name,
		Namespace:               paramRef.Namespace,
		Selector:                paramRef.Selector,
		ParameterNotFoundAction: (*admissionregistrationv1beta1.ParameterNotFoundActionType)(paramRef.ParameterNotFoundAction),
	}
}

func convertMatchResourcesForBinding(matchResources *admissionregistrationv1alpha1.MatchResources) *admissionregistrationv1beta1.MatchResources {
	if matchResources == nil {
		return nil
	}
	return &admissionregistrationv1beta1.MatchResources{
		ResourceRules:        convertResourceRules(matchResources.ResourceRules),
		ExcludeResourceRules: convertResourceRules(matchResources.ExcludeResourceRules),
		MatchPolicy:          (*admissionregistrationv1beta1.MatchPolicyType)(matchResources.MatchPolicy),
	}
}

func NewMutatingAdmissionPolicyWithBindings(pol *admissionregistrationv1beta1.MutatingAdmissionPolicy, bindings ...admissionregistrationv1beta1.MutatingAdmissionPolicyBinding) GenericPolicy {
	return &genericPolicy{
		Object:                  pol,
		MutatingAdmissionPolicy: NewMutatingAdmissionPolicyData(pol, bindings...),
	}
}

func NewValidatingPolicy(pol *policiesv1beta1.ValidatingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:           pol,
		ValidatingPolicy: pol,
	}
}

func NewNamespacedValidatingPolicy(pol *policiesv1beta1.NamespacedValidatingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:                     pol,
		NamespacedValidatingPolicy: pol,
	}
}

func NewValidatingPolicyFromLike(pol policiesv1beta1.ValidatingPolicyLike) GenericPolicy {
	switch typed := pol.(type) {
	case *policiesv1beta1.ValidatingPolicy:
		return NewValidatingPolicy(typed)
	case *policiesv1beta1.NamespacedValidatingPolicy:
		return NewNamespacedValidatingPolicy(typed)
	default:
		return nil
	}
}

func NewImageValidatingPolicy(pol *policiesv1beta1.ImageValidatingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:                pol,
		ImageValidatingPolicy: pol,
	}
}

func NewNamespacedImageValidatingPolicy(pol *policiesv1beta1.NamespacedImageValidatingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:                          pol,
		NamespacedImageValidatingPolicy: pol,
	}
}

func NewImageValidatingPolicyFromLike(pol policiesv1beta1.ImageValidatingPolicyLike) GenericPolicy {
	switch typed := pol.(type) {
	case *policiesv1beta1.ImageValidatingPolicy:
		return NewImageValidatingPolicy(typed)
	case *policiesv1beta1.NamespacedImageValidatingPolicy:
		return NewNamespacedImageValidatingPolicy(typed)
	default:
		return nil
	}
}

func NewMutatingPolicy(pol *policiesv1beta1.MutatingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:         pol,
		MutatingPolicy: pol,
	}
}

func NewNamespacedMutatingPolicy(pol *policiesv1beta1.NamespacedMutatingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:                   pol,
		NamespacedMutatingPolicy: pol,
	}
}

func NewMutatingPolicyFromLike(pol policiesv1beta1.MutatingPolicyLike) GenericPolicy {
	switch typed := pol.(type) {
	case *policiesv1beta1.MutatingPolicy:
		return NewMutatingPolicy(typed)
	case *policiesv1beta1.NamespacedMutatingPolicy:
		return NewNamespacedMutatingPolicy(typed)
	default:
		return nil
	}
}

func NewGeneratingPolicy(pol *policiesv1beta1.GeneratingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:           pol,
		GeneratingPolicy: pol,
	}
}

func NewNamespacedGeneratingPolicy(pol *policiesv1beta1.NamespacedGeneratingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:                     pol,
		NamespacedGeneratingPolicy: pol,
	}
}

func NewGeneratingPolicyFromLike(pol policiesv1beta1.GeneratingPolicyLike) GenericPolicy {
	switch typed := pol.(type) {
	case *policiesv1beta1.GeneratingPolicy:
		return NewGeneratingPolicy(typed)
	case *policiesv1beta1.NamespacedGeneratingPolicy:
		return NewNamespacedGeneratingPolicy(typed)
	default:
		return nil
	}
}

func NewDeletingPolicy(pol *policiesv1beta1.DeletingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:         pol,
		DeletingPolicy: pol,
	}
}

func NewNamespacedDeletingPolicy(pol *policiesv1beta1.NamespacedDeletingPolicy) GenericPolicy {
	return &genericPolicy{
		Object:         pol,
		DeletingPolicy: pol,
	}
}

func NewDeletingPolicyFromLike(pol policiesv1beta1.DeletingPolicyLike) GenericPolicy {
	switch typed := pol.(type) {
	case *policiesv1beta1.DeletingPolicy:
		return NewDeletingPolicy(typed)
	case *policiesv1beta1.NamespacedDeletingPolicy:
		return NewNamespacedDeletingPolicy(typed)
	default:
		return nil
	}
}
