package admissionpolicy

import (
	"context"
	"fmt"
	"strings"
	"time"

	celutils "github.com/kyverno/kyverno/pkg/cel/utils"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/generic"
	k8smatching "k8s.io/apiserver/pkg/admission/plugin/policy/matching"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

func MutateResource(
	policy admissionregistrationv1alpha1.MutatingAdmissionPolicy,
	binding *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding,
	resource unstructured.Unstructured,
) (engineapi.EngineResponse, error) {
	startTime := time.Now()

	engineResponse := engineapi.NewEngineResponse(resource, engineapi.NewMutatingAdmissionPolicy(&policy), nil)
	policyResp := engineapi.NewPolicyResponse()

	gvk := resource.GroupVersionKind()
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: strings.ToLower(gvk.Kind) + "s",
	}

	var namespace *corev1.Namespace
	namespaceName := resource.GetNamespace()
	// Special case, the namespace object has the namespace of itself.
	// unset it if the incoming object is a namespace
	if gvk.Kind == "Namespace" && gvk.Version == "v1" && gvk.Group == "" {
		namespaceName = ""
	}

	if namespaceName != "" {
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		}
	}

	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, resource.GroupVersionKind(), resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
	versionedAttributes, _ := admission.NewVersionedAttributes(a, a.GetKind(), nil)
	o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())

	matchConditions := make([]admissionregistrationv1.MatchCondition, 0, len(policy.Spec.MatchConditions))
	for _, m := range policy.Spec.MatchConditions {
		matchConditions = append(matchConditions, admissionregistrationv1.MatchCondition(m))
	}
	variables := make([]admissionregistrationv1.Variable, 0, len(policy.Spec.Variables))
	for _, v := range policy.Spec.Variables {
		variables = append(variables, admissionregistrationv1.Variable(v))
	}

	// create compiler
	compiler, err := NewCompiler(matchConditions, variables)
	if err != nil {
		return engineResponse, err
	}
	compiler.WithMutations(policy.Spec.Mutations)

	optionalVars := cel.OptionalVariableDeclarations{
		HasParams:     false,
		HasAuthorizer: false,
		HasPatchTypes: true,
		StrictCost:    true,
	}
	// compile variables
	compiler.CompileVariables(optionalVars)

	var failPolicy admissionregistrationv1.FailurePolicyType
	if policy.Spec.FailurePolicy == nil {
		failPolicy = admissionregistrationv1.Fail
	} else {
		failPolicy = admissionregistrationv1.FailurePolicyType(*policy.Spec.FailurePolicy)
	}

	// compile matchers
	matcher := matchconditions.NewMatcher(compiler.CompileMatchConditions(optionalVars), &failPolicy, "policy", "mutate", policy.Name)
	if matcher != nil {
		matchResults := matcher.Match(context.TODO(), versionedAttributes, nil, nil)

		// if preconditions are not met, then skip mutations
		if !matchResults.Matches {
			ruleResp := engineapi.RuleSkip(policy.GetName(), engineapi.Mutation, "match conditions not met", nil)
			policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
			return engineResponse.WithPolicyResponse(policyResp), nil
		}
	}
	// compile mutations
	patchers := compiler.CompileMutations(optionalVars)
	if len(patchers) == 0 {
		ruleResp := engineapi.RuleSkip(policy.GetName(), engineapi.Mutation, "mutation returned no patchers", nil)
		policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
		return engineResponse.WithPolicyResponse(policyResp), nil
	}
	// apply mutations
	for _, patcher := range patchers {
		patchRequest := patch.Request{
			MatchedResource:     gvr,
			VersionedAttributes: versionedAttributes,
			ObjectInterfaces:    o,
			OptionalVariables:   cel.OptionalVariableBindings{VersionedParams: nil, Authorizer: nil},
			Namespace:           namespace,
			TypeConverter:       managedfields.NewDeducedTypeConverter(),
		}
		original := versionedAttributes.VersionedObject

		newVersionedObject, err := patcher.Patch(context.TODO(), patchRequest, celconfig.RuntimeCELCostBudget)
		if err != nil {
			ruleResp := engineapi.RuleError(policy.GetName(), engineapi.Mutation, err.Error(), nil, nil)
			policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
			return engineResponse.WithPolicyResponse(policyResp), nil
		}

		// check if mutation actually changed anything
		if equality.Semantic.DeepEqual(original, newVersionedObject) {
			ruleResp := engineapi.RuleSkip(policy.GetName(), engineapi.Mutation, "mutation had no effect", nil)
			policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
			return engineResponse.WithPolicyResponse(policyResp), nil
		}

		versionedAttributes.VersionedObject = newVersionedObject

	}

	patchedResource, err := celutils.ConvertObjectToUnstructured(versionedAttributes.VersionedObject)
	if err != nil {
		ruleResp := engineapi.RuleError(policy.GetName(), engineapi.Mutation, err.Error(), nil, nil)
		policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
		return engineResponse.WithPolicyResponse(policyResp), nil
	}

	ruleResp := engineapi.RulePass(policy.GetName(), engineapi.Mutation, "", nil)
	if binding != nil {
		ruleResp = ruleResp.WithMutatingBinding(binding)
	}
	policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
	engineResponse = engineResponse.
		WithPatchedResource(*patchedResource).
		WithPolicyResponse(policyResp)

	return engineResponse, nil
}

// MutatingPolicyData holds a MAP and its associated MAPBs
type MutatingPolicyData struct {
	definition admissionregistrationv1alpha1.MutatingAdmissionPolicy
	bindings   []admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding
}

// NewMutatingPolicyData initializes a MAP wrapper with no bindings
func NewMutatingPolicyData(p admissionregistrationv1alpha1.MutatingAdmissionPolicy) MutatingPolicyData {
	return MutatingPolicyData{definition: p}
}

// AddBinding appends a MAPB to the policy data
func (m *MutatingPolicyData) AddBinding(b admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) {
	m.bindings = append(m.bindings, b)
}

func toGVR(gvk schema.GroupVersionKind) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: strings.ToLower(gvk.Kind) + "s",
	}
}

func Mutate(
	data MutatingPolicyData,
	resource unstructured.Unstructured,
	client dclient.Interface,
	namespaceSelectorMap map[string]map[string]string,
) (engineapi.EngineResponse, error) {
	var emptyResp engineapi.EngineResponse

	resPath := fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
	policy := data.definition
	bindings := data.bindings

	gvk := resource.GroupVersionKind()
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: strings.ToLower(gvk.Kind) + "s",
	}

	a := admission.NewAttributesRecord(
		resource.DeepCopyObject(), nil,
		gvk,
		resource.GetNamespace(), resource.GetName(),
		gvr, "", admission.Create, nil, false, nil,
	)

	// no bindings
	if len(bindings) == 0 {
		//admissionregistrationv1.MatchResources(*policy.Spec.MatchConstraints)
		isMatch, err := matches(a, namespaceSelectorMap, convertMatchResources(*policy.Spec.MatchConstraints))

		if err != nil {
			return emptyResp, err
		}
		if !isMatch {
			return emptyResp, nil
		}

		logger.V(3).Info("mutate resource %s against policy %s", resPath, policy.GetName())
		return MutateResource(policy, nil, resource)
	}

	// bindings exist
	if client != nil {
		nsLister := NewCustomNamespaceLister(client)
		matcher := generic.NewPolicyMatcher(k8smatching.NewMatcher(nsLister, client.GetKubeClient()))

		serverGVR, err := client.Discovery().GetGVRFromGVK(gvk)
		if err != nil {
			return emptyResp, err
		}

		a = admission.NewAttributesRecord(
			resource.DeepCopyObject(), nil,
			gvk,
			resource.GetNamespace(), resource.GetName(),
			serverGVR, "", admission.Create, nil, false, nil,
		)

		o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())

		// match policy
		isPolicyMatch, _, _, err := matcher.DefinitionMatches(a, o, NewMutatingAdmissionPolicyAccessor(&policy))
		if err != nil {
			return emptyResp, err
		}
		if !isPolicyMatch {
			return emptyResp, nil
		}

		// match bindings
		for i, binding := range bindings {
			isBindingMatch, err := matcher.BindingMatches(a, o, NewMutatingAdmissionPolicyBindingAccessor(&binding))
			if err != nil {
				return emptyResp, err
			}
			if !isBindingMatch {
				continue
			}

			logger.V(3).Info("mutate resource %s against policy %s with binding %s", resPath, policy.GetName(), binding.GetName())
			return MutateResource(policy, &bindings[i], resource)
		}

		return emptyResp, nil
	} else {
		// client == nil → offline matching
		isPolicyMatch, err := matches(a, namespaceSelectorMap, convertMatchResources(*policy.Spec.MatchConstraints))
		if err != nil {
			return emptyResp, err
		}
		if !isPolicyMatch {
			return emptyResp, nil
		}

		for i, binding := range bindings {
			isBindingMatch, err := matches(a, namespaceSelectorMap, convertMatchResources(*binding.Spec.MatchResources))
			if err != nil {
				return emptyResp, err
			}
			if !isBindingMatch {
				continue
			}

			logger.V(3).Info("mutate resource %s against policy %s with binding %s", resPath, policy.GetName(), binding.GetName())
			return MutateResource(policy, &bindings[i], resource)
		}

		return emptyResp, nil
	}
}
func NewMutatingAdmissionPolicyBindingAccessor(binding *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) generic.BindingAccessor {
	return &mutatingBindingAccessor{binding}
}

type mutatingBindingAccessor struct {
	binding *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding
}

func (b *mutatingBindingAccessor) GetName() string {
	return b.binding.Name
}

func (b *mutatingBindingAccessor) GetNamespace() string {
	return b.binding.Namespace
}

func (b *mutatingBindingAccessor) GetMatchResources() *admissionregistrationv1.MatchResources {
	if b.binding.Spec.MatchResources == nil {
		return nil
	}
	converted := convertMatchResources(*b.binding.Spec.MatchResources)
	return &converted
}
func (b *mutatingBindingAccessor) GetParamRef() *admissionregistrationv1.ParamRef {
	return nil
}

func (b *mutatingBindingAccessor) GetPolicyName() types.NamespacedName {
	return types.NamespacedName{
		// no explicit policy field, so we can only provide binding's own namespace and empty name
		Namespace: b.binding.Namespace,
		Name:      "", // <-- no policy name, because field is missing
	}
}

func convertMatchResources(in admissionregistrationv1alpha1.MatchResources) admissionregistrationv1.MatchResources {
	var resourceRules []admissionregistrationv1.NamedRuleWithOperations
	for _, r := range in.ResourceRules {
		resourceRules = append(resourceRules, admissionregistrationv1.NamedRuleWithOperations{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: r.Operations,
				Rule: admissionregistrationv1.Rule{
					APIGroups:   r.APIGroups,
					APIVersions: r.APIVersions,
					Resources:   r.Resources,
					Scope:       r.Scope,
				},
			},
		})
	}

	var excludeResourceRules []admissionregistrationv1.NamedRuleWithOperations
	for _, r := range in.ExcludeResourceRules {
		excludeResourceRules = append(excludeResourceRules, admissionregistrationv1.NamedRuleWithOperations{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: r.Operations,
				Rule: admissionregistrationv1.Rule{
					APIGroups:   r.APIGroups,
					APIVersions: r.APIVersions,
					Resources:   r.Resources,
					Scope:       r.Scope,
				},
			},
		})
	}

	var matchPolicy *admissionregistrationv1.MatchPolicyType
	if in.MatchPolicy != nil {
		mp := admissionregistrationv1.MatchPolicyType(*in.MatchPolicy)
		matchPolicy = &mp
	}

	return admissionregistrationv1.MatchResources{
		NamespaceSelector:    in.NamespaceSelector,
		ObjectSelector:       in.ObjectSelector,
		ResourceRules:        resourceRules,
		ExcludeResourceRules: excludeResourceRules,
		MatchPolicy:          matchPolicy,
	}
}

func NewMutatingAdmissionPolicyAccessor(policy *admissionregistrationv1alpha1.MutatingAdmissionPolicy) generic.PolicyAccessor {
	return &mutatingPolicyAccessor{policy}
}

type mutatingPolicyAccessor struct {
	policy *admissionregistrationv1alpha1.MutatingAdmissionPolicy
}

// GetName implements generic.PolicyAccessor.
func (m *mutatingPolicyAccessor) GetName() string {
	return m.policy.Name
}

// GetNamespace implements generic.PolicyAccessor.
func (m *mutatingPolicyAccessor) GetNamespace() string {
	return m.policy.Namespace
}

func (m *mutatingPolicyAccessor) GetParamKind() *admissionregistrationv1.ParamKind {
	return nil // MAP currently does not use ParamKind in Kubernetes v1.32
}

func (m *mutatingPolicyAccessor) GetMatchConstraints() *admissionregistrationv1.MatchResources {
	if m.policy.Spec.MatchConstraints == nil {
		return nil
	}
	converted := convertMatchResources(*m.policy.Spec.MatchConstraints)
	return &converted
}
func (m *mutatingPolicyAccessor) GetFailurePolicy() *admissionregistrationv1.FailurePolicyType {
	if m.policy.Spec.FailurePolicy == nil {
		return nil
	}
	fp := admissionregistrationv1.FailurePolicyType(*m.policy.Spec.FailurePolicy)
	return &fp
}

func (m *mutatingPolicyAccessor) GetVariables() []admissionregistrationv1.Variable {
	vars := make([]admissionregistrationv1.Variable, len(m.policy.Spec.Variables))
	for i, v := range m.policy.Spec.Variables {
		vars[i] = admissionregistrationv1.Variable(v)
	}
	return vars
}

func (m *mutatingPolicyAccessor) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	conds := make([]admissionregistrationv1.MatchCondition, len(m.policy.Spec.MatchConditions))
	for i, c := range m.policy.Spec.MatchConditions {
		conds[i] = admissionregistrationv1.MatchCondition(c)
	}
	return conds
}
