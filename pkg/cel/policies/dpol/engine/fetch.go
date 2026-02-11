package engine

import (
	"context"
	"fmt"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
)

type fetchProvider struct {
	compiler     compiler.Compiler
	dpolLister   policiesv1beta1listers.DeletingPolicyLister
	ndpolLister  policiesv1beta1listers.NamespacedDeletingPolicyLister
	polexLister  policiesv1beta1listers.PolicyExceptionLister
	polexEnabled bool
}

func NewFetchProvider(
	compiler compiler.Compiler,
	dpolLister policiesv1beta1listers.DeletingPolicyLister,
	ndpolLister policiesv1beta1listers.NamespacedDeletingPolicyLister,
	polexLister policiesv1beta1listers.PolicyExceptionLister,
	polexEnabled bool,
) *fetchProvider {
	return &fetchProvider{
		compiler:     compiler,
		dpolLister:   dpolLister,
		ndpolLister:  ndpolLister,
		polexLister:  polexLister,
		polexEnabled: polexEnabled,
	}
}

func (r *fetchProvider) Get(ctx context.Context, namespace, name string) (Policy, error) {
	var (
		policy policiesv1beta1.DeletingPolicyLike
		err    error
	)
	if namespace == "" {
		policy, err = r.dpolLister.Get(name)
	} else {
		policy, err = r.ndpolLister.NamespacedDeletingPolicies(namespace).Get(name)
	}
	if err != nil {
		return Policy{}, err
	}
	if policy == nil {
		if namespace == "" {
			return Policy{}, fmt.Errorf("deleting policy %s not found", name)
		}
		return Policy{}, fmt.Errorf("deleting policy %s/%s not found", namespace, name)
	}
	// get exceptions that match the policy
	var exceptions []*policiesv1beta1.PolicyException
	if r.polexEnabled {
		exceptions, err = engine.ListExceptions(r.polexLister, policy.GetKind(), policy.GetName())
		if err != nil {
			return Policy{}, err
		}
	}
	compiled, errList := r.compiler.Compile(policy, exceptions)
	if errList != nil {
		return Policy{}, errList.ToAggregate()
	}
	return Policy{
		Policy:         policy,
		CompiledPolicy: compiled,
	}, nil
}
