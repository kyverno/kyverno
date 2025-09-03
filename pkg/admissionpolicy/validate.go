package admissionpolicy

import (
	"context"
	"fmt"
	"strings"
	"time"

	celmatching "github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/generic"
	"k8s.io/apiserver/pkg/admission/plugin/policy/matching"
	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

func GetKinds(matchResources *admissionregistrationv1.MatchResources, mapper meta.RESTMapper) []string {
	if matchResources == nil {
		return nil
	}

	var kindList []string
	for _, rule := range matchResources.ResourceRules {
		if len(rule.APIGroups) == 0 || len(rule.APIVersions) == 0 {
			continue
		}

		for _, group := range rule.APIGroups {
			for _, version := range rule.APIVersions {
				for _, resource := range rule.Resources {
					kinds, err := resolveKinds(group, version, resource, mapper)
					if err != nil {
						vapLogger.Error(err, fmt.Sprintf("failed to resolve kind for group %s, version %s, resource %s", group, version, resource))
						continue
					}
					kindList = append(kindList, kinds...)
				}
			}
		}
	}
	return kindList
}

func resolveKinds(group, version, resource string, mapper meta.RESTMapper) ([]string, error) {
	var kinds []string

	formatGVK := func(gvk schema.GroupVersionKind, subresource string) string {
		var parts []string
		if gvk.Group != "" {
			parts = append(parts, gvk.Group)
		}
		parts = append(parts, gvk.Version, gvk.Kind)
		if subresource != "" {
			parts = append(parts, subresource)
		}
		return strings.Join(parts, "/")
	}

	switch {
	case group == "*" || version == "*" || resource == "*":
		gvrList, err := mapper.ResourcesFor(schema.GroupVersionResource{
			Group: group, Version: version, Resource: resource,
		})
		if err != nil {
			return nil, fmt.Errorf("list resources failed for %s/%s/%s: %w", group, version, resource, err)
		}
		for _, gvr := range gvrList {
			gvk, err := mapper.KindFor(gvr)
			if err != nil {
				return nil, fmt.Errorf("kind lookup failed for %v: %w", gvr, err)
			}
			kinds = append(kinds, formatGVK(gvk, ""))
		}
	case kubeutils.IsSubresource(resource):
		parts := strings.SplitN(resource, "/", 2)
		mainResource, subResource := parts[0], parts[1]
		gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: mainResource}
		gvk, err := mapper.KindFor(gvr)
		if err != nil {
			return nil, fmt.Errorf("mapping gvr %v to gvk failed: %w", gvr, err)
		}
		kinds = append(kinds, formatGVK(gvk, subResource))
	default:
		gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
		gvk, err := mapper.KindFor(gvr)
		if err != nil {
			return nil, fmt.Errorf("mapping gvr %v to gvk failed: %w", gvr, err)
		}
		kinds = append(kinds, formatGVK(gvk, ""))
	}

	return kinds, nil
}

func Validate(
	policyData *engineapi.ValidatingAdmissionPolicyData,
	resource unstructured.Unstructured,
	gvk schema.GroupVersionKind,
	gvr schema.GroupVersionResource,
	namespaceSelectorMap map[string]map[string]string,
	client dclient.Interface,
	userInfo *authenticationv1.UserInfo,
	isFake bool,
) (engineapi.EngineResponse, error) {
	var (
		resPath       = fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
		policy        = policyData.GetDefinition()
		bindings      = policyData.GetBindings()
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
		return processVAPNoBindings(policy, resource, namespace, a, resPath)
	}

	if client != nil && !isFake {
		return processVAPWithClient(policy, bindings, resource, namespaceName, namespace, client, a, resPath)
	}

	return processVAPWithoutClient(policy, bindings, resource, namespace, policyData.GetParams(), a, resPath)
}

func processVAPNoBindings(policy *admissionregistrationv1.ValidatingAdmissionPolicy, resource unstructured.Unstructured, namespace *corev1.Namespace, a admission.Attributes, resPath string) (engineapi.EngineResponse, error) {
	matcher := celmatching.NewMatcher()
	isMatch, err := matcher.Match(&celmatching.MatchCriteria{Constraints: policy.Spec.MatchConstraints}, a, namespace)
	er := engineapi.NewEngineResponse(resource, engineapi.NewValidatingAdmissionPolicy(policy), nil)

	if err != nil {
		vapLogger.Error(err, "failed to match resource against validatingadmissionpolicy constraints", "policy", policy.GetName(), "resource", resPath)
		return er, err
	}
	if !isMatch {
		return er, nil
	}

	vapLogger.V(3).Info("apply mutatingadmissionpolicy %s to resource %s", policy.GetName(), resPath)
	er, err = validateResource(policy, nil, resource, nil, namespace, a)
	if err != nil {
		vapLogger.Error(err, "failed to validate resource with validatingadmissionpolicy", "policy", policy.GetName(), "resource", resPath)
		return er, err
	}
	return er, nil
}

func processVAPWithClient(policy *admissionregistrationv1.ValidatingAdmissionPolicy, bindings []admissionregistrationv1.ValidatingAdmissionPolicyBinding, resource unstructured.Unstructured, namespaceName string, namespace *corev1.Namespace, client dclient.Interface, a admission.Attributes, resPath string) (engineapi.EngineResponse, error) {
	nsLister := NewCustomNamespaceLister(client)
	matcher := generic.NewPolicyMatcher(matching.NewMatcher(nsLister, client.GetKubeClient()))
	o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())
	er := engineapi.NewEngineResponse(resource, engineapi.NewValidatingAdmissionPolicy(policy), nil)

	// the two nil checks and the addition of an empty selector are needed for policies that specify no namespaceSelector or objectSelector.
	// during parsing those selectors in the DefinitionMatches function, if they are nil, they skip all resources.
	// this should be moved to upstream k8s. see: https://github.com/kubernetes/kubernetes/pull/133575
	if policy.Spec.MatchConstraints != nil {
		if policy.Spec.MatchConstraints.NamespaceSelector == nil {
			policy.Spec.MatchConstraints.NamespaceSelector = &metav1.LabelSelector{}
		}
		if policy.Spec.MatchConstraints.ObjectSelector == nil {
			policy.Spec.MatchConstraints.ObjectSelector = &metav1.LabelSelector{}
		}
	}

	isMatch, _, _, err := matcher.DefinitionMatches(a, o, validating.NewValidatingAdmissionPolicyAccessor(policy))
	if err != nil {
		vapLogger.Error(err, "failed to match policy definition for validatingadmissionpolicy", "policy", policy.GetName(), "resource", resPath)
		return er, err
	}
	if !isMatch {
		return er, nil
	}

	if namespaceName != "" {
		namespace, err = client.GetKubeClient().CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
		if err != nil {
			vapLogger.Error(err, "failed to get namespace for validatingadmissionpolicy", "policy", policy.GetName(), "namespace", namespaceName, "resource", resPath)
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

		isMatch, err := matcher.BindingMatches(a, o, validating.NewValidatingAdmissionPolicyBindingAccessor(&binding))
		if err != nil {
			vapLogger.Error(err, "failed to match policy binding for validatingadmissionpolicy", "policy", policy.GetName(), "binding", binding.GetName(), "resource", resPath)
			continue
		}
		if !isMatch {
			continue
		}

		if binding.Spec.ParamRef != nil {
			params, err := CollectParams(context.TODO(), adapters.Client(client), policy.Spec.ParamKind, binding.Spec.ParamRef, namespace.Name)
			if err != nil {
				vapLogger.Error(err, "failed to collect params for validatingadmissionpolicy", "policy", policy.GetName(), "binding", binding.GetName(), "resource", resPath)
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
			engineResponse, err := validateResource(policy, &bindings[i], resource, matchedParams, namespace, a)
			if err != nil {
				vapLogger.Error(err, "failed to validate resource with params for validatingadmissionpolicy", "policy", policy.GetName(), "binding", binding.GetName(), "resource", resPath)
				continue
			}
			er = engineResponse
		} else {
			engineResponse, err := validateResource(policy, &bindings[i], resource, nil, namespace, a)
			if err != nil {
				vapLogger.Error(err, "failed to validate resource for validatingadmissionpolicy", "policy", policy.GetName(), "binding", binding.GetName(), "resource", resPath)
				continue
			}
			er = engineResponse
		}
	}
	return er, nil
}

func processVAPWithoutClient(policy *admissionregistrationv1.ValidatingAdmissionPolicy, bindings []admissionregistrationv1.ValidatingAdmissionPolicyBinding, resource unstructured.Unstructured, namespace *corev1.Namespace, params []runtime.Object, a admission.Attributes, resPath string) (engineapi.EngineResponse, error) {
	matcher := celmatching.NewMatcher()
	er := engineapi.NewEngineResponse(resource, engineapi.NewValidatingAdmissionPolicy(policy), nil)

	for i, binding := range bindings {
		if binding.Spec.MatchResources != nil {
			bindingMatches, err := matcher.Match(&celmatching.MatchCriteria{Constraints: binding.Spec.MatchResources}, a, namespace)
			if err != nil {
				vapLogger.Error(err, "failed to match binding resources for validatingadmissionpolicy", "policy", policy.GetName(), "binding", binding.GetName(), "resource", resPath)
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
				if matchesSelector(obj, binding.Spec.ParamRef) {
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

			engineResponse, err := validateResource(policy, &bindings[i], resource, matchedParams, namespace, a)
			if err != nil {
				vapLogger.Error(err, "failed to validate resource with params for validatingadmissionpolicy", "policy", policy.GetName(), "binding", binding.GetName(), "resource", resPath)
				continue
			}
			er = engineResponse
		} else {
			engineResponse, err := validateResource(policy, &bindings[i], resource, nil, namespace, a)
			if err != nil {
				vapLogger.Error(err, "failed to validate resource for validatingadmissionpolicy", "policy", policy.GetName(), "binding", binding.GetName(), "resource", resPath)
				continue
			}
			er = engineResponse
		}
	}
	return er, nil
}

func validateResource(
	policy *admissionregistrationv1.ValidatingAdmissionPolicy,
	binding *admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	resource unstructured.Unstructured,
	parameterResource runtime.Object,
	namespace *corev1.Namespace,
	a admission.Attributes,
) (engineapi.EngineResponse, error) {
	startTime := time.Now()

	engineResponse := engineapi.NewEngineResponse(resource, engineapi.NewValidatingAdmissionPolicy(policy), nil)
	policyResp := engineapi.NewPolicyResponse()
	var ruleResp *engineapi.RuleResponse

	// compile CEL expressions
	compiler, err := NewCompiler(policy.Spec.MatchConditions, policy.Spec.Variables)
	if err != nil {
		vapLogger.Error(err, "failed to create compiler for validatingadmissionpolicy", "policy", policy.GetName(), "resource", fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName()))
		return engineResponse, err
	}
	compiler.WithValidations(policy.Spec.Validations)
	compiler.WithAuditAnnotations(policy.Spec.AuditAnnotations)

	hasParam := policy.Spec.ParamKind != nil
	optionalVars := cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: false}
	compiler.CompileVariables(optionalVars)

	var matchPolicy admissionregistrationv1.MatchPolicyType
	if policy.Spec.MatchConstraints.MatchPolicy == nil {
		matchPolicy = admissionregistrationv1.Equivalent
	} else {
		matchPolicy = *policy.Spec.MatchConstraints.MatchPolicy
	}

	newMatcher := matchconditions.NewMatcher(compiler.CompileMatchConditions(optionalVars), policy.Spec.FailurePolicy, "", string(matchPolicy), "")
	validator := validating.NewValidator(
		compiler.CompileValidations(optionalVars),
		newMatcher,
		compiler.CompileAuditAnnotationsExpressions(optionalVars),
		compiler.CompileMessageExpressions(optionalVars),
		policy.Spec.FailurePolicy,
	)
	versionedAttr, _ := admission.NewVersionedAttributes(a, a.GetKind(), nil)
	validateResult := validator.Validate(context.TODO(), a.GetResource(), versionedAttr, parameterResource, namespace, celconfig.RuntimeCELCostBudget, nil)

	// no validations are returned if match conditions aren't met
	if datautils.DeepEqual(validateResult, validating.ValidateResult{}) {
		ruleResp = engineapi.RuleSkip(policy.GetName(), engineapi.Validation, "match conditions aren't met", nil)
	} else {
		isPass := true
		for _, policyDecision := range validateResult.Decisions {
			if policyDecision.Evaluation == validating.EvalError {
				isPass = false
				vapLogger.Error(nil, "validation evaluation error for validatingadmissionpolicy", "policy", policy.GetName(), "resource", fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName()), "message", policyDecision.Message)
				ruleResp = engineapi.RuleError(policy.GetName(), engineapi.Validation, policyDecision.Message, nil, nil)
				break
			} else if policyDecision.Action == validating.ActionDeny {
				isPass = false
				vapLogger.V(2).Info("validation denied for validatingadmissionpolicy", "policy", policy.GetName(), "resource", fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName()), "message", policyDecision.Message)
				ruleResp = engineapi.RuleFail(policy.GetName(), engineapi.Validation, policyDecision.Message, nil)
				break
			}
		}

		if isPass {
			ruleResp = engineapi.RulePass(policy.GetName(), engineapi.Validation, "", nil)
		}
	}

	if binding != nil {
		ruleResp = ruleResp.WithVAPBinding(binding)
	}
	policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
	engineResponse = engineResponse.WithPolicyResponse(policyResp)

	return engineResponse, nil
}
