package engine

import (
	"context"
	"fmt"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

type Provider interface {
	Get(context.Context, string) (Policy, error)
}

type fetchProvider struct {
	compiler    compiler.Compiler
	gpolLister  policiesv1beta1listers.GeneratingPolicyLister
	ngpolLister policiesv1beta1listers.NamespacedGeneratingPolicyLister
	polexLister engine.PolicyExceptionLister
}

func NewFetchProvider(
	compiler compiler.Compiler,
	gpolLister policiesv1beta1listers.GeneratingPolicyLister,
	ngpolLister policiesv1beta1listers.NamespacedGeneratingPolicyLister,
	polexLister engine.PolicyExceptionLister,
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
		Policy:               policy,
		Exceptions:           matchedExceptions,
		CompiledPolicy:       compiled,
		LegacyOwnerConflicts: fp.legacyOwnerConflicts(policy),
	}, nil
}

// legacyOwnerConflicts returns the namespaces in which a downstream resource
// generated before the policy namespace label existed has an unknown owner,
// because a same-named policy of the other scope could have generated it. A
// namespaced policy only generates into its own namespace, so it shadows a
// cluster-scoped policy of the same name in exactly that namespace, and a
// cluster-scoped policy shadows a namespaced one in the namespaced policy's
// namespace. Cluster-scoped downstreams are never contested, because a
// namespaced policy cannot generate them. Lookup failures contest every
// namespace so ownership is never established from incomplete information.
func (fp *fetchProvider) legacyOwnerConflicts(policy policiesv1beta1.GeneratingPolicyLike) map[string]bool {
	if policy.GetNamespace() != "" {
		if _, err := fp.gpolLister.Get(policy.GetName()); err == nil || !apierrors.IsNotFound(err) {
			return map[string]bool{policy.GetNamespace(): true}
		}
		return nil
	}
	namespacedPolicies, err := fp.ngpolLister.List(labels.Everything())
	if err != nil {
		return map[string]bool{libs.LegacyOwnerAnyNamespace: true}
	}
	conflicts := map[string]bool{}
	for _, namespacedPolicy := range namespacedPolicies {
		if namespacedPolicy.GetName() == policy.GetName() {
			conflicts[namespacedPolicy.GetNamespace()] = true
		}
	}
	if len(conflicts) == 0 {
		return nil
	}
	return conflicts
}
