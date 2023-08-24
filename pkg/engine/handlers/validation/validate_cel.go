package validation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// get resource's name, namespace, GroupVersionResource, and GroupVersionKind
	gvr := schema.GroupVersionResource(policyContext.RequestResource())
	gvk := resource.GroupVersionKind()
	namespaceName := resource.GetNamespace()
	resourceName := resource.GetName()

	object := resource.DeepCopyObject()
	// in case of update request, set the oldObject to the current resource before it gets updated
	var oldObject, versionedParams runtime.Object
	oldResource := policyContext.OldResource()
	if oldResource.Object == nil {
		oldObject = nil
	} else {
		oldObject = oldResource.DeepCopyObject()
	}

	// check if the rule uses parameter resources
	hasParam := rule.Validation.CEL.HasParam()
	// extract preconditions written as CEL expressions
	matchConditions := rule.CELPreconditions
	// extract CEL expressions used in validations and audit annotations
	variables := rule.Validation.CEL.Variables
	validations := rule.Validation.CEL.Expressions
	auditAnnotations := rule.Validation.CEL.AuditAnnotations

	matchExpressions := convertMatchExpressions(matchConditions)
	validateExpressions := convertValidations(validations)
	messageExpressions := convertMessageExpressions(validations)
	auditExpressions := convertAuditAnnotations(auditAnnotations)
	variableExpressions := convertVariables(variables)

	// get the parameter resource if exists
	if hasParam && h.client != nil {
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

	optionalVars := cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: false}

	// compile CEL expressions
	compositedCompiler, err := cel.NewCompositedCompiler(environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion()))
	if err != nil {
		return resource, handlers.WithError(rule, engineapi.Validation, "Error while creating composited compiler", err)
	}
	compositedCompiler.CompileAndStoreVariables(variableExpressions, optionalVars, environment.StoredExpressions)
	filter := compositedCompiler.Compile(validateExpressions, optionalVars, environment.StoredExpressions)
	messageExpressionfilter := compositedCompiler.Compile(messageExpressions, optionalVars, environment.StoredExpressions)
	auditAnnotationFilter := compositedCompiler.Compile(auditExpressions, optionalVars, environment.StoredExpressions)
	matchConditionFilter := compositedCompiler.Compile(matchExpressions, optionalVars, environment.StoredExpressions)

	// newMatcher will be used to check if the incoming resource matches the CEL preconditions
	newMatcher := matchconditions.NewMatcher(matchConditionFilter, nil, "", "", "")
	// newValidator will be used to validate CEL expressions against the incoming object
	validator := validatingadmissionpolicy.NewValidator(filter, newMatcher, auditAnnotationFilter, messageExpressionfilter, nil)

	var namespace *corev1.Namespace
	// Special case, the namespace object has the namespace of itself.
	// unset it if the incoming object is a namespace
	if gvk.Kind == "Namespace" && gvk.Version == "v1" && gvk.Group == "" {
		namespaceName = ""
	}
	if namespaceName != "" && h.client != nil {
		namespace, err = h.client.GetNamespace(ctx, namespaceName, metav1.GetOptions{})
		if err != nil {
			return resource, handlers.WithResponses(
				engineapi.RuleError(rule.Name, engineapi.Validation, "Error getting the resource's namespace", err),
			)
		}
	}

	admissionAttributes := admission.NewAttributesRecord(object, oldObject, gvk, namespaceName, resourceName, gvr, "", admission.Operation(policyContext.Operation()), nil, false, nil)
	versionedAttr, _ := admission.NewVersionedAttributes(admissionAttributes, admissionAttributes.GetKind(), nil)
	// validate the incoming object against the rule
	validateResult := validator.Validate(ctx, gvr, versionedAttr, versionedParams, namespace, celconfig.RuntimeCELCostBudget, nil)

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

func convertValidations(inputValidations []admissionregistrationv1alpha1.Validation) []cel.ExpressionAccessor {
	celExpressionAccessor := make([]cel.ExpressionAccessor, len(inputValidations))
	for i, validation := range inputValidations {
		validation := validatingadmissionpolicy.ValidationCondition{
			Expression: validation.Expression,
			Message:    validation.Message,
			Reason:     validation.Reason,
		}
		celExpressionAccessor[i] = &validation
	}
	return celExpressionAccessor
}

func convertMessageExpressions(inputValidations []admissionregistrationv1alpha1.Validation) []cel.ExpressionAccessor {
	celExpressionAccessor := make([]cel.ExpressionAccessor, len(inputValidations))
	for i, validation := range inputValidations {
		if validation.MessageExpression != "" {
			condition := validatingadmissionpolicy.MessageExpressionCondition{
				MessageExpression: validation.MessageExpression,
			}
			celExpressionAccessor[i] = &condition
		}
	}
	return celExpressionAccessor
}

func convertAuditAnnotations(inputValidations []admissionregistrationv1alpha1.AuditAnnotation) []cel.ExpressionAccessor {
	celExpressionAccessor := make([]cel.ExpressionAccessor, len(inputValidations))
	for i, validation := range inputValidations {
		validation := validatingadmissionpolicy.AuditAnnotationCondition{
			Key:             validation.Key,
			ValueExpression: validation.ValueExpression,
		}
		celExpressionAccessor[i] = &validation
	}
	return celExpressionAccessor
}

func convertMatchExpressions(matchExpressions []admissionregistrationv1.MatchCondition) []cel.ExpressionAccessor {
	celExpressionAccessor := make([]cel.ExpressionAccessor, len(matchExpressions))
	for i, condition := range matchExpressions {
		condition := matchconditions.MatchCondition{
			Name:       condition.Name,
			Expression: condition.Expression,
		}
		celExpressionAccessor[i] = &condition
	}
	return celExpressionAccessor
}

func convertVariables(variables []admissionregistrationv1alpha1.Variable) []cel.NamedExpressionAccessor {
	namedExpressions := make([]cel.NamedExpressionAccessor, len(variables))
	for i, variable := range variables {
		namedExpressions[i] = &validatingadmissionpolicy.Variable{
			Name:       variable.Name,
			Expression: variable.Expression,
		}
	}
	return namedExpressions
}
