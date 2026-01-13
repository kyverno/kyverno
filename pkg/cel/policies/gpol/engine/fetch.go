package engine

import (
	"context"
	"fmt"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
)

type Provider interface {
	Get(context.Context, string) (Policy, error)
}

type fetchProvider struct {
	compiler    compiler.Compiler
	gpolLister  policiesv1beta1listers.GeneratingPolicyLister
	ngpolLister policiesv1beta1listers.NamespacedGeneratingPolicyLister
	polexLister policiesv1beta1listers.PolicyExceptionLister
}

func NewFetchProvider(
	compiler compiler.Compiler,
	gpolLister policiesv1beta1listers.GeneratingPolicyLister,
	ngpolLister policiesv1beta1listers.NamespacedGeneratingPolicyLister,
	polexLister policiesv1beta1listers.PolicyExceptionLister,
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

func (fp *fetchProvider) Get(ctx context.Context, name string) (Policy, error) {
	var policy policiesv1beta1.GeneratingPolicyLike
	var err error

	// Check if the name contains a namespace (format: "namespace/policy-name")
	parts := strings.Split(name, "/")
	if len(parts) == 2 {
		// Namespaced policy
		namespace, policyName := parts[0], parts[1]
		policy, err = fp.ngpolLister.NamespacedGeneratingPolicies(namespace).Get(policyName)
		if err != nil {
			return Policy{}, fmt.Errorf("namespaced generating policy %s/%s not found: %w", namespace, policyName, err)
		}
	} else {
		// Cluster-scoped policy
		policy, err = fp.gpolLister.Get(name)
		if err != nil {
			return Policy{}, fmt.Errorf("generating policy %s not found: %w", name, err)
		}
	}
	var exceptions, matchedExceptions []*policiesv1beta1.PolicyException
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
