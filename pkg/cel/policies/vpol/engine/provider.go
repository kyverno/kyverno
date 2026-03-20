package engine

import (
	"context"
	"fmt"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/autogen"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Provider = engine.Provider[Policy]

type ProviderFunc func(context.Context) ([]Policy, error)

func (f ProviderFunc) Fetch(ctx context.Context) ([]Policy, error) {
	return f(ctx)
}

func NewProvider(
	compiler vpolcompiler.Compiler,
	policies []policiesv1beta1.ValidatingPolicyLike,
	exceptions []*policiesv1beta1.PolicyException,
) (ProviderFunc, error) {
	out := make([]Policy, 0, len(policies))
	for _, policy := range policies {
		spec := policy.GetValidatingPolicySpec()
		actions := sets.New(spec.ValidationActions()...)
		compiled, errs := compiler.Compile(policy)
		if len(errs) > 0 {
			return nil, fmt.Errorf("failed to compile policy %s (%w)", policy.GetName(), errs.ToAggregate())
		}
		out = append(out, Policy{
			Actions:        actions,
			Policy:         policy,
			CompiledPolicy: compiled,
		})
		generated, err := autogen.Autogen(policy)
		if err != nil {
			return nil, err
		}
		for _, autogen := range generated {
			autogenPolicy := policy.DeepCopyObject().(policiesv1beta1.ValidatingPolicyLike)
			*autogenPolicy.GetValidatingPolicySpec() = *autogen.Spec
			compiled, errs := compiler.Compile(autogenPolicy)
			if len(errs) > 0 {
				return nil, fmt.Errorf("failed to compile policy %s (%w)", autogenPolicy.GetName(), errs.ToAggregate())
			}
			out = append(out, Policy{
				Actions:        actions,
				Policy:         autogenPolicy,
				CompiledPolicy: compiled,
			})
		}
	}
	return func(context.Context) ([]Policy, error) {
		return out, nil
	}, nil
}

func NewKubeProvider(
	compiler vpolcompiler.Compiler,
	mgr ctrl.Manager,
) (Provider, error) {
	reconciler := newReconciler(compiler, mgr.GetClient())

	vpolBuilder := ctrl.NewControllerManagedBy(mgr).For(&policiesv1beta1.ValidatingPolicy{})
	nvpolBuilder := ctrl.NewControllerManagedBy(mgr).For(&policiesv1beta1.NamespacedValidatingPolicy{})

	if err := vpolBuilder.Complete(reconciler); err != nil {
		return nil, fmt.Errorf("failed to construct validatingpolicy controller: %w", err)
	}
	if err := nvpolBuilder.Complete(reconciler); err != nil {
		return nil, fmt.Errorf("failed to construct namespacedvalidatingpolicy controller: %w", err)
	}

	return reconciler, nil
}
