package engine

import (
	"context"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
)

type Provider interface {
	Get(context.Context, string) (Policy, error)
}

type fetchProvider struct {
	compiler    compiler.Compiler
	gpolLister  policiesv1alpha1listers.GeneratingPolicyLister
	polexLister policiesv1alpha1listers.PolicyExceptionLister
}

func NewFetchProvider(
	compiler compiler.Compiler,
	gpolLister policiesv1alpha1listers.GeneratingPolicyLister,
	polexLister policiesv1alpha1listers.PolicyExceptionLister,
	polexEnabled bool,
) *fetchProvider {
	fp := &fetchProvider{
		compiler:   compiler,
		gpolLister: gpolLister,
	}

	if polexEnabled {
		fp.polexLister = polexLister
	}
	return fp
}

func (fp *fetchProvider) Get(ctx context.Context, name string) (Policy, error) {
	policy, err := fp.gpolLister.Get(name)
	if err != nil {
		return Policy{}, err
	}
	var exceptions, matchedExceptions []*policiesv1alpha1.PolicyException
	if fp.polexLister != nil {
		exceptions, err = fp.polexLister.List(labels.Everything())
		if err != nil {
			return Policy{}, err
		}
	}
	for _, polex := range exceptions {
		for _, ref := range polex.Spec.PolicyRefs {
			if ref.Name == policy.GetName() && ref.Kind == policy.GetKind() {
				matchedExceptions = append(matchedExceptions, polex)
			}
		}
	}
	compiled, errList := fp.compiler.Compile(policy, matchedExceptions)
	if errList != nil {
		return Policy{}, errList.ToAggregate()
	}

	return Policy{
		Policy:         *policy,
		Exceptions:     matchedExceptions,
		CompiledPolicy: compiled,
	}, nil
}
