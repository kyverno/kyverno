package admissionpolicy

import (
	"context"
	"fmt"
	"time"

	celmatching "github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	celutils "github.com/kyverno/sdk/cel/utils"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
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
	userInfo *authenticationv1.UserInfo,
	isFake bool,
	backgroundScan bool,
) (engineapi.EngineResponse, error) {
	var (
		resPath       = fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
		policy        = data.GetDefinition()
		bindings      = data.GetBindings()
		namespace     *corev1.Namespace
		namespaceName = resource.GetNamespace()
	)

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
	var user UserInfo
	if userInfo != nil {
		user = NewUser(*userInfo)
	}
	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, user)

	if len(bindings) == 0 {
		return processMAPNoBindings(policy, resource, namespace, gvr, a, resPath, backgroundScan)
	}

	if client != nil && !isFake {
		return processMAPWithClient(policy, bindings, resource, namespaceName, namespace, client, gvr, a, resPath, backgroundScan)
	}

	return processMAPWithoutClient(policy, bindings, resource, namespace, data.GetParams(), gvr, a, resPath, backgroundScan)
}

func processMAPNoBindings(policy *admissionregistrationv1beta1.MutatingAdmissionPolicy, resource unstructured.Unstructured, namespace *corev1.Namespace, gvr schema.GroupVersionResource, a admission.Attributes, resPath string, backgroundScan bool) (engineapi.EngineResponse, error) {
	matcher := celmatching.NewMatcher()
	matchResources := ConvertMatchResources(policy.Spec.MatchConstraints)
	er := engineapi.NewEngineResponse(resource, engineapi.NewMutatingAdmissionPolicy(policy), nil)

	isMatch, err := matcher.Match(&celmatching.MatchCriteria{Constraints: matchResources}, a, namespace)
	if err != nil {
		mapLogger.Error(err, "failed to match resource against mutatingadmissionpolicy constraints", "policy", policy.GetName(), "resource", resPath)
		return er, err
	}
	if !isMatch {
		return engineapi.NewEngineResponse(resource, engineapi.NewMutatingAdmissionPolicy(policy), nil), nil
	}

	mapLogger.V(3).Info("applying mutatingadmissionpolicy to resource", "policy", policy.GetName(), "resource", resPath)
	er, err = mutateResource(policy, nil, resource, nil, gvr, namespace, a, backgroundScan)
	if err != nil {
		mapLogger.Error(err, "failed to mutate resource with mutatingadmissionpolicy", "policy", policy.GetName(), "resource", resPath)
		return er, err
	}
	return er, nil
}

func processMAPWithClient(policy *admissionregistrationv1beta1.MutatingAdmissionPolicy, bindings []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding, resource unstructured.Unstructured, namespaceName string, namespace *corev1.Namespace, client dclient.Interface, gvr schema.GroupVersionResource, a admission.Attributes, resPath string, backgroundScan bool) (engineapi.EngineResponse, error) {
	nsLister := NewCustomNamespaceLister(client)
	matcher := generic.NewPolicyMatcher(matching.NewMatcher(nsLister, client.GetKubeClient()))
	o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())
	er := engineapi.NewEngineResponse(resource, engineapi.NewMutatingAdmissionPolicy(policy), nil)

	// the two nil checks and the addition of an empty selector are needed for policies that specify no namespaceSelector or objectSelector.
	// during parsing those selectors in the DefinitionMatches function, if they are nil, they skip all resources
	// this should be moved to upstream k8s. see: https://github.com/kubernetes/kubernetes/pull/133575
	if policy.Spec.MatchConstraints != nil {
		if policy.Spec.MatchConstraints.NamespaceSelector == nil {
			policy.Spec.MatchConstraints.NamespaceSelector = &metav1.LabelSelector{}
		}
		if policy.Spec.MatchConstraints.ObjectSelector == nil {
			policy.Spec.MatchConstraints.ObjectSelector = &metav1.LabelSelector{}
		}
	}

	isMatch, _, _, err := matcher.DefinitionMatches(a, o, mutating.NewMutatingAdmissionPolicyAccessor(policy))
	if err != nil {
		mapLogger.Error(err, "failed to match policy definition for mutatingadmissionpolicy", "policy", policy.GetName(), "resource", resPath)
		return er, err
	}
	if !isMatch {
		return er, nil
	}

	if namespaceName != "" {
		namespace, err = client.GetKubeClient().CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
		if err != nil {
			mapLogger.Error(err, "failed to get namespace for mutatingadmissionpolicy", "policy", policy.GetName(), "namespace", namespaceName, "resource", resPath)
			return er, err
		}
	}

	for i, binding := range bindings {
		if binding.Spec.MatchResources != nil {
			if binding.Spec.MatchResources.NamespaceSelector == nil {
				binding.Spec.MatchResources.NamespaceSelector = &metav1.LabelSelector{}
			}
			if binding.Spec.MatchResources.ObjectSelector == nil {
				binding.Spec.MatchResources.ObjectSelector = &metav1.LabelSelector{}
			}
		}

		isMatch, err := matcher.BindingMatches(a, o, mutating.NewMutatingAdmissionPolicyBindingAccessor(&binding))
		if err != nil {
			mapLogger.Error(err, "failed to match policy binding for mutatingadmissionpolicy", "policy", policy.GetName(), "binding", binding.GetName(), "resource", resPath)
			continue
		}
		if !isMatch {
			continue
		}

		if binding.Spec.ParamRef != nil {
			params, err := CollectParams(context.TODO(), adapters.Client(client), convertParamKind(policy.Spec.ParamKind), convertParamRef(binding.Spec.ParamRef), namespace.Name)
			if err != nil {
				mapLogger.Error(err, "failed to collect params for mutatingadmissionpolicy", "policy", policy.GetName(), "binding", binding.GetName(), "resource", resPath)
				return er, err
			}

			// a selector being present in the binding is the only case in which params will contain more than 1 entry
			var matchedParams runtime.Object
			if len(params) > 1 {
				paramList := &unstructured.UnstructuredList{}
				for _, p := range params {
					unstructuredMap, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(p)
					obj := &unstructured.Unstructured{Object: unstructuredMap}
					paramList.Items = append(paramList.Items, *obj)
				}
				matchedParams = paramList
			} else {
				matchedParams = params[0]
			}

			newEr, err := mutateResource(policy, &bindings[i], resource, matchedParams, gvr, namespace, a, backgroundScan)
			if err != nil {
				mapLogger.Error(err, "failed to mutate resource with params for mutatingadmissionpolicy", "policy", policy.GetName(), "binding", binding.GetName(), "resource", resPath)
				continue
			}
			// replace the resource with the new patched resource to apply it to the next binding and replace the engine response with the latest engine response
			resource = newEr.PatchedResource
			er = newEr
		} else {
			newEr, err := mutateResource(policy, &bindings[i], resource, nil, gvr, namespace, a, backgroundScan)
			if err != nil {
				mapLogger.Error(err, "failed to mutate resource with params for mutatingadmissionpolicy", "policy", policy.GetName(), "binding", binding.GetName(), "resource", resPath)
				continue
			}
			// replace the resource with the new patched resource to apply it to the next binding and replace the engine response with the latest engine response
			resource = newEr.PatchedResource
			er = newEr
		}
	}
	return er, nil
}

func processMAPWithoutClient(policy *admissionregistrationv1beta1.MutatingAdmissionPolicy, bindings []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding, resource unstructured.Unstructured, namespace *corev1.Namespace, params []runtime.Object, gvr schema.GroupVersionResource, a admission.Attributes, resPath string, backgroundScan bool) (engineapi.EngineResponse, error) {
	matcher := celmatching.NewMatcher()
	er := engineapi.NewEngineResponse(resource, engineapi.NewMutatingAdmissionPolicy(policy), nil)

	for i, binding := range bindings {
		if binding.Spec.MatchResources != nil {
			matchResources := ConvertMatchResources(binding.Spec.MatchResources)
			bindingMatches, err := matcher.Match(&celmatching.MatchCriteria{Constraints: matchResources}, a, namespace)
			if err != nil {
				mapLogger.Error(err, "failed to match binding resources for mutatingadmissionpolicy", "policy", policy.GetName(), "binding", binding.GetName(), "resource", resPath)
				continue
			}
			if !bindingMatches {
				continue
			}
		}
		if binding.Spec.ParamRef != nil {
			var matchedParams runtime.Object
			paramList := &unstructured.UnstructuredList{}
			for _, param := range params {
				unstructuredMap, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(param)
				obj := &unstructured.Unstructured{Object: unstructuredMap}
				if matchesSelector(obj, convertParamRef(binding.Spec.ParamRef)) {
					// if there is no selector, the binding will match the first resource only. match the first resource and exit
					if binding.Spec.ParamRef.Selector == nil {
						matchedParams = obj
						break
					}
					paramList.Items = append(paramList.Items, *obj)
				}
			}
			// if there were resources in the parameter list, use it as the matched params
			if len(paramList.Items) != 0 {
				matchedParams = paramList
			}

			newEr, err := mutateResource(policy, &bindings[i], resource, matchedParams, gvr, namespace, a, backgroundScan)
			if err != nil {
				mapLogger.Error(err, "failed to mutate resource for mutatingadmissionpolicy", "policy", policy.GetName(), "binding", binding.GetName(), "resource", resPath)
				continue
			}
			resource = newEr.PatchedResource
			er = newEr
		} else {
			newEr, err := mutateResource(policy, &bindings[i], resource, nil, gvr, namespace, a, backgroundScan)
			if err != nil {
				mapLogger.Error(err, "failed to mutate resource with params for mutatingadmissionpolicy", "policy", policy.GetName(), "binding", binding.GetName(), "resource", resPath)
				continue
			}
			resource = newEr.PatchedResource
			er = newEr
		}
	}
	return er, nil
}

func matchesSelector(obj *unstructured.Unstructured, ref *admissionregistrationv1.ParamRef) bool {
	if ref.Selector != nil {
		selector, _ := metav1.LabelSelectorAsSelector(ref.Selector)
		return selector.Matches(labels.Set(obj.GetLabels()))
	}
	matches := false
	if ref.Namespace != "" {
		matches = obj.GetNamespace() == ref.Namespace
	}
	if ref.Name != "" {
		matches = obj.GetName() == ref.Name
	}
	return matches
}

func mutateResource(
	policy *admissionregistrationv1beta1.MutatingAdmissionPolicy,
	binding *admissionregistrationv1beta1.MutatingAdmissionPolicyBinding,
	resource unstructured.Unstructured,
	param runtime.Object,
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
		mapLogger.Error(err, "failed to create compiler for mutatingadmissionpolicy", "policy", policy.GetName(), "resource", fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName()))
		return engineResponse, err
	}
	optionalVars := cel.OptionalVariableDeclarations{
		HasParams:     param != nil,
		HasAuthorizer: false,
		HasPatchTypes: true,
	}
	// compile variables
	compiler.CompileVariables(optionalVars)

	var matchPolicy admissionregistrationv1.MatchPolicyType
	if policy.Spec.MatchConstraints == nil || policy.Spec.MatchConstraints.MatchPolicy == nil {
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
		mapLogger.Error(matchResults.Error, "match conditions evaluation failed for mutatingadmissionpolicy", "policy", policy.GetName(), "resource", fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName()))
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
				OptionalVariables:   cel.OptionalVariableBindings{VersionedParams: param, Authorizer: nil},
				Namespace:           namespace,
				TypeConverter:       managedfields.NewDeducedTypeConverter(),
			}
			newVersionedObject, err := patcher.Patch(context.TODO(), patchRequest, celconfig.RuntimeCELCostBudget)
			if err != nil {
				mapLogger.Error(err, "failed to apply patch for mutatingadmissionpolicy", "policy", policy.GetName(), "resource", fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName()))
				return engineResponse, nil
			}
			versionedAttributes.Dirty = true
			versionedAttributes.VersionedObject = newVersionedObject
		}
		patchedResource, err = celutils.ConvertObjectToUnstructured(versionedAttributes.VersionedObject)
		if err != nil {
			mapLogger.Error(err, "failed to convert patched object to unstructured for mutatingadmissionpolicy", "policy", policy.GetName(), "resource", fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName()))
			return engineResponse, err
		}
		// in case of background scan and the existing resource is not already mutated, we should return a fail response
		if backgroundScan && !equality.Semantic.DeepEqual(resource.DeepCopyObject(), patchedResource.DeepCopyObject()) {
			mapLogger.V(2).Info("mutation not applied during background scan for mutatingadmissionpolicy", "policy", policy.GetName(), "resource", fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName()))
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
