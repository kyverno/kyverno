package polex

import (
	"context"
	"fmt"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Provider = engine.Provider[Exception]

type ProviderFunc func(context.Context) ([]Exception, error)

func (f ProviderFunc) Fetch(ctx context.Context) ([]Exception, error) {
	return f(ctx)
}

func NewProvider(
	compiler Compiler,
	exceptions []policiesv1beta1.PolicyException,
) (ProviderFunc, error) {
	out := make([]Exception, 0, len(exceptions))
	for _, exception := range exceptions {
		compiled, errs := compiler.Compile(exception)
		if len(errs) > 0 {
			return nil, fmt.Errorf("failed to compile exception %s (%w)", exception.GetName(), errs.ToAggregate())
		}
		out = append(out, *compiled)
	}
	return func(context.Context) ([]Exception, error) {
		return out, nil
	}, nil
}

func NewKubeProvider(
	compiler Compiler,
	mgr ctrl.Manager,
) (Provider, error) {
	reconciler := newReconciler(compiler, mgr.GetClient())
	polexBuilder := ctrl.NewControllerManagedBy(mgr).For(&policiesv1beta1.PolicyException{})
	if err := polexBuilder.Complete(reconciler); err != nil {
		return nil, fmt.Errorf("failed to construct policyexception controller: %w", err)
	}
	return reconciler, nil
}
