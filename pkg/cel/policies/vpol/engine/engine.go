package engine

import (
	"context"
	"strings"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/tools/cache"
)

type (
	EngineRequest  = engine.EngineRequest
	EngineResponse = engine.EngineResponse
	Engine         = engine.Engine[policiesv1alpha1.ValidatingPolicy]
	Predicate      = func(policiesv1alpha1.ValidatingPolicy) bool
)

type engineImpl struct {
	provider   Provider
	nsResolver engine.NamespaceResolver
	matcher    matching.Matcher
}

func NewEngine(provider Provider, nsResolver engine.NamespaceResolver, matcher matching.Matcher) Engine {
	return &engineImpl{
		provider:   provider,
		nsResolver: nsResolver,
		matcher:    matcher,
	}
}

func (e *engineImpl) Handle(ctx context.Context, request EngineRequest, predicate Predicate) (EngineResponse, error) {
	var response EngineResponse
	// fetch compiled policies
	policies, err := e.provider.Fetch(ctx)
	if err != nil {
		return response, err
	}
	// if the request is for a json payload, handle it directly
	if request.JsonPayload != nil {
		response.Resource = request.JsonPayload
		for _, policy := range policies {
			response.Policies = append(response.Policies, e.handlePolicy(ctx, policy, request.JsonPayload.Object, nil, nil, nil, request.Context))
		}
		return response, nil
	}
	// load objects
	object, oldObject, err := admissionutils.ExtractResources(nil, request.Request)
	if err != nil {
		return response, err
	}
	response.Resource = &object
	if response.Resource.Object == nil {
		response.Resource = &oldObject
	}
	// default dry run
	dryRun := false
	if request.Request.DryRun != nil {
		dryRun = *request.Request.DryRun
	}
	// create admission attributes
	attr := admission.NewAttributesRecord(
		&object,
		&oldObject,
		schema.GroupVersionKind(request.Request.Kind),
		request.Request.Namespace,
		request.Request.Name,
		schema.GroupVersionResource(request.Request.Resource),
		request.Request.SubResource,
		admission.Operation(request.Request.Operation),
		nil,
		dryRun,
		// TODO
		nil,
	)
	// resolve namespace
	var namespace runtime.Object
	if ns := request.Request.Namespace; ns != "" {
		namespace = e.nsResolver(ns)
	}
	// evaluate policies
	for _, policy := range policies {
		if predicate != nil && !predicate(policy.Policy) {
			continue
		}
		response.Policies = append(response.Policies, e.handlePolicy(ctx, policy, nil, attr, &request.Request, namespace, request.Context))
	}
	return response, nil
}

func (e *engineImpl) handlePolicy(ctx context.Context, policy Policy, jsonPayload any, attr admission.Attributes, request *admissionv1.AdmissionRequest, namespace runtime.Object, context libs.Context) engine.ValidatingPolicyResponse {
	response := engine.ValidatingPolicyResponse{
		Actions: policy.Actions,
		Policy:  policy.Policy,
	}
	if e.matcher != nil {
		matches, err := e.matchPolicy(policy.Policy.Spec.MatchConstraints, attr, namespace)
		if err != nil {
			response.Rules = handlers.WithResponses(engineapi.RuleError("match", engineapi.Validation, "failed to execute matching", err, nil))
			return response
		} else if !matches {
			return response
		}
	}
	var result *compiler.EvaluationResult
	var err error
	if jsonPayload != nil {
		result, err = policy.CompiledPolicy.Evaluate(ctx, jsonPayload, nil, nil, nil, context)
	} else {
		result, err = policy.CompiledPolicy.Evaluate(ctx, nil, attr, request, namespace, context)
	}
	// TODO: error is about match conditions here ?
	if err != nil {
		response.Rules = handlers.WithResponses(engineapi.RuleError("evaluation", engineapi.Validation, "failed to load context", err, nil))
	} else if result == nil {
		response.Rules = append(response.Rules, *engineapi.RuleSkip("", engineapi.Validation, "skip", nil))
	} else if len(result.Exceptions) > 0 {
		exceptions := make([]engineapi.GenericException, 0, len(result.Exceptions))
		var keys []string
		for i := range result.Exceptions {
			key, err := cache.MetaNamespaceKeyFunc(result.Exceptions[i])
			if err != nil {
				response.Rules = handlers.WithResponses(engineapi.RuleError("exception", engineapi.Validation, "failed to compute exception key", err, nil))
				return response
			}
			keys = append(keys, key)
			exceptions = append(exceptions, engineapi.NewCELPolicyException(result.Exceptions[i]))
		}
		response.Rules = handlers.WithResponses(engineapi.RuleSkip("exception", engineapi.Validation, "rule is skipped due to policy exception: "+strings.Join(keys, ", "), nil).WithExceptions(exceptions))
	} else {
		// TODO: do we want to set a rule name?
		ruleName := ""
		if result.Error != nil {
			response.Rules = append(response.Rules, *engineapi.RuleError(ruleName, engineapi.Validation, "error", err, nil))
		} else if result.Result {
			response.Rules = append(response.Rules, *engineapi.RulePass(ruleName, engineapi.Validation, "success", nil))
		} else {
			response.Rules = append(response.Rules, *engineapi.RuleFail(ruleName, engineapi.Validation, result.Message, result.AuditAnnotations))
		}
	}
	return response
}

func (e *engineImpl) matchPolicy(constraints *admissionregistrationv1.MatchResources, attr admission.Attributes, namespace runtime.Object) (bool, error) {
	if constraints == nil {
		return false, nil
	}
	matches, err := e.matcher.Match(&matching.MatchCriteria{Constraints: constraints}, attr, namespace)
	if err != nil {
		return false, err
	}
	return matches, nil
}
