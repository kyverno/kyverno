package engine

import (
	"context"
	"fmt"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	contextlib "github.com/kyverno/kyverno/pkg/cel/libs/context"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

type EngineRequest struct {
	Request  *admissionv1.AdmissionRequest
	Resource *unstructured.Unstructured
	Context  contextlib.ContextInterface
}

type EngineResponse struct {
	Resource *unstructured.Unstructured
	Policies []PolicyResponse
}

type PolicyResponse struct {
	Policy kyvernov2alpha1.ValidatingPolicy
	Rules  []engineapi.RuleResponse
}

type Engine interface {
	Handle(context.Context, EngineRequest) (EngineResponse, error)
}

type NamespaceResolver = func(string) *corev1.Namespace

type engine struct {
	provider   Provider
	nsResolver NamespaceResolver
	matcher    matching.Matcher
}

func NewEngine(provider Provider, nsResolver NamespaceResolver, matcher matching.Matcher) Engine {
	return &engine{
		provider:   provider,
		nsResolver: nsResolver,
		matcher:    matcher,
	}
}

func (e *engine) Handle(ctx context.Context, request EngineRequest) (EngineResponse, error) {
	response := EngineResponse{
		Resource: request.Resource,
	}
	policies, err := e.provider.CompiledPolicies(ctx)
	if err != nil {
		return response, err
	}
	// resolve namespace
	var namespace runtime.Object
	var attr admission.Attributes
	if request.Request != nil {
		object, oldObject, err := admissionutils.ExtractResources(nil, *request.Request)
		if err != nil {
			return response, err
		}
		dryRun := false
		if request.Request.DryRun != nil {
			dryRun = *request.Request.DryRun
		}
		attr = admission.NewAttributesRecord(
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
		if ns := request.Request.Namespace; ns != "" {
			namespace = e.nsResolver(ns)
		}
	} else {
		attr = admission.NewAttributesRecord(
			request.Resource,
			nil,
			request.Resource.GroupVersionKind(),
			request.Resource.GetNamespace(),
			request.Resource.GetName(),
			schema.GroupVersionResource{},
			"",
			admission.Create,
			nil,
			false,
			nil,
		)
		if ns := request.Resource.GetNamespace(); ns != "" {
			namespace = e.nsResolver(ns)
		}
	}
	for _, policy := range policies {
		response.Policies = append(response.Policies, e.handlePolicy(ctx, policy, attr, request.Request, namespace, request.Context))
	}
	return response, nil
}

func (e *engine) handlePolicy(ctx context.Context, policy CompiledPolicy, attr admission.Attributes, request *admissionv1.AdmissionRequest, namespace runtime.Object, context contextlib.ContextInterface) PolicyResponse {
	response := PolicyResponse{
		Policy: policy.Policy,
	}
	if e.matcher != nil {
		criteria := matchCriteria{constraints: policy.Policy.Spec.MatchConstraints}
		if matches, err := e.matcher.Match(&criteria, attr, namespace); err != nil {
			response.Rules = handlers.WithResponses(engineapi.RuleError("match", engineapi.Validation, "failed to execute matching", err, nil))
			return response
		} else if !matches {
			return response
		}
	}
	results, err := policy.CompiledPolicy.Evaluate(ctx, attr, request, namespace, context)
	// TODO: error is about match conditions here ?
	if err != nil {
		response.Rules = handlers.WithResponses(engineapi.RuleError("evaluation", engineapi.Validation, "failed to load context", err, nil))
	} else {
		for index, result := range results {
			ruleName := fmt.Sprintf("rule-%d", index)
			if result.Error != nil {
				response.Rules = append(response.Rules, *engineapi.RuleError(ruleName, engineapi.Validation, "error", err, nil))
			} else if result, err := utils.ConvertToNative[bool](result.Result); err != nil {
				response.Rules = append(response.Rules, *engineapi.RuleError(ruleName, engineapi.Validation, "conversion error", err, nil))
			} else if result {
				response.Rules = append(response.Rules, *engineapi.RulePass(ruleName, engineapi.Validation, "success", nil))
			} else {
				response.Rules = append(response.Rules, *engineapi.RuleFail(ruleName, engineapi.Validation, "failure", nil))
			}
		}
	}
	return response
}
