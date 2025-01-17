package engine

import (
	"context"

	"github.com/kyverno/kyverno/pkg/cel/policy"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type EngineRequest struct {
	Resource        *unstructured.Unstructured
	NamespaceLabels map[string]map[string]string
}

type EngineResponse struct{}

type Engine interface {
	Handle(context.Context, EngineRequest, ...policy.CompiledPolicy) (EngineResponse, error)
}

type engine struct {
	provider Provider
}

func NewEngine(provider Provider) *engine {
	return &engine{
		provider: provider,
	}
}

func (e *engine) Handle(ctx context.Context, request EngineRequest) (EngineResponse, error) {
	var response EngineResponse
	policies, err := e.provider.CompiledPolicies(ctx)
	if err != nil {
		return response, err
	}
	for _, policy := range policies {
		// TODO
		_, err := policy.Evaluate(ctx, request.Resource, request.NamespaceLabels)
		if err != nil {
			return response, err
		}
	}
	return response, nil
}
