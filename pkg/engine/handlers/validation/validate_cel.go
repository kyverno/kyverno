package validation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	celutils "github.com/kyverno/kyverno/pkg/utils/cel"
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
	// check if a corresponding validating admission policy is generated
	vapStatus := policyContext.Policy().GetStatus().ValidatingAdmissionPolicy
	if vapStatus.Generated {
		logger.V(3).Info("skipping CEL validation due to the generation of its corresponding ValidatingAdmissionPolicy")
		return resource, nil
	}

	// get resource's name, namespace, GroupVersionResource, and GroupVersionKind
	gvr := schema.GroupVersionResource(policyContext.RequestResource())
	gvk := resource.GroupVersionKind()
	namespaceName := resource.GetNamespace()
	resourceName := resource.GetName()
	resourceKind, _ := policyContext.ResourceKind()
	policyKind := policyContext.Policy().GetKind()
	policyName := policyContext.Policy().GetName()

	object := resource.DeepCopyObject()
	// in case of update request, set the oldObject to the current resource before it gets updated
	var oldObject runtime.Object
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
	for i := range validations {
		if validations[i].Message == "" {
			validations[i].Message = rule.Validation.Message
		}
	}
	auditAnnotations := rule.Validation.CEL.AuditAnnotations

	optionalVars := cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: true}
	expressionOptionalVars := cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: false}
	// compile CEL expressions
	compiler, err := celutils.NewCompiler(validations, auditAnnotations, matchConditions, variables)
	if err != nil {
		return resource, handlers.WithError(rule, engineapi.Validation, "Error while creating composited compiler", err)
	}
	compiler.CompileVariables(optionalVars)
	filter := compiler.CompileValidateExpressions(optionalVars)
	messageExpressionfilter := compiler.CompileMessageExpressions(expressionOptionalVars)
	auditAnnotationFilter := compiler.CompileAuditAnnotationsExpressions(optionalVars)
	matchConditionFilter := compiler.CompileMatchExpressions(optionalVars)

	// newMatcher will be used to check if the incoming resource matches the CEL preconditions
	newMatcher := matchconditions.NewMatcher(matchConditionFilter, nil, policyKind, "", policyName)
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

	requestInfo := policyContext.AdmissionInfo()
	userInfo := internal.NewUser(requestInfo.AdmissionUserInfo.Username, requestInfo.AdmissionUserInfo.UID, requestInfo.AdmissionUserInfo.Groups)
	admissionAttributes := admission.NewAttributesRecord(object, oldObject, gvk, namespaceName, resourceName, gvr, "", admission.Operation(policyContext.Operation()), nil, false, &userInfo)
	versionedAttr, _ := admission.NewVersionedAttributes(admissionAttributes, admissionAttributes.GetKind(), nil)
	authorizer := internal.NewAuthorizer(h.client, resourceKind)
	// validate the incoming object against the rule
	var validationResults []validatingadmissionpolicy.ValidateResult
	if hasParam {
		paramKind := rule.Validation.CEL.ParamKind
		paramRef := rule.Validation.CEL.ParamRef

		params, err := collectParams(ctx, h.client, paramKind, paramRef, namespaceName)
		if err != nil {
			return resource, handlers.WithResponses(
				engineapi.RuleError(rule.Name, engineapi.Validation, "error in parameterized resource", err),
			)
		}

		for _, param := range params {
			validationResults = append(validationResults, validator.Validate(ctx, gvr, versionedAttr, param, namespace, celconfig.RuntimeCELCostBudget, &authorizer))
		}
	} else {
		validationResults = append(validationResults, validator.Validate(ctx, gvr, versionedAttr, nil, namespace, celconfig.RuntimeCELCostBudget, &authorizer))
	}

	for _, validationResult := range validationResults {
		for _, decision := range validationResult.Decisions {
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
	}

	msg := fmt.Sprintf("Validation rule '%s' passed.", rule.Name)
	return resource, handlers.WithResponses(
		engineapi.RulePass(rule.Name, engineapi.Validation, msg),
	)
}

func collectParams(ctx context.Context, client engineapi.Client, paramKind *admissionregistrationv1alpha1.ParamKind, paramRef *admissionregistrationv1alpha1.ParamRef, namespace string) ([]runtime.Object, error) {
	var params []runtime.Object

	apiVersion := paramKind.APIVersion
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

	if len(params) == 0 && paramRef.ParameterNotFoundAction != nil && *paramRef.ParameterNotFoundAction == admissionregistrationv1alpha1.DenyAction {
		return nil, fmt.Errorf("no params found")
	}

	return params, nil
}
