package validation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/validatingadmissionpolicy"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/cel/environment"
)

type validateCELHandler struct {
	client engineapi.Client
}

func NewValidateCELHandler(client engineapi.Client) (handlers.Handler, error) {
	return validateCELHandler{
		client: client,
	}, nil
}

func (h validateCELHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	_ engineapi.EngineContextLoader,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	if engineutils.IsDeleteRequest(policyContext) {
		logger.V(3).Info("skipping CEL validation on deleted resource")
		return resource, nil
	}

	oldResource := policyContext.OldResource()
	gvr := schema.GroupVersionResource(policyContext.RequestResource())

	var object, oldObject, versionedParams runtime.Object
	object = resource.DeepCopyObject()
	if oldResource.Object == nil {
		oldObject = nil
	} else {
		oldObject = oldResource.DeepCopyObject()
	}

	var expressions, messageExpressions, matchExpressions, auditExpressions []cel.ExpressionAccessor

	validations := rule.Validation.CEL.Expressions
	auditAnnotations := rule.Validation.CEL.AuditAnnotations

	// Get the parameter resource
	hasParam := rule.Validation.CEL.HasParam()

	if hasParam {
		paramKind := rule.Validation.CEL.GetParamKind()
		paramRef := rule.Validation.CEL.GetParamRef()

		apiVersion := paramKind.APIVersion
		kind := paramKind.Kind

		name := paramRef.Name
		namespace := paramRef.Namespace

		if namespace == "" {
			namespace = "default"
		}

		paramResource, err := h.client.GetResource(ctx, apiVersion, kind, namespace, name, "")
		if err != nil {
			return resource, handlers.WithError(rule, engineapi.Validation, "Error while getting the parameterized resource", err)
		}

		versionedParams = paramResource.DeepCopyObject()
	}

	for _, cel := range validations {
		condition := &validatingadmissionpolicy.ValidationCondition{
			Expression: cel.Expression,
			Message:    cel.Message,
		}

		messageCondition := &validatingadmissionpolicy.MessageExpressionCondition{
			MessageExpression: cel.MessageExpression,
		}

		expressions = append(expressions, condition)
		messageExpressions = append(messageExpressions, messageCondition)
	}

	for _, condition := range rule.CELPreconditions {
		matchCondition := &matchconditions.MatchCondition{
			Name:       condition.Name,
			Expression: condition.Expression,
		}

		matchExpressions = append(matchExpressions, matchCondition)
	}

	for _, auditAnnotation := range auditAnnotations {
		auditCondition := &validatingadmissionpolicy.AuditAnnotationCondition{
			Key:             auditAnnotation.Key,
			ValueExpression: auditAnnotation.ValueExpression,
		}

		auditExpressions = append(auditExpressions, auditCondition)
	}

	compositedCompiler, err := cel.NewCompositedCompiler(environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion()))
	if err != nil {
		return resource, handlers.WithError(rule, engineapi.Validation, "Error while creating composited complier", err)
	}
	filter := compositedCompiler.Compile(expressions, cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: false}, environment.StoredExpressions)
	messageExpressionfilter := compositedCompiler.Compile(messageExpressions, cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: false}, environment.StoredExpressions)
	auditAnnotationFilter := compositedCompiler.Compile(auditExpressions, cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: false}, environment.StoredExpressions)
	matchConditionFilter := compositedCompiler.Compile(matchExpressions, cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: false}, environment.StoredExpressions)

	newMatcher := matchconditions.NewMatcher(matchConditionFilter, nil, "", "", "")

	validator := validatingadmissionpolicy.NewValidator(filter, newMatcher, auditAnnotationFilter, messageExpressionfilter, nil)

	admissionAttributes := admission.NewAttributesRecord(
		object,
		oldObject,
		resource.GroupVersionKind(),
		resource.GetNamespace(),
		resource.GetName(),
		gvr,
		"",
		admission.Operation(policyContext.Operation()),
		nil,
		false,
		nil,
	)
	versionedAttr, _ := admission.NewVersionedAttributes(admissionAttributes, admissionAttributes.GetKind(), nil)
	validateResult := validator.Validate(ctx, gvr, versionedAttr, versionedParams, nil, celconfig.RuntimeCELCostBudget, nil)

	for _, decision := range validateResult.Decisions {
		switch decision.Action {
		case validatingadmissionpolicy.ActionAdmit:
			if decision.Evaluation == validatingadmissionpolicy.EvalError {
				return resource, handlers.WithResponses(
					engineapi.RuleError(rule.Name, engineapi.Validation, decision.Message, nil),
				)
			}
		case validatingadmissionpolicy.ActionDeny:
			return resource, handlers.WithResponses(
				engineapi.RuleFail(rule.Name, engineapi.Validation, decision.Message),
			)
		}
	}

	msg := fmt.Sprintf("Validation rule '%s' passed.", rule.Name)
	return resource, handlers.WithResponses(
		engineapi.RulePass(rule.Name, engineapi.Validation, msg),
	)
}
