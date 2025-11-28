package engine

import (
	"context"
	"fmt"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
)

type Provider interface {
	Get(context.Context, string, string) (Policy, error)
}

type fetchProvider struct {
	compiler    compiler.Compiler
	gpolLister  policiesv1beta1listers.GeneratingPolicyLister
	ngpolLister policiesv1beta1listers.NamespacedGeneratingPolicyLister
	polexLister policiesv1alpha1listers.PolicyExceptionLister
}

func NewFetchProvider(
	compiler compiler.Compiler,
	gpolLister policiesv1beta1listers.GeneratingPolicyLister,
	ngpolLister policiesv1beta1listers.NamespacedGeneratingPolicyLister,
	polexLister policiesv1alpha1listers.PolicyExceptionLister,
	polexEnabled bool,
) *fetchProvider {
	fp := &fetchProvider{
		compiler:    compiler,
		gpolLister:  gpolLister,
		ngpolLister: ngpolLister,
	}

	if polexEnabled {
		fp.polexLister = polexLister
	}
	return fp
}

func (fp *fetchProvider) Get(ctx context.Context, namespace, name string) (Policy, error) {
	var (
		policy policiesv1beta1.GeneratingPolicyLike
		err    error
	)
	if namespace == "" {
		policy, err = fp.gpolLister.Get(name)
	} else {
		policy, err = fp.ngpolLister.NamespacedGeneratingPolicies(namespace).Get(name)
	}
	if err != nil {
		return Policy{}, err
	}
	if policy == nil {
		if namespace == "" {
			return Policy{}, fmt.Errorf("generating policy %s not found", name)
		}
		return Policy{}, fmt.Errorf("generating policy %s/%s not found", namespace, name)
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
		Policy:         policy,
		Exceptions:     matchedExceptions,
		CompiledPolicy: compiled,
	}, nil
}
