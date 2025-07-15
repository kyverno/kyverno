package admissionpolicy

import (
	"context"
	"fmt"
	"strings"
	"time"

	celmatching "github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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

func GetKinds(matchResources *admissionregistrationv1.MatchResources, mapper meta.RESTMapper) ([]string, error) {
	if matchResources == nil {
		return nil, nil
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
						return kindList, err
					}
					kindList = append(kindList, kinds...)
				}
			}
		}
	}
	return kindList, nil
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
	isFake bool,
) (engineapi.EngineResponse, error) {
	resPath := fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
	policy := policyData.GetDefinition()
	bindings := policyData.GetBindings()
	engineResponse := engineapi.NewEngineResponse(resource, engineapi.NewValidatingAdmissionPolicy(policy), nil)

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
		matcher := celmatching.NewMatcher()
		isMatch, err := matcher.Match(
			&celmatching.MatchCriteria{
				Constraints: policy.Spec.MatchConstraints,
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
		vapLogger.V(3).Info("validate resource %s against policy %s", resPath, policy.GetName())
		return validateResource(policy, nil, resource, namespace, a)
	}

	if client != nil && !isFake {
		nsLister := NewCustomNamespaceLister(client)
		matcher := generic.NewPolicyMatcher(matching.NewMatcher(nsLister, client.GetKubeClient()))

		// check if policy matches the incoming resource
		o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())
		isMatch, _, _, err := matcher.DefinitionMatches(a, o, validating.NewValidatingAdmissionPolicyAccessor(policy))
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
			isMatch, err := matcher.BindingMatches(a, o, validating.NewValidatingAdmissionPolicyBindingAccessor(&binding))
			if err != nil {
				return engineResponse, err
			}
			if !isMatch {
				continue
			}

			vapLogger.V(3).Info("validate resource %s against policy %s with binding %s", resPath, policy.GetName(), binding.GetName())
			return validateResource(policy, &bindings[i], resource, namespace, a)
		}
	} else {
		matcher := celmatching.NewMatcher()
		for i, binding := range bindings {
			// check if the binding matches the incoming resource
			if binding.Spec.MatchResources != nil {
				bindingMatches, err := matcher.Match(
					&celmatching.MatchCriteria{
						Constraints: binding.Spec.MatchResources,
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
			vapLogger.V(3).Info("validate resource %s against policy %s with binding %s", resPath, policy.GetName(), binding.GetName())
			return validateResource(policy, &bindings[i], resource, namespace, a)
		}
	}

	return engineResponse, nil
}

func validateResource(
	policy *admissionregistrationv1.ValidatingAdmissionPolicy,
	binding *admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	resource unstructured.Unstructured,
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
	validateResult := validator.Validate(context.TODO(), a.GetResource(), versionedAttr, nil, namespace, celconfig.RuntimeCELCostBudget, nil)

	// no validations are returned if match conditions aren't met
	if datautils.DeepEqual(validateResult, validating.ValidateResult{}) {
		ruleResp = engineapi.RuleSkip(policy.GetName(), engineapi.Validation, "match conditions aren't met", nil)
	} else {
		isPass := true
		for _, policyDecision := range validateResult.Decisions {
			if policyDecision.Evaluation == validating.EvalError {
				isPass = false
				ruleResp = engineapi.RuleError(policy.GetName(), engineapi.Validation, policyDecision.Message, nil, nil)
				break
			} else if policyDecision.Action == validating.ActionDeny {
				isPass = false
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
