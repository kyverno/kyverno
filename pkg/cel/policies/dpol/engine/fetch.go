package engine

import (
	"context"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
)

type fetchProvider struct {
	compiler     compiler.Compiler
	dpolLister   policiesv1alpha1listers.DeletingPolicyLister
	polexLister  policiesv1alpha1listers.PolicyExceptionLister
	polexEnabled bool
}

func NewFetchProvider(
	compiler compiler.Compiler,
	dpolLister policiesv1alpha1listers.DeletingPolicyLister,
	polexLister policiesv1alpha1listers.PolicyExceptionLister,
	polexEnabled bool,
) *fetchProvider {
	return &fetchProvider{
		compiler:     compiler,
		dpolLister:   dpolLister,
		polexLister:  polexLister,
		polexEnabled: polexEnabled,
	}
}

func (r *fetchProvider) Get(ctx context.Context, name string) (Policy, error) {
	policy, err := r.dpolLister.Get(name)
	if err != nil {
		return Policy{}, err
	}
	// get exceptions that match the policy
	var exceptions []*policiesv1alpha1.PolicyException
	if r.polexEnabled {
		exceptions, err = engine.ListExceptions(r.polexLister, policy.Kind, policy.GetName())
		if err != nil {
			return Policy{}, err
		}
	}
	compiled, errList := r.compiler.Compile(policy, exceptions)
	if err != nil {
		return Policy{}, errList.ToAggregate()
	}

	return Policy{
		Policy:         *policy,
		CompiledPolicy: compiled,
	}, nil
}
