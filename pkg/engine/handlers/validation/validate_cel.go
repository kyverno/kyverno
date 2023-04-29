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
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

type validateCELHandler struct{}

func NewValidateCELHandler() (handlers.Handler, error) {
	return validateCELHandler{}, nil
}

func (h validateCELHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	contextLoader engineapi.EngineContextLoader,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	if engineutils.IsDeleteRequest(policyContext) {
		logger.V(3).Info("skipping validation on deleted resource")
		return resource, nil
	}

	celValidations := rule.Validation.CEL
	oldResource := policyContext.OldResource()

	var object, oldObject runtime.Object
	object = resource.DeepCopyObject()
	if oldResource.Object == nil {
		oldObject = nil
	} else {
		oldObject = oldResource.DeepCopyObject()
	}

	var expressions, messageExpressions, auditExpressions []cel.ExpressionAccessor

	for _, cel := range celValidations {
		condition := &validatingadmissionpolicy.ValidationCondition{
			Expression: cel.Expression,
			Message:    cel.Message,
		}

		messageCondition := &validatingadmissionpolicy.MessageExpressionCondition{
			MessageExpression: cel.MessageExpression,
		}

		expressions = append(expressions, condition)
		messageExpressions = append(messageExpressions, messageCondition)

		for _, auditAnnotation := range cel.AuditAnnotations {
			auditCondition := &validatingadmissionpolicy.AuditAnnotationCondition{
				Key:             auditAnnotation.Key,
				ValueExpression: auditAnnotation.ValueExpression,
			}

			auditExpressions = append(auditExpressions, auditCondition)
		}
	}

	filterCompiler := cel.NewFilterCompiler()
	filter := filterCompiler.Compile(expressions, cel.OptionalVariableDeclarations{HasParams: false, HasAuthorizer: false}, celconfig.PerCallLimit)
	messageExpressionfilter := filterCompiler.Compile(messageExpressions, cel.OptionalVariableDeclarations{HasParams: false, HasAuthorizer: false}, celconfig.PerCallLimit)
	auditAnnotationFilter := filterCompiler.Compile(auditExpressions, cel.OptionalVariableDeclarations{HasParams: false, HasAuthorizer: false}, celconfig.PerCallLimit)

	validator := validatingadmissionpolicy.NewValidator(filter, nil, auditAnnotationFilter, messageExpressionfilter, nil, nil)

	admissionAttributes := admission.NewAttributesRecord(
		object,
		oldObject,
		resource.GroupVersionKind(),
		resource.GetNamespace(),
		resource.GetName(),
		schema.GroupVersionResource{},
		"",
		admission.Operation(policyContext.Operation()),
		nil,
		false,
		nil,
	)
	versionedAttr, _ := admission.NewVersionedAttributes(admissionAttributes, admissionAttributes.GetKind(), nil)
	validateResult := validator.Validate(ctx, versionedAttr, nil, celconfig.RuntimeCELCostBudget)

	for _, decision := range validateResult.Decisions {
		switch decision.Action {
		case validatingadmissionpolicy.ActionAdmit:
			if decision.Evaluation == validatingadmissionpolicy.EvalError {
				fmt.Println("error")
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
