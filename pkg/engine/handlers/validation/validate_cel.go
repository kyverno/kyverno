package validation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	celutils "github.com/kyverno/kyverno/pkg/utils/cel"
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
	compiler, err := celutils.NewCompiler(validations, auditAnnotations, matchConditions, variables)
	if err != nil {
		return resource, handlers.WithError(rule, engineapi.Validation, "Error while creating composited compiler", err)
	}
	compiler.CompileVariables(optionalVars)
	filter := compiler.CompileValidateExpressions(optionalVars)
	messageExpressionfilter := compiler.CompileMessageExpressions(optionalVars)
	auditAnnotationFilter := compiler.CompileAuditAnnotationsExpressions(optionalVars)
	matchConditionFilter := compiler.CompileMatchExpressions(optionalVars)

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
