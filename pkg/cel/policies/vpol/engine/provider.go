package engine

import (
	"context"
	"fmt"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/autogen"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Provider = engine.Provider[Policy]

type ProviderFunc func(context.Context) ([]Policy, error)

func (f ProviderFunc) Fetch(ctx context.Context) ([]Policy, error) {
	return f(ctx)
}

func NewProvider(
	compiler vpolcompiler.Compiler,
	policies []policiesv1alpha1.ValidatingPolicyLike,
	exceptions []*policiesv1alpha1.PolicyException,
) (ProviderFunc, error) {
	out := make([]Policy, 0, len(policies))
	for _, policy := range policies {
		spec := policy.GetValidatingPolicySpec()
		actions := sets.New(spec.ValidationActions()...)
		var matchedExceptions []*policiesv1alpha1.PolicyException
		for _, polex := range exceptions {
			for _, ref := range polex.Spec.PolicyRefs {
				if ref.Name == policy.GetName() && ref.Kind == policy.GetKind() {
					matchedExceptions = append(matchedExceptions, polex)
				}
			}
		}
		compiled, errs := compiler.Compile(policy, matchedExceptions)
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
			var autogenPolicy policiesv1alpha1.ValidatingPolicyLike
			if vp, ok := policy.(*policiesv1alpha1.ValidatingPolicy); ok {
				vpCopy := vp.DeepCopy()
				vpCopy.Spec = *autogen.Spec
				autogenPolicy = vpCopy
			} else if nvp, ok := policy.(*policiesv1alpha1.NamespacedValidatingPolicy); ok {
				nvpCopy := nvp.DeepCopy()
				nvpCopy.Spec = *autogen.Spec
				autogenPolicy = nvpCopy
			}
			compiled, errs := compiler.Compile(autogenPolicy, matchedExceptions)
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
	polexLister policiesv1alpha1listers.PolicyExceptionLister,
	polexEnabled bool,
) (Provider, error) {
	reconciler := newReconciler(compiler, mgr.GetClient(), polexLister, polexEnabled)
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&policiesv1alpha1.ValidatingPolicy{}).
		Watches(&policiesv1alpha1.NamespacedValidatingPolicy{}, &handler.EnqueueRequestForObject{})
	if polexEnabled {
		exceptionHandlerFuncs := &handler.Funcs{
			CreateFunc: func(
				ctx context.Context,
				tce event.TypedCreateEvent[client.Object],
				trli workqueue.TypedRateLimitingInterface[reconcile.Request],
			) {
				polex := tce.Object.(*policiesv1alpha1.PolicyException)
				for _, ref := range polex.Spec.PolicyRefs {
					trli.Add(reconcile.Request{
						NamespacedName: client.ObjectKey{
							Name: ref.Name,
						},
					})
				}
			},
			UpdateFunc: func(
				ctx context.Context,
				tue event.TypedUpdateEvent[client.Object],
				trli workqueue.TypedRateLimitingInterface[reconcile.Request],
			) {
				polex := tue.ObjectNew.(*policiesv1alpha1.PolicyException)
				for _, ref := range polex.Spec.PolicyRefs {
					trli.Add(reconcile.Request{
						NamespacedName: client.ObjectKey{
							Name: ref.Name,
						},
					})
				}
			},
			DeleteFunc: func(
				ctx context.Context,
				tde event.TypedDeleteEvent[client.Object],
				trli workqueue.TypedRateLimitingInterface[reconcile.Request],
			) {
				polex := tde.Object.(*policiesv1alpha1.PolicyException)
				for _, ref := range polex.Spec.PolicyRefs {
					trli.Add(reconcile.Request{
						NamespacedName: client.ObjectKey{
							Name: ref.Name,
						},
					})
				}
			},
		}
		builder = builder.Watches(&policiesv1alpha1.PolicyException{}, exceptionHandlerFuncs)
	}
	if err := builder.Complete(reconciler); err != nil {
		return nil, fmt.Errorf("failed to construct validatingpolicies manager: %w", err)
	}
	return reconciler, nil
}
