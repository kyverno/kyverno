package validation

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/client-go/tools/cache"
)

type celEvalStatus int

const (
	celEvalPass celEvalStatus = iota
	celEvalFail
	celEvalSkip
	celEvalError
)

type celEvalResult struct {
	status  celEvalStatus
	message string
}

func processCELValidationResults(validationResults []validating.ValidateResult) celEvalResult {
	for _, validationResult := range validationResults {
		if datautils.DeepEqual(validationResult, validating.ValidateResult{}) {
			return celEvalResult{status: celEvalSkip, message: "cel preconditions not met"}
		}

		for _, decision := range validationResult.Decisions {
			switch decision.Action {
			case validating.ActionAdmit:
				if decision.Evaluation == validating.EvalError {
					return celEvalResult{status: celEvalError, message: decision.Message}
				}
			case validating.ActionDeny:
				return celEvalResult{status: celEvalFail, message: decision.Message}
			}
		}
	}

	return celEvalResult{status: celEvalPass}
}

type validateCELHandler struct {
	client    engineapi.Client
	isCluster bool
}

func NewValidateCELHandler(client engineapi.Client, isCluster bool) (handlers.Handler, error) {
	return validateCELHandler{
		client:    client,
		isCluster: isCluster,
	}, nil
}

func (h validateCELHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	_ engineapi.EngineContextLoader,
	exceptions []*kyvernov2.PolicyException,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	// check if there are policy exceptions that match the incoming resource
	matchedExceptions := engineutils.MatchesException(h.client, exceptions, policyContext, h.isCluster, logger)
	if len(matchedExceptions) > 0 {
		exceptions := make([]engineapi.GenericException, 0, len(matchedExceptions))
		var keys []string
		for i, exception := range matchedExceptions {
			key, err := cache.MetaNamespaceKeyFunc(&matchedExceptions[i])
			if err != nil {
				logger.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
				return resource, handlers.WithError(rule, engineapi.Validation, "failed to compute exception key", err)
			}
			keys = append(keys, key)
			exceptions = append(exceptions, engineapi.NewPolicyException(&exception))
		}

		logger.V(3).Info("policy rule is skipped due to policy exceptions", "exceptions", keys)
		return resource, handlers.WithResponses(
			engineapi.RuleSkip(rule.Name, engineapi.Validation, "rule is skipped due to policy exceptions"+strings.Join(keys, ", "), rule.ReportProperties).WithExceptions(exceptions),
		)
	}

	// check if a corresponding validating admission policy is generated
	vapStatus := policyContext.Policy().GetStatus().ValidatingAdmissionPolicy
	if vapStatus.Generated {
		logger.V(3).Info("skipping CEL validation due to the generation of its corresponding ValidatingAdmissionPolicy")
		return resource, nil
	}

	// get resource's name, namespace, GroupVersionResource, and GroupVersionKind
	gvr := schema.GroupVersionResource(policyContext.RequestResource())
	gvk, _ := policyContext.ResourceKind()
	policyKind := policyContext.Policy().GetKind()
	policyName := policyContext.Policy().GetName()

	// in case of UPDATE requests, set the oldObject to the current resource before it gets updated
	var object, oldObject runtime.Object
	oldResource := policyContext.OldResource()
	if oldResource.Object == nil {
		oldObject = nil
	} else {
		oldResource = *oldResource.DeepCopy()
		oldResource.SetGroupVersionKind(gvk)
		oldObject = oldResource.DeepCopyObject()
	}

	var ns, name string
	// in case of DELETE request, get the name and the namespace from the old object
	if resource.Object == nil {
		ns = oldResource.GetNamespace()
		name = oldResource.GetName()
		object = nil
	} else {
		ns = resource.GetNamespace()
		name = resource.GetName()
		resource = *resource.DeepCopy()
		resource.SetGroupVersionKind(gvk)
		object = resource.DeepCopyObject()
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
	compiler, err := admissionpolicy.NewCompiler(matchConditions, variables)
	if err != nil {
		return resource, handlers.WithError(rule, engineapi.Validation, "Error while creating composited compiler", err)
	}
	compiler.WithValidations(validations)
	compiler.WithAuditAnnotations(auditAnnotations)
	compiler.CompileVariables(optionalVars)
	filter := compiler.CompileValidations(optionalVars)
	messageExpressionfilter := compiler.CompileMessageExpressions(expressionOptionalVars)
	auditAnnotationFilter := compiler.CompileAuditAnnotationsExpressions(optionalVars)
	matchConditionFilter := compiler.CompileMatchConditions(optionalVars)

	// newMatcher will be used to check if the incoming resource matches the CEL preconditions
	newMatcher := matchconditions.NewMatcher(matchConditionFilter, nil, policyKind, "", policyName)
	// newValidator will be used to validate CEL expressions against the incoming object
	validator := validating.NewValidator(filter, newMatcher, auditAnnotationFilter, messageExpressionfilter, nil, nil)

	var namespace *corev1.Namespace
	// Special case, the namespace object has the namespace of itself.
	// unset it if the incoming object is a namespace
	if gvk.Kind == "Namespace" && gvk.Version == "v1" && gvk.Group == "" {
		ns = ""
	}
	if ns != "" {
		if h.client != nil && h.isCluster {
			namespace, err = h.client.GetNamespace(ctx, ns, metav1.GetOptions{})
			if err != nil {
				return resource, handlers.WithResponses(
					engineapi.RuleError(rule.Name, engineapi.Validation, "Error getting the resource's namespace", err, rule.ReportProperties),
				)
			}
		} else {
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			}
		}
	}

	authorizer := internal.NewAuthorizer(h.client, gvk)
	newEval, err := h.evaluateCEL(
		ctx,
		gvr,
		gvk,
		object,
		oldObject,
		ns,
		name,
		admission.Operation(policyContext.Operation()),
		policyContext,
		validator,
		namespace,
		authorizer,
		hasParam,
		rule,
	)
	if err != nil {
		msg := "error while creating versioned attributes"
		if hasParam {
			msg = "error in parameterized resource"
		}
		return resource, handlers.WithError(rule, engineapi.Validation, msg, err)
	}

	switch newEval.status {
	case celEvalSkip:
		return resource, handlers.WithResponses(
			engineapi.RuleSkip(rule.Name, engineapi.Validation, newEval.message, rule.ReportProperties),
		)
	case celEvalError:
		return resource, handlers.WithResponses(
			engineapi.RuleError(rule.Name, engineapi.Validation, newEval.message, nil, rule.ReportProperties),
		)
	case celEvalFail:
		var action kyvernov1.ValidationFailureAction
		if rule.Validation.FailureAction != nil {
			action = *rule.Validation.FailureAction
		} else {
			action = policyContext.Policy().GetSpec().ValidationFailureAction
		}

		if action.Enforce() && engineutils.IsUpdateRequest(policyContext) && rule.HasValidateAllowExistingViolations() {
			if oldResource.Object != nil {
				if matchResource(logger, oldResource, rule, policyContext.NamespaceLabels(), policyContext.Policy().GetNamespace(), kyvernov1.Create, policyContext.JSONContext()) {
					oldEval, err := h.evaluateCEL(
						ctx,
						gvr,
						gvk,
						oldObject,
						nil,
						ns,
						name,
						admission.Create,
						policyContext,
						validator,
						namespace,
						authorizer,
						hasParam,
						rule,
					)
					if err != nil {
						logger.V(4).Info("warning: failed to validate old object", "rule", rule.Name, "error", err.Error())
						return resource, handlers.WithResponses(
							engineapi.RuleSkip(rule.Name, engineapi.Validation, "failed to validate old object", rule.ReportProperties),
						)
					}

					if oldEval.status == celEvalFail {
						logger.V(2).Info("warning: skipping the rule evaluation as pre-existing violations are allowed", "rule", rule.Name)
						return resource, handlers.WithResponses(
							engineapi.RuleSkip(rule.Name, engineapi.Validation, "skipping the rule evaluation as pre-existing violations are allowed", rule.ReportProperties),
						)
					}
				}
			}
		}

		return resource, handlers.WithResponses(
			engineapi.RuleFail(rule.Name, engineapi.Validation, newEval.message, rule.ReportProperties),
		)
	}

	msg := fmt.Sprintf("Validation rule '%s' passed.", rule.Name)
	return resource, handlers.WithResponses(
		engineapi.RulePass(rule.Name, engineapi.Validation, msg, rule.ReportProperties),
	)
}

func (h validateCELHandler) evaluateCEL(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	gvk schema.GroupVersionKind,
	object, oldObject runtime.Object,
	ns, name string,
	operation admission.Operation,
	policyContext engineapi.PolicyContext,
	validator validating.Validator,
	namespace *corev1.Namespace,
	authorizer internal.Authorizer,
	hasParam bool,
	rule kyvernov1.Rule,
) (celEvalResult, error) {
	requestInfo := policyContext.AdmissionInfo()
	userInfo := admissionpolicy.NewUser(requestInfo.AdmissionUserInfo)
	attr := admission.NewAttributesRecord(object, oldObject, gvk, ns, name, gvr, "", operation, nil, false, &userInfo)
	o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())
	versionedAttr, err := admission.NewVersionedAttributes(attr, attr.GetKind(), o)
	if err != nil {
		return celEvalResult{}, err
	}

	var validationResults []validating.ValidateResult
	if hasParam {
		paramKind := rule.Validation.CEL.ParamKind
		paramRef := rule.Validation.CEL.ParamRef

		params, err := admissionpolicy.CollectParams(ctx, h.client, paramKind, paramRef, ns)
		if err != nil {
			return celEvalResult{}, err
		}

		for _, param := range params {
			validationResults = append(validationResults, validator.Validate(ctx, gvr, versionedAttr, param, namespace, celconfig.RuntimeCELCostBudget, &authorizer))
		}
	} else {
		validationResults = append(validationResults, validator.Validate(ctx, gvr, versionedAttr, nil, namespace, celconfig.RuntimeCELCostBudget, &authorizer))
	}

	return processCELValidationResults(validationResults), nil
}
