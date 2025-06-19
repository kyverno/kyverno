package engine

import (
	"context"
	"fmt"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/client-go/openapi"
	workqueue "k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	event "sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Provider = engine.Provider[Policy]

type ProviderFunc func(context.Context) ([]Policy, error)

func (f ProviderFunc) Fetch(ctx context.Context) ([]Policy, error) {
	return f(ctx)
}

func NewKubeProvider(
	ctx context.Context,
	compiler mpolcompiler.Compiler,
	mgr ctrl.Manager,
	c openapi.Client,
	polexLister policiesv1alpha1listers.PolicyExceptionLister,
	polexEnabled bool,
) (Provider, patch.TypeConverterManager, error) {
	typeConverter := patch.NewTypeConverterManager(nil, c)
	go typeConverter.Run(ctx)

	reconciler := newReconciler(mgr.GetClient(), compiler, polexLister, polexEnabled)
	builder := ctrl.NewControllerManagedBy(mgr).For(&policiesv1alpha1.MutatingPolicy{})
	if polexEnabled {
		polexHandler := &handler.Funcs{
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
				tce event.TypedUpdateEvent[client.Object],
				trli workqueue.TypedRateLimitingInterface[reconcile.Request],
			) {
				polex := tce.ObjectNew.(*policiesv1alpha1.PolicyException)
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
		builder.Watches(&policiesv1alpha1.PolicyException{}, polexHandler)
	}
	if err := builder.Complete(reconciler); err != nil {
		return nil, typeConverter, fmt.Errorf("failed to construct mutatingpolicies manager: %w", err)
	}

	return reconciler, typeConverter, nil
}
