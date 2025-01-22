package engine

import (
	"context"
	"fmt"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policy"
)

type Provider interface {
	CompiledPolicies(context.Context) ([]policy.CompiledPolicy, error)
}

type ProviderFunc func(context.Context) ([]policy.CompiledPolicy, error)

func (f ProviderFunc) CompiledPolicies(ctx context.Context) ([]policy.CompiledPolicy, error) {
	return f(ctx)
}

func NewProvider(compiler policy.Compiler, policies ...kyvernov2alpha1.ValidatingPolicy) (ProviderFunc, error) {
	compiled := make([]policy.CompiledPolicy, 0, len(policies))
	for _, vp := range policies {
		policy, err := compiler.Compile(&vp)
		if err != nil {
			return nil, fmt.Errorf("failed to compile policy %s (%w)", vp.GetName(), err.ToAggregate())
		}
		compiled = append(compiled, policy)
	}
	provider := func(context.Context) ([]policy.CompiledPolicy, error) {
		return compiled, nil
	}
	return provider, nil
}
