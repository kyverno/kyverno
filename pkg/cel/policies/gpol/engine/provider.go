package engine

import (
	"context"
	"fmt"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
)

type Provider interface {
	Get(context.Context, string) (Policy, error)
}

type ProviderFunc func(context.Context) ([]Policy, error)

func (f ProviderFunc) Fetch(ctx context.Context) ([]Policy, error) {
	return f(ctx)
}

func NewProvider(
	compiler compiler.Compiler,
	policies []policiesv1alpha1.GeneratingPolicy,
) (ProviderFunc, error) {
	out := make([]Policy, 0, len(policies))
	for _, policy := range policies {
		compiled, errs := compiler.Compile(&policy)
		if len(errs) > 0 {
			return nil, fmt.Errorf("failed to compile policy %s (%w)", policy.GetName(), errs.ToAggregate())
		}
		out = append(out, Policy{
			Policy:         policy,
			CompiledPolicy: compiled,
		})
	}
	return func(context.Context) ([]Policy, error) {
		return out, nil
	}, nil
}
