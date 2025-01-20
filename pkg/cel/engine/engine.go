package engine

import (
	"context"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policy"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type EngineRequest struct {
	Resource *unstructured.Unstructured
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
	Handle(context.Context, EngineRequest, ...policy.CompiledPolicy) (EngineResponse, error)
}

type NamespaceResolver = func(string) *corev1.Namespace

type engine struct {
	nsResolver NamespaceResolver
	provider   Provider
}

func NewEngine(provider Provider, nsResolver NamespaceResolver) *engine {
	return &engine{
		nsResolver: nsResolver,
		provider:   provider,
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
	var namespace *unstructured.Unstructured
	if ns := request.Resource.GetNamespace(); ns != "" {
		coreNs := e.nsResolver(ns)
		if coreNs != nil {
			ns, err := kubeutils.ObjToUnstructured(coreNs)
			if err != nil {
				return response, err
			}
			namespace = ns
		}
	}
	for _, policy := range policies {
		response.Policies = append(response.Policies, e.handlePolicy(ctx, policy, request.Resource, namespace))
	}
	return response, nil
}

func (e *engine) handlePolicy(ctx context.Context, policy policy.CompiledPolicy, resource *unstructured.Unstructured, namespace *unstructured.Unstructured) PolicyResponse {
	var rules []engineapi.RuleResponse
	ok, err := policy.Evaluate(ctx, resource, namespace)
	// TODO: evaluation should be per rule
	if err != nil {
		rules = handlers.WithResponses(engineapi.RuleError("todo", engineapi.Validation, "failed to load context", err, nil))
	} else if ok {
		rules = handlers.WithResponses(engineapi.RulePass("todo", engineapi.Validation, "success", nil))
	} else {
		rules = handlers.WithResponses(engineapi.RuleFail("todo", engineapi.Validation, "failure", nil))
	}
	return PolicyResponse{
		// TODO
		Policy: kyvernov2alpha1.ValidatingPolicy{},
		Rules:  rules,
	}
}
