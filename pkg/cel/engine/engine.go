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

type engine struct{}

func NewEngine() *engine {
	return &engine{}
}

func (e *engine) Handle(ctx context.Context, request EngineRequest, policies ...policy.CompiledPolicy) (EngineResponse, error) {
	var response EngineResponse
	for _, policy := range policies {
		// TODO
		_, err := policy.Evaluate(ctx, request.Resource, request.NamespaceLabels)
		if err != nil {
			return response, nil
		}
	}
	return response, nil
}
