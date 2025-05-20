package admissionpolicy

import (
	"context"
	"fmt"
	"time"

	celmatching "github.com/kyverno/kyverno/pkg/cel/matching"
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
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/generic"
	k8smatching "k8s.io/apiserver/pkg/admission/plugin/policy/matching"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

func MutateResource(
	policy admissionregistrationv1alpha1.MutatingAdmissionPolicy,
	binding *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding,
	resource unstructured.Unstructured,
	gvr schema.GroupVersionResource,
	client dclient.Interface,
	namespaceSelectorMap map[string]map[string]string,
	isFake bool,
) (engineapi.EngineResponse, error) {
	startTime := time.Now()

	engineResponse := engineapi.NewEngineResponse(resource, engineapi.NewMutatingAdmissionPolicy(&policy), nil)
	policyResp := engineapi.NewPolicyResponse()

	gvk := resource.GroupVersionKind()

	var namespace *corev1.Namespace
	namespaceName := resource.GetNamespace()
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
		matchResults := matcher.Match(context.TODO(), versionedAttributes, namespace, nil)

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
		ruleName := policy.GetName()

		newVersionedObject, err := patcher.Patch(context.TODO(), patchRequest, celconfig.RuntimeCELCostBudget)
		if err != nil {
			ruleResp := engineapi.RuleError(policy.GetName(), engineapi.Mutation, err.Error(), nil, nil).
				WithResource(resource.GetNamespace(), resource.GetName(), gvk.Kind)

			if binding != nil {
				ruleResp = ruleResp.WithMutatingBinding(binding)
			}
			policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
			continue
		}

		if equality.Semantic.DeepEqual(original, newVersionedObject) {
			ruleResp := engineapi.RuleSkip(ruleName, engineapi.Mutation, "mutation had no effect", nil).
				WithResource(resource.GetNamespace(), resource.GetName(), gvk.Kind)
			if binding != nil {
				ruleResp = ruleResp.WithMutatingBinding(binding)
			}
			policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
			continue
		}
		versionedAttributes.VersionedObject = newVersionedObject
		ruleResp := engineapi.RulePass(ruleName, engineapi.Mutation, "", nil).
			WithResource(resource.GetNamespace(), resource.GetName(), gvk.Kind)
		if binding != nil {
			ruleResp = ruleResp.WithMutatingBinding(binding)
		}
		policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
	}

	patchedResource, err := celutils.ConvertObjectToUnstructured(versionedAttributes.VersionedObject)
	if err != nil {
		ruleResp := engineapi.RuleError(policy.GetName(), engineapi.Mutation, err.Error(), nil, nil).
			WithResource(resource.GetNamespace(), resource.GetName(), gvk.Kind)
		if binding != nil {
			ruleResp = ruleResp.WithMutatingBinding(binding)
		}
		policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
		return engineResponse.WithPolicyResponse(policyResp), nil
	}

	patchedResource.SetName(resource.GetName())
	patchedResource.SetNamespace(resource.GetNamespace())
	engineResponse = engineResponse.
		WithPatchedResource(*patchedResource).
		WithPolicyResponse(policyResp)

	return engineResponse, nil
}

func Mutate(
	data engineapi.MutatingAdmissionPolicyData,
	resource unstructured.Unstructured,
	gvr schema.GroupVersionResource,
	client dclient.Interface,
	namespaceSelectorMap map[string]map[string]string,
	isFake bool,
) (engineapi.EngineResponse, error) {
	var emptyResp engineapi.EngineResponse

	resPath := fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
	policy := data.GetDefinition()
	bindings := data.GetBindings()
	gvk := resource.GroupVersionKind()

	a := admission.NewAttributesRecord(
		resource.DeepCopyObject(), nil,
		gvk,
		resource.GetNamespace(), resource.GetName(),
		gvr, "", admission.Create, nil, false, nil,
	)

	var namespace *corev1.Namespace
	namespaceName := resource.GetNamespace()
	if gvk.Kind == "Namespace" && gvk.Version == "v1" && gvk.Group == "" {
		namespaceName = ""
	}

	if namespaceName != "" {
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   namespaceName,
				Labels: namespaceSelectorMap[namespaceName],
			},
		}
	}

	offlineMatcher := celmatching.NewMatcher()

	// no bindings: offline CEL matcher
	if len(bindings) == 0 {
		pr := convertMatchResources(*policy.Spec.MatchConstraints)
		isMatch, err := offlineMatcher.Match(
			&celmatching.MatchCriteria{Constraints: &pr},
			a,
			namespace,
		)
		if err != nil {
			return emptyResp, err
		}
		if !isMatch {
			return emptyResp, nil
		}

		return MutateResource(*policy, nil, resource, gvr, client, namespaceSelectorMap, isFake)
	}

	// bindings exist
	if client != nil && !isFake {
		nsLister := NewCustomNamespaceLister(client)
		policyMatcher := generic.NewPolicyMatcher(k8smatching.NewMatcher(nsLister, client.GetKubeClient()))

		o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())

		// match policy
		isPolicyMatch, _, _, err := policyMatcher.DefinitionMatches(a, o, mutating.NewMutatingAdmissionPolicyAccessor(policy))
		if err != nil {
			return emptyResp, err
		}
		if !isPolicyMatch {
			return emptyResp, nil
		}

		// match bindings
		for i, binding := range bindings {
			isBindingMatch, err := policyMatcher.BindingMatches(a, o, mutating.NewMutatingAdmissionPolicyBindingAccessor(&binding))
			if err != nil {
				return emptyResp, err
			}
			if !isBindingMatch {
				continue
			}

			logger.V(3).Info("mutate resource %s against policy %s with binding %s", resPath, policy.GetName(), binding.GetName())
			return MutateResource(*policy, &bindings[i], resource, gvr, client, namespaceSelectorMap, isFake)
		}
		return emptyResp, nil
	} else {
		offline := celmatching.NewMatcher()
		// 1) policy-level
		pr := convertMatchResources(*policy.Spec.MatchConstraints)
		ok, err := offline.Match(
			&celmatching.MatchCriteria{Constraints: &pr},
			a,
			namespace,
		)
		if err != nil {
			return emptyResp, err
		}
		if !ok {
			return emptyResp, nil
		}

		// 2) binding-level
		for i, binding := range bindings {
			if binding.Spec.MatchResources == nil {
				continue
			}
			pr := convertMatchResources(*binding.Spec.MatchResources)
			ok, err := offline.Match(
				&celmatching.MatchCriteria{Constraints: &pr},
				a,
				namespace,
			)
			if err != nil {
				return emptyResp, err
			}
			if !ok {
				continue
			}
			return MutateResource(*policy, &bindings[i], resource, gvr, client, namespaceSelectorMap, isFake)
		}

		return emptyResp, nil
	}
}

// convertMatchResources turns a v1alpha1.MatchResources into a v1.MatchResources
func convertMatchResources(in admissionregistrationv1alpha1.MatchResources) admissionregistrationv1.MatchResources {
	resourceRules := make([]admissionregistrationv1.NamedRuleWithOperations, 0, len(in.ResourceRules))
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
	exclude := make([]admissionregistrationv1.NamedRuleWithOperations, 0, len(in.ExcludeResourceRules))
	for _, r := range in.ExcludeResourceRules {
		exclude = append(exclude, admissionregistrationv1.NamedRuleWithOperations{
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
	var mp *admissionregistrationv1.MatchPolicyType
	if in.MatchPolicy != nil {
		m := admissionregistrationv1.MatchPolicyType(*in.MatchPolicy)
		mp = &m
	}
	return admissionregistrationv1.MatchResources{
		NamespaceSelector:    in.NamespaceSelector,
		ObjectSelector:       in.ObjectSelector,
		ResourceRules:        resourceRules,
		ExcludeResourceRules: exclude,
		MatchPolicy:          mp,
	}
}
