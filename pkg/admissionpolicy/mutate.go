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
	matching "k8s.io/apiserver/pkg/admission/plugin/policy/matching"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

func Mutate(
	data *engineapi.MutatingAdmissionPolicyData,
	resource unstructured.Unstructured,
	gvk schema.GroupVersionKind,
	gvr schema.GroupVersionResource,
	namespaceSelectorMap map[string]map[string]string,
	client dclient.Interface,
	isFake bool,
	backgroundScan bool,
) (engineapi.EngineResponse, error) {
	resPath := fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
	policy := data.GetDefinition()
	bindings := data.GetBindings()
	engineResponse := engineapi.NewEngineResponse(resource, engineapi.NewMutatingAdmissionPolicy(policy), nil)

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
				Name:   namespaceName,
				Labels: namespaceSelectorMap[namespaceName],
			},
		}
	}
	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)

	if len(bindings) == 0 {
		matchResources := ConvertMatchResources(policy.Spec.MatchConstraints)
		matcher := celmatching.NewMatcher()
		isMatch, err := matcher.Match(
			&celmatching.MatchCriteria{
				Constraints: matchResources,
			},
			a,
			namespace,
		)
		if err != nil {
			return engineResponse, err
		}
		if !isMatch {
			return engineResponse, nil
		}
		mapLogger.V(3).Info("apply mutatingadmissionpolicy %s to resource %s", policy.GetName(), resPath)
		return mutateResource(policy, nil, resource, gvr, namespace, a, backgroundScan)
	}

	if client != nil && !isFake {
		nsLister := NewCustomNamespaceLister(client)
		matcher := generic.NewPolicyMatcher(matching.NewMatcher(nsLister, client.GetKubeClient()))

		// check if policy matches the incoming resource
		o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())
		isMatch, _, _, err := matcher.DefinitionMatches(a, o, mutating.NewMutatingAdmissionPolicyAccessor(policy))
		if err != nil {
			return engineResponse, err
		}
		if !isMatch {
			return engineResponse, nil
		}

		if namespaceName != "" {
			namespace, err = client.GetKubeClient().CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
			if err != nil {
				return engineResponse, err
			}
		}

		for i, binding := range bindings {
			isMatch, err := matcher.BindingMatches(a, o, mutating.NewMutatingAdmissionPolicyBindingAccessor(&binding))
			if err != nil {
				return engineResponse, err
			}
			if !isMatch {
				fmt.Println("bindings does not match the incoming resource, skipping mutations")
				continue
			}

			mapLogger.V(3).Info("apply mutatingadmissionpolicy %s with its binding %s to the resource %s", policy.GetName(), binding.GetName(), resPath)
			return mutateResource(policy, &bindings[i], resource, gvr, namespace, a, backgroundScan)
		}
	} else {
		matcher := celmatching.NewMatcher()
		for i, binding := range bindings {
			// check if the binding matches the incoming resource
			if binding.Spec.MatchResources != nil {
				matchResources := ConvertMatchResources(binding.Spec.MatchResources)
				bindingMatches, err := matcher.Match(
					&celmatching.MatchCriteria{
						Constraints: matchResources,
					},
					a,
					namespace,
				)
				if err != nil {
					return engineResponse, err
				}
				if !bindingMatches {
					continue
				}
			}
			return mutateResource(policy, &bindings[i], resource, gvr, namespace, a, backgroundScan)
		}
	}
	return engineResponse, nil
}

func mutateResource(
	policy *admissionregistrationv1alpha1.MutatingAdmissionPolicy,
	binding *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding,
	resource unstructured.Unstructured,
	gvr schema.GroupVersionResource,
	namespace *corev1.Namespace,
	a admission.Attributes,
	backgroundScan bool,
) (engineapi.EngineResponse, error) {
	startTime := time.Now()

	engineResponse := engineapi.NewEngineResponse(resource, engineapi.NewMutatingAdmissionPolicy(policy), nil)
	policyResp := engineapi.NewPolicyResponse()
	var patchedResource *unstructured.Unstructured
	var ruleResp *engineapi.RuleResponse

	// compile CEL expressions
	matchConditions := convertMatchConditions(policy.Spec.MatchConditions)
	variables := convertVariables(policy.Spec.Variables)
	compiler, err := NewCompiler(matchConditions, variables)
	if err != nil {
		return engineResponse, err
	}
	optionalVars := cel.OptionalVariableDeclarations{
		HasParams:     false,
		HasAuthorizer: false,
		HasPatchTypes: true,
		StrictCost:    true,
	}
	// compile variables
	compiler.CompileVariables(optionalVars)

	var matchPolicy admissionregistrationv1.MatchPolicyType
	if policy.Spec.MatchConstraints.MatchPolicy == nil {
		matchPolicy = admissionregistrationv1.Equivalent
	} else {
		matchPolicy = admissionregistrationv1.MatchPolicyType(*policy.Spec.MatchConstraints.MatchPolicy)
	}
	var failPolicy admissionregistrationv1.FailurePolicyType
	if policy.Spec.FailurePolicy == nil {
		failPolicy = admissionregistrationv1.Fail
	} else {
		failPolicy = admissionregistrationv1.FailurePolicyType(*policy.Spec.FailurePolicy)
	}
	versionedAttributes, _ := admission.NewVersionedAttributes(a, a.GetKind(), nil)
	o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())

	// compile match conditions and check if the incoming resource matches them
	matcher := matchconditions.NewMatcher(compiler.CompileMatchConditions(optionalVars), &failPolicy, "policy", string(matchPolicy), policy.Name)
	matchResults := matcher.Match(context.TODO(), versionedAttributes, nil, nil)
	if matchResults.Error != nil {
		return engineResponse, matchResults.Error
	} else if !matchResults.Matches {
		// match conditions are not met, then skip mutations
		ruleResp = engineapi.RuleSkip(policy.GetName(), engineapi.Mutation, "match conditions aren't met", nil)
	} else {
		// if match conditions are met, we can proceed with the mutations
		compiler.WithMutations(policy.Spec.Mutations)
		patchers := compiler.CompileMutations(optionalVars)
		for _, patcher := range patchers {
			patchRequest := patch.Request{
				MatchedResource:     gvr,
				VersionedAttributes: versionedAttributes,
				ObjectInterfaces:    o,
				OptionalVariables:   cel.OptionalVariableBindings{VersionedParams: nil, Authorizer: nil},
				Namespace:           namespace,
				TypeConverter:       managedfields.NewDeducedTypeConverter(),
			}
			newVersionedObject, err := patcher.Patch(context.TODO(), patchRequest, celconfig.RuntimeCELCostBudget)
			if err != nil {
				return engineResponse, nil
			}
			versionedAttributes.Dirty = true
			versionedAttributes.VersionedObject = newVersionedObject
		}
		patchedResource, err = celutils.ConvertObjectToUnstructured(versionedAttributes.VersionedObject)
		if err != nil {
			return engineResponse, err
		}
		// in case of background scan and the existing resource is not already mutated, we should return a fail response
		if backgroundScan && !equality.Semantic.DeepEqual(resource.DeepCopyObject(), patchedResource.DeepCopyObject()) {
			ruleResp = engineapi.RuleFail(policy.GetName(), engineapi.Mutation, "mutation is not applied", nil)
		} else {
			ruleResp = engineapi.RulePass(policy.GetName(), engineapi.Mutation, "mutation is successfully applied", nil)
		}
	}

	if binding != nil {
		ruleResp = ruleResp.WithMAPBinding(binding)
	}
	policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
	if patchedResource != nil {
		engineResponse = engineResponse.
			WithPatchedResource(*patchedResource).
			WithPolicyResponse(policyResp)
	} else {
		engineResponse = engineResponse.WithPolicyResponse(policyResp)
	}
	return engineResponse, nil
}
