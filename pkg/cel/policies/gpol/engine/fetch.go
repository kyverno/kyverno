package engine

import (
	"context"

	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
)

type Provider interface {
	Get(context.Context, string) (Policy, error)
}

type fetchProvider struct {
	compiler   compiler.Compiler
	gpolLister policiesv1alpha1listers.GeneratingPolicyLister
}

func NewFetchProvider(
	compiler compiler.Compiler,
	gpolLister policiesv1alpha1listers.GeneratingPolicyLister,
) *fetchProvider {
	return &fetchProvider{
		compiler:   compiler,
		gpolLister: gpolLister,
	}
}

func (r *fetchProvider) Get(ctx context.Context, name string) (Policy, error) {
	policy, err := r.gpolLister.Get(name)
	compiled, errList := r.compiler.Compile(policy)
	if err != nil {
		return Policy{}, errList.ToAggregate()
	}

	return Policy{
		Policy:         *policy,
		CompiledPolicy: compiled,
	}, nil
}
