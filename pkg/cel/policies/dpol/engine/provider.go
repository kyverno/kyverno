package engine

import (
	"context"
	"fmt"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	dpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
)

type Provider interface {
	Get(context.Context, string) (Policy, error)
}

type ProviderFunc func(context.Context) ([]Policy, error)

func (f ProviderFunc) Fetch(ctx context.Context) ([]Policy, error) {
	return f(ctx)
}

func NewProvider(
	compiler dpolcompiler.Compiler,
	policies []policiesv1alpha1.DeletingPolicy,
	exceptions []*policiesv1alpha1.PolicyException,
) (ProviderFunc, error) {
	out := make([]Policy, 0, len(policies))
	for _, policy := range policies {
		var matchedExceptions []*policiesv1alpha1.PolicyException
		for _, polex := range exceptions {
			for _, ref := range polex.Spec.PolicyRefs {
				if ref.Name == policy.GetName() && ref.Kind == policy.Kind {
					matchedExceptions = append(matchedExceptions, polex)
				}
			}
		}
		compiled, errs := compiler.Compile(&policy, matchedExceptions)
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
