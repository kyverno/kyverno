package common

import (
	"context"
	"fmt"
	"time"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/validatingadmissionpolicy"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

type ValidatingAdmissionPolicies struct{}

func (p *ValidatingAdmissionPolicies) ApplyPolicyOnResource(c ApplyPolicyConfig) ([]engineapi.EngineResponse, error) {
	var engineResponses []engineapi.EngineResponse

	resPath := fmt.Sprintf("%s/%s/%s", c.Resource.GetNamespace(), c.Resource.GetKind(), c.Resource.GetName())
	log.V(3).Info("applying policy on resource", "policy", c.ValidatingAdmissionPolicy.GetName(), "resource", resPath)

	startTime := time.Now()

	validations := c.ValidatingAdmissionPolicy.Spec.Validations
	var expressions, messageExpressions []cel.ExpressionAccessor

	for _, validation := range validations {
		var reason metav1.StatusReason
		if validation.Reason == nil {
			reason = metav1.StatusReasonInvalid
		} else {
			reason = *validation.Reason
		}

		var message string
		if validation.Message != "" && validation.MessageExpression != "" {
			message = validation.MessageExpression
		} else if validation.Message != "" {
			message = validation.Message
		} else if validation.MessageExpression != "" {
			message = validation.MessageExpression
		} else {
			message = fmt.Sprintf("error: failed to create %s: %s \"%s\" is forbidden: ValidatingAdmissionPolicy '%s' denied request: failed expression: %s", c.Resource.GetKind(), c.Resource.GetAPIVersion(), c.Resource.GetName(), c.ValidatingAdmissionPolicy.Name, validation.Expression)
		}

		condition := &validatingadmissionpolicy.ValidationCondition{
			Expression: validation.Expression,
			Message:    message,
			Reason:     &reason,
		}

		messageCondition := &validatingadmissionpolicy.MessageExpressionCondition{
			MessageExpression: validation.MessageExpression,
		}

		expressions = append(expressions, condition)
		messageExpressions = append(messageExpressions, messageCondition)
	}

	hasParams := c.ValidatingAdmissionPolicy.Spec.ParamKind != nil

	filterCompiler := cel.NewFilterCompiler()
	filter := filterCompiler.Compile(expressions, cel.OptionalVariableDeclarations{HasParams: hasParams, HasAuthorizer: false}, celconfig.PerCallLimit)
	messageExpressionfilter := filterCompiler.Compile(messageExpressions, cel.OptionalVariableDeclarations{HasParams: hasParams, HasAuthorizer: false}, celconfig.PerCallLimit)

	admissionAttributes := admission.NewAttributesRecord(c.Resource.DeepCopyObject(), nil, c.Resource.GroupVersionKind(), c.Resource.GetNamespace(), c.Resource.GetName(), schema.GroupVersionResource{}, "", admission.Create, nil, false, nil)
	versionedAttr, _ := admission.NewVersionedAttributes(admissionAttributes, admissionAttributes.GetKind(), nil)

	ctx := context.TODO()
	failPolicy := admissionregistrationv1.FailurePolicyType(*c.ValidatingAdmissionPolicy.Spec.FailurePolicy)

	matchConditions := c.ValidatingAdmissionPolicy.Spec.MatchConditions
	var matchExpressions []cel.ExpressionAccessor

	for _, expression := range matchConditions {
		condition := &matchconditions.MatchCondition{
			Name:       expression.Name,
			Expression: expression.Expression,
		}
		matchExpressions = append(matchExpressions, condition)
	}

	matchFilter := filterCompiler.Compile(matchExpressions, cel.OptionalVariableDeclarations{HasParams: hasParams, HasAuthorizer: false}, celconfig.PerCallLimit)

	var matchPolicy *v1alpha1.MatchPolicyType
	if c.ValidatingAdmissionPolicy.Spec.MatchConstraints.MatchPolicy == nil {
		equivalent := v1alpha1.Equivalent
		matchPolicy = &equivalent
	} else {
		matchPolicy = c.ValidatingAdmissionPolicy.Spec.MatchConstraints.MatchPolicy
	}

	newMatcher := matchconditions.NewMatcher(matchFilter, nil, &failPolicy, string(*matchPolicy), "")

	auditAnnotations := c.ValidatingAdmissionPolicy.Spec.AuditAnnotations
	var auditExpressions []cel.ExpressionAccessor

	for _, expression := range auditAnnotations {
		condition := &validatingadmissionpolicy.AuditAnnotationCondition{
			Key:             expression.Key,
			ValueExpression: expression.ValueExpression,
		}
		auditExpressions = append(auditExpressions, condition)
	}
	auditAnnotationFilter := filterCompiler.Compile(auditExpressions, cel.OptionalVariableDeclarations{HasParams: hasParams, HasAuthorizer: false}, celconfig.PerCallLimit)

	validator := validatingadmissionpolicy.NewValidator(filter, newMatcher, auditAnnotationFilter, messageExpressionfilter, &failPolicy, nil)
	validateResult := validator.Validate(ctx, versionedAttr, nil, celconfig.RuntimeCELCostBudget)

	engineResponse := engineapi.NewEngineResponseWithValidatingAdmissionPolicy(*c.Resource, c.ValidatingAdmissionPolicy, nil)
	policyResp := engineapi.NewPolicyResponse()
	var ruleResp *engineapi.RuleResponse
	isPass := true

	for _, policyDecision := range validateResult.Decisions {
		if policyDecision.Evaluation == validatingadmissionpolicy.EvalError {
			isPass = false
			c.Rc.Error++
			ruleResp = engineapi.RuleError(c.ValidatingAdmissionPolicy.GetName(), engineapi.Validation, policyDecision.Message, nil)
			break
		} else if policyDecision.Action == validatingadmissionpolicy.ActionDeny {
			isPass = false
			c.Rc.Fail++
			ruleResp = engineapi.RuleFail(c.ValidatingAdmissionPolicy.GetName(), engineapi.Validation, policyDecision.Message)
			break
		}
	}

	if isPass {
		c.Rc.Pass++
		ruleResp = engineapi.RulePass(c.ValidatingAdmissionPolicy.GetName(), engineapi.Validation, "")
	}

	policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)

	engineResponse = engineResponse.WithPolicyResponse(policyResp)
	engineResponses = append(engineResponses, engineResponse)

	return engineResponses, nil
}
