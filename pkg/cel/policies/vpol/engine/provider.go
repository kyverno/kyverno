package engine

import (
	"context"
	"fmt"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
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

type Provider = engine.PProvider[Policy]

type ProviderFunc func(context.Context) ([]Policy, error)

func (f ProviderFunc) Fetch(ctx context.Context) ([]Policy, error) {
	return f(ctx)
}

func NewProvider(
	compiler vpolcompiler.Compiler,
	policies []policiesv1alpha1.ValidatingPolicy,
	exceptions []*policiesv1alpha1.PolicyException,
) (ProviderFunc, error) {
	compiled := make([]Policy, 0, len(policies))
	for _, vp := range policies {
		var matchedExceptions []*policiesv1alpha1.PolicyException
		for _, polex := range exceptions {
			for _, ref := range polex.Spec.PolicyRefs {
				if ref.Name == vp.GetName() && ref.Kind == vp.GetKind() {
					matchedExceptions = append(matchedExceptions, polex)
				}
			}
		}
		policy, err := compiler.Compile(&vp, matchedExceptions)
		if err != nil {
			return nil, fmt.Errorf("failed to compile policy %s (%w)", vp.GetName(), err.ToAggregate())
		}
		compiled = append(compiled, Policy{
			Actions:        sets.New(vp.Spec.ValidationActions()...),
			Policy:         vp,
			CompiledPolicy: policy,
		})
	}
	return func(context.Context) ([]Policy, error) {
		return compiled, nil
	}, nil
}

func NewKubeProvider(
	compiler vpolcompiler.Compiler,
	mgr ctrl.Manager,
	polexLister policiesv1alpha1listers.PolicyExceptionLister,
) (Provider, error) {
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
	reconciler := newReconciler(compiler, mgr.GetClient(), polexLister)
	err := ctrl.NewControllerManagedBy(mgr).
		For(&policiesv1alpha1.ValidatingPolicy{}).
		Watches(&policiesv1alpha1.PolicyException{}, exceptionHandlerFuncs).
		Complete(reconciler)
	if err != nil {
		return nil, fmt.Errorf("failed to construct controller: %w", err)
	}
	return reconciler, nil
}
