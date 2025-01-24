package engine

import (
	"context"
	"fmt"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	contextlib "github.com/kyverno/kyverno/pkg/cel/libs/context"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type EngineRequest struct {
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
	nsResolver NamespaceResolver
	provider   Provider
}

func NewEngine(provider Provider, nsResolver NamespaceResolver) Engine {
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
		response.Policies = append(response.Policies, e.handlePolicy(ctx, request, policy, namespace))
	}
	return response, nil
}

func (e *engine) handlePolicy(ctx context.Context, request EngineRequest, policy CompiledPolicy, namespace *unstructured.Unstructured) PolicyResponse {
	var rules []engineapi.RuleResponse
	results, err := policy.CompiledPolicy.Evaluate(ctx, request.Resource, namespace, request.Context)
	// TODO: error is about match conditions here ?
	if err != nil {
		rules = handlers.WithResponses(engineapi.RuleError("evaluation", engineapi.Validation, "failed to load context", err, nil))
	} else {
		for index, result := range results {
			ruleName := fmt.Sprintf("rule-%d", index)
			if result.Error != nil {
				rules = append(rules, *engineapi.RuleError(ruleName, engineapi.Validation, "error", err, nil))
			} else if result, err := utils.ConvertToNative[bool](result.Result); err != nil {
				rules = append(rules, *engineapi.RuleError(ruleName, engineapi.Validation, "conversion error", err, nil))
			} else if result {
				rules = append(rules, *engineapi.RulePass(ruleName, engineapi.Validation, "success", nil))
			} else {
				rules = append(rules, *engineapi.RuleFail(ruleName, engineapi.Validation, "failure", nil))
			}
		}
	}
	return PolicyResponse{
		Policy: policy.Policy,
		Rules:  rules,
	}
}
