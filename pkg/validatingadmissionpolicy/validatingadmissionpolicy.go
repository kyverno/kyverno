package validatingadmissionpolicy

import (
	"context"
	"fmt"
	"strings"
	"time"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	celutils "github.com/kyverno/kyverno/pkg/utils/cel"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/validatingadmissionpolicy"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

func GetKinds(policy v1alpha1.ValidatingAdmissionPolicy) []string {
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

func Validate(policy v1alpha1.ValidatingAdmissionPolicy, resource unstructured.Unstructured) engineapi.EngineResponse {
	resPath := fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
	logger.V(3).Info("applying policy on resource", "policy", policy.GetName(), "resource", resPath)

	startTime := time.Now()

	validations := policy.Spec.Validations
	auditAnnotations := policy.Spec.AuditAnnotations
	matchConditions := policy.Spec.MatchConditions
	variables := policy.Spec.Variables

	hasParam := policy.Spec.ParamKind != nil

	var failPolicy admissionregistrationv1.FailurePolicyType
	if policy.Spec.FailurePolicy == nil {
		failPolicy = admissionregistrationv1.Fail
	} else {
		failPolicy = admissionregistrationv1.FailurePolicyType(*policy.Spec.FailurePolicy)
	}

	var matchPolicy v1alpha1.MatchPolicyType
	if policy.Spec.MatchConstraints.MatchPolicy == nil {
		matchPolicy = v1alpha1.Equivalent
	} else {
		matchPolicy = *policy.Spec.MatchConstraints.MatchPolicy
	}

	engineResponse := engineapi.NewEngineResponse(resource, engineapi.NewValidatingAdmissionPolicy(policy), nil)
	policyResp := engineapi.NewPolicyResponse()
	var ruleResp *engineapi.RuleResponse

	optionalVars := cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: false}

	// compile CEL expressions
	compiler, err := celutils.NewCompiler(validations, auditAnnotations, matchConditions, variables)
	if err != nil {
		ruleResp = engineapi.RuleError(policy.GetName(), engineapi.Validation, "Error creating composited compiler", err)
		policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
		engineResponse = engineResponse.WithPolicyResponse(policyResp)
		return engineResponse
	}
	compiler.CompileVariables(optionalVars)
	filter := compiler.CompileValidateExpressions(optionalVars)
	messageExpressionfilter := compiler.CompileMessageExpressions(optionalVars)
	auditAnnotationFilter := compiler.CompileAuditAnnotationsExpressions(optionalVars)
	matchConditionFilter := compiler.CompileMatchExpressions(optionalVars)

	newMatcher := matchconditions.NewMatcher(matchConditionFilter, &failPolicy, "", string(matchPolicy), "")
	validator := validatingadmissionpolicy.NewValidator(filter, newMatcher, auditAnnotationFilter, messageExpressionfilter, nil)

	admissionAttributes := admission.NewAttributesRecord(
		resource.DeepCopyObject(),
		nil, resource.GroupVersionKind(),
		resource.GetNamespace(),
		resource.GetName(),
		schema.GroupVersionResource{},
		"",
		admission.Create,
		nil,
		false,
		nil,
	)
	versionedAttr, _ := admission.NewVersionedAttributes(admissionAttributes, admissionAttributes.GetKind(), nil)
	validateResult := validator.Validate(context.TODO(), schema.GroupVersionResource{}, versionedAttr, nil, nil, celconfig.RuntimeCELCostBudget, nil)

	isPass := true
	for _, policyDecision := range validateResult.Decisions {
		if policyDecision.Evaluation == validatingadmissionpolicy.EvalError {
			isPass = false
			ruleResp = engineapi.RuleError(policy.GetName(), engineapi.Validation, policyDecision.Message, nil)
			break
		} else if policyDecision.Action == validatingadmissionpolicy.ActionDeny {
			isPass = false
			ruleResp = engineapi.RuleFail(policy.GetName(), engineapi.Validation, policyDecision.Message)
			break
		}
	}

	if isPass {
		ruleResp = engineapi.RulePass(policy.GetName(), engineapi.Validation, "")
	}
	policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
	engineResponse = engineResponse.WithPolicyResponse(policyResp)

	return engineResponse
}
