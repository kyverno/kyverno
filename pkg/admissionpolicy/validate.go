package admissionpolicy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
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

func GetKinds(policy admissionregistrationv1beta1.ValidatingAdmissionPolicy) []string {
	var kindList []string

	matchResources := policy.Spec.MatchConstraints
	for _, rule := range matchResources.ResourceRules {
		group := rule.APIGroups[0]
		version := rule.APIVersions[0]
		for _, resource := range rule.Resources {
			isSubresource := kubeutils.IsSubresource(resource)
			if isSubresource {
				parts := strings.Split(resource, "/")

				kind := cases.Title(language.English, cases.NoLower).String(parts[0])
				kind, _ = strings.CutSuffix(kind, "s")
				subresource := parts[1]

				if group == "" {
					kindList = append(kindList, strings.Join([]string{version, kind, subresource}, "/"))
				} else {
					kindList = append(kindList, strings.Join([]string{group, version, kind, subresource}, "/"))
				}
			} else {
				resource = cases.Title(language.English, cases.NoLower).String(resource)
				resource, _ = strings.CutSuffix(resource, "s")
				kind := resource

				if group == "" {
					kindList = append(kindList, strings.Join([]string{version, kind}, "/"))
				} else {
					kindList = append(kindList, strings.Join([]string{group, version, kind}, "/"))
				}
			}
		}
	}

	return kindList
}

func Validate(
	policyData PolicyData,
	resource unstructured.Unstructured,
	namespaceSelectorMap map[string]map[string]string,
	client dclient.Interface,
	isCluster bool,
) ([]engineapi.EngineResponse, error) {
	resPath := fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
	policy := policyData.definition
	bindings := policyData.bindings
	var ers []engineapi.EngineResponse

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
				Name:   namespaceName,
				Labels: namespaceSelectorMap[namespaceName],
			},
		}
	}

	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, resource.GroupVersionKind(), resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)

	if len(bindings) == 0 {
		isMatch, err := matches(a, namespaceSelectorMap, *policy.Spec.MatchConstraints)
		if err != nil {
			return nil, err
		}
		if !isMatch {
			return nil, nil
		}
		logger.V(3).Info("validate resource %s against policy %s", resPath, policy.GetName())

		engineResponse, err := validateResource(policy, nil, resource, namespace, a, nil)
		if err != nil {
			return nil, err
		}
		ers = append(ers, engineResponse)
		return ers, nil
	}

	if isCluster {
		nsLister := NewCustomNamespaceLister(client)
		matcher := generic.NewPolicyMatcher(matching.NewMatcher(nsLister, client.GetKubeClient()))

		// convert policy from v1beta1 to v1
		v1policy := ConvertValidatingAdmissionPolicy(policy)

		// construct admission attributes
		gvr, err := client.Discovery().GetGVRFromGVK(gvk)
		if err != nil {
			return nil, err
		}
		a = admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)

		// check if policy matches the incoming resource
		o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())
		isMatch, _, _, err := matcher.DefinitionMatches(a, o, validating.NewValidatingAdmissionPolicyAccessor(&v1policy))
		if err != nil {
			return nil, err
		}
		if !isMatch {
			return nil, nil
		}

		if namespaceName != "" {
			namespace, err = client.GetKubeClient().CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
		}

		for i, binding := range bindings {
			// convert policy binding from v1alpha1 to v1
			v1binding := ConvertValidatingAdmissionPolicyBinding(binding)
			isMatch, err := matcher.BindingMatches(a, o, validating.NewValidatingAdmissionPolicyBindingAccessor(&v1binding))
			if err != nil {
				return nil, err
			}
			if !isMatch {
				continue
			}
			if binding.Spec.ParamRef != nil {
				params, err := CollectParams(context.TODO(), adapters.Client(client), policy.Spec.ParamKind, binding.Spec.ParamRef, resource.GetNamespace())
				if err != nil {
					return nil, err
				}
				logger.V(3).Info("validate resource %s against policy %s with binding %s", resPath, policy.GetName(), binding.GetName())
				for _, p := range params {
					engineResponse, err := validateResource(policy, &bindings[i], resource, namespace, a, p)
					if err != nil {
						continue // is this correct ?
					}
					ers = append(ers, engineResponse)
				}
				return ers, nil
			}

			engineResponse, err := validateResource(policy, &bindings[i], resource, namespace, a, nil)
			if err != nil {
				continue // is this correct ?
			}
			ers = append(ers, engineResponse)
			return ers, nil
		}
	} else {
		for i, binding := range bindings {
			// check if its a global binding
			if binding.Spec.MatchResources != nil {
				isMatch, err := matches(a, namespaceSelectorMap, *binding.Spec.MatchResources)
				if err != nil {
					return nil, err
				}
				if !isMatch {
					continue
				}
			}

			if binding.Spec.ParamRef != nil {
				var params []runtime.Object
				for _, param := range policyData.params {
					unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(param)
					if err != nil {
						return nil, fmt.Errorf("failed to convert to unstructured: %w", err)
					}
					obj := &unstructured.Unstructured{Object: unstructuredMap}
					if binding.Spec.ParamRef.Selector != nil {
						labelSelector, err := metav1.LabelSelectorAsSelector(binding.Spec.ParamRef.Selector)
						if err != nil {
							return nil, fmt.Errorf("failed to convert LabelSelector: %w", err)
						}

						objLabels := obj.GetLabels()
						if labelSelector.Matches(labels.Set(objLabels)) {
							params = append(params, param)
						}
					} else {
						if obj.GetName() == binding.Spec.ParamRef.Name {
							params = append(params, param)
						}
					}
				}
				logger.V(3).Info("validate resource %s against policy %s with binding %s", resPath, policy.GetName(), binding.GetName())
				for _, p := range params {
					engineResponse, err := validateResource(policy, &bindings[i], resource, namespace, a, p)
					if err != nil {
						continue // is this correct ?
					}
					ers = append(ers, engineResponse)
				}
				return ers, nil
			}

			engineResponse, err := validateResource(policy, &bindings[i], resource, namespace, a, nil)
			if err != nil {
				continue // is this correct ?
			}
			// <<<<<<< HEAD:pkg/admissionpolicy/validate.go
			// 			if !isMatch {
			// 				continue
			// 			}
			// 			logger.V(3).Info("validate resource %s against policy %s with binding %s", resPath, policy.GetName(), binding.GetName())
			// 			return validateResource(policy, &bindings[i], resource, namespace, a)
			// =======
			ers = append(ers, engineResponse)
			return ers, nil
			// >>>>>>> vap-cli-params:pkg/validatingadmissionpolicy/validate.go
		}
	}

	return ers, nil
}

func CollectParams(ctx context.Context, client engineapi.Client, paramKind *admissionregistrationv1beta1.ParamKind, paramRef *admissionregistrationv1beta1.ParamRef, namespace string) ([]runtime.Object, error) {
	var params []runtime.Object

	apiVersion := paramKind.APIVersion // nil pointer ?
	kind := paramKind.Kind
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, fmt.Errorf("can't parse the parameter resource group version")
	}

	// If `paramKind` is cluster-scoped, then paramRef.namespace MUST be unset.
	// If `paramKind` is namespace-scoped, the namespace of the object being evaluated for admission will be used
	// when paramRef.namespace is left unset.
	var paramsNamespace string
	isNamespaced, err := client.IsNamespaced(gv.Group, gv.Version, kind)
	if err != nil {
		return nil, fmt.Errorf("failed to check if resource is namespaced or not (%w)", err)
	}

	// check if `paramKind` is namespace-scoped
	if isNamespaced {
		// set params namespace to the incoming object's namespace by default.
		paramsNamespace = namespace
		if paramRef.Namespace != "" {
			paramsNamespace = paramRef.Namespace
		} else if paramsNamespace == "" {
			return nil, fmt.Errorf("can't use namespaced paramRef to match cluster-scoped resources")
		}
	} else {
		// It isn't allowed to set namespace for cluster-scoped params
		if paramRef.Namespace != "" {
			return nil, fmt.Errorf("paramRef.namespace must not be provided for a cluster-scoped `paramKind`")
		}
	}

	if paramRef.Name != "" {
		param, err := client.GetResource(ctx, apiVersion, kind, paramsNamespace, paramRef.Name, "")
		if err != nil {
			return nil, err
		}
		return []runtime.Object{param}, nil
	} else if paramRef.Selector != nil {
		paramList, err := client.ListResource(ctx, apiVersion, kind, paramsNamespace, paramRef.Selector)
		if err != nil {
			return nil, err
		}
		for i := range paramList.Items {
			params = append(params, &paramList.Items[i])
		}
	}

	if len(params) == 0 && paramRef.ParameterNotFoundAction != nil && *paramRef.ParameterNotFoundAction == admissionregistrationv1beta1.DenyAction {
		return nil, fmt.Errorf("no params found")
	}

	return params, nil
}

func validateResource(
	policy admissionregistrationv1beta1.ValidatingAdmissionPolicy,
	binding *admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding,
	resource unstructured.Unstructured,
	namespace *corev1.Namespace,
	a admission.Attributes,
	param runtime.Object,
) (engineapi.EngineResponse, error) {
	startTime := time.Now()

	engineResponse := engineapi.NewEngineResponse(resource, engineapi.NewValidatingAdmissionPolicy(policy), nil)
	policyResp := engineapi.NewPolicyResponse()
	var ruleResp *engineapi.RuleResponse

	// compile CEL expressions
	matchConditions := ConvertMatchConditionsV1(policy.Spec.MatchConditions)
	compiler, err := NewCompiler(policy.Spec.Validations, policy.Spec.AuditAnnotations, matchConditions, policy.Spec.Variables)
	if err != nil {
		return engineResponse, err
	}
	hasParam := policy.Spec.ParamKind != nil
	optionalVars := cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: false}
	compiler.CompileVariables(optionalVars)

	var failPolicy admissionregistrationv1.FailurePolicyType
	if policy.Spec.FailurePolicy == nil {
		failPolicy = admissionregistrationv1.Fail
	} else {
		failPolicy = admissionregistrationv1.FailurePolicyType(*policy.Spec.FailurePolicy)
	}

	var matchPolicy admissionregistrationv1beta1.MatchPolicyType
	if policy.Spec.MatchConstraints.MatchPolicy == nil {
		matchPolicy = admissionregistrationv1beta1.Equivalent
	} else {
		matchPolicy = *policy.Spec.MatchConstraints.MatchPolicy
	}

	newMatcher := matchconditions.NewMatcher(compiler.CompileMatchExpressions(optionalVars), &failPolicy, "", string(matchPolicy), "")
	validator := validating.NewValidator(
		compiler.CompileValidateExpressions(optionalVars),
		newMatcher,
		compiler.CompileAuditAnnotationsExpressions(optionalVars),
		compiler.CompileMessageExpressions(optionalVars),
		&failPolicy,
	)
	versionedAttr, _ := admission.NewVersionedAttributes(a, a.GetKind(), nil)

	validateResult := validator.Validate(context.TODO(), a.GetResource(), versionedAttr, param, namespace, celconfig.RuntimeCELCostBudget, nil)

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
		ruleResp = ruleResp.WithBinding(binding)
	}
	policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
	engineResponse = engineResponse.WithPolicyResponse(policyResp)

	return engineResponse, nil
}
