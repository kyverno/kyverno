package engine

import (
	"context"
	"fmt"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policy"
)

type CompiledPolicy struct {
	Policy         kyvernov2alpha1.ValidatingPolicy
	CompiledPolicy policy.CompiledPolicy
}

type Provider interface {
	CompiledPolicies(context.Context) ([]CompiledPolicy, error)
}

type ProviderFunc func(context.Context) ([]CompiledPolicy, error)

func (f ProviderFunc) CompiledPolicies(ctx context.Context) ([]CompiledPolicy, error) {
	return f(ctx)
}

func NewProvider(compiler policy.Compiler, policies ...kyvernov2alpha1.ValidatingPolicy) (ProviderFunc, error) {
	compiled := make([]CompiledPolicy, 0, len(policies))
	for _, vp := range policies {
		policy, err := compiler.Compile(&vp)
		if err != nil {
			return nil, fmt.Errorf("failed to compile policy %s (%w)", vp.GetName(), err.ToAggregate())
		}
		compiled = append(compiled, CompiledPolicy{
			Policy:         vp,
			CompiledPolicy: policy,
		})
	}
	provider := func(context.Context) ([]CompiledPolicy, error) {
		return compiled, nil
	}
	return provider, nil
}
