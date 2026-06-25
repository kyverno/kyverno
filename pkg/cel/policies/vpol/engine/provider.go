package engine

import (
	"context"
	"fmt"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/autogen"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
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
	policies []policiesv1beta1.ValidatingPolicyLike,
	exceptions []*policiesv1beta1.PolicyException,
) (ProviderFunc, error) {
	out := make([]Policy, 0, len(policies))
	for _, policy := range policies {
		spec := policy.GetValidatingPolicySpec()
		actions := sets.New(spec.ValidationActions()...)
		var matchedExceptions []*policiesv1beta1.PolicyException
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
			autogenPolicy := policy.DeepCopyObject().(policiesv1beta1.ValidatingPolicyLike)
			*autogenPolicy.GetValidatingPolicySpec() = *autogen.Spec
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
	polexLister engine.PolicyExceptionLister,
	polexEnabled bool,
) (Provider, error) {
	reconciler := newReconciler(compiler, mgr.GetClient(), polexLister, polexEnabled)

	vpolBuilder := ctrl.NewControllerManagedBy(mgr).For(&policiesv1beta1.ValidatingPolicy{})
	nvpolBuilder := ctrl.NewControllerManagedBy(mgr).For(&policiesv1beta1.NamespacedValidatingPolicy{})

	type object = client.Object
	type eventCreate = event.TypedCreateEvent[object]
	type eventUpdate = event.TypedUpdateEvent[object]
	type eventDelete = event.TypedDeleteEvent[object]
	type queue = workqueue.TypedRateLimitingInterface[reconcile.Request]

	if polexEnabled {
		exceptionHandlerFuncs := &handler.Funcs{
			CreateFunc: func(ctx context.Context, tce eventCreate, trli queue) {
				polex := tce.Object.(*policiesv1beta1.PolicyException)
				for _, ref := range polex.Spec.PolicyRefs {
					applies := ref.Kind == policieskyvernoio.ValidatingPolicyKind || ref.Kind == policieskyvernoio.NamespacedValidatingPolicyKind
					if applies {
						trli.Add(reconcile.Request{
							NamespacedName: client.ObjectKey{
								Name: ref.Name,
							},
						})
					}
				}
			},
			UpdateFunc: func(ctx context.Context, tue eventUpdate, trli queue) {
				newPolex := tue.ObjectNew.(*policiesv1beta1.PolicyException)
				for _, ref := range newPolex.Spec.PolicyRefs {
					applies := ref.Kind == policieskyvernoio.ValidatingPolicyKind || ref.Kind == policieskyvernoio.NamespacedValidatingPolicyKind
					if applies {
						trli.Add(reconcile.Request{
							NamespacedName: client.ObjectKey{
								Name: ref.Name,
							},
						})
					}
				}
				oldPolex := tue.ObjectOld.(*policiesv1beta1.PolicyException)
				for _, ref := range oldPolex.Spec.PolicyRefs {
					applies := ref.Kind == policieskyvernoio.ValidatingPolicyKind || ref.Kind == policieskyvernoio.NamespacedValidatingPolicyKind
					if applies {
						trli.Add(reconcile.Request{
							NamespacedName: client.ObjectKey{
								Name: ref.Name,
							},
						})
					}
				}
			},
			DeleteFunc: func(ctx context.Context, tde eventDelete, trli queue) {
				polex := tde.Object.(*policiesv1beta1.PolicyException)
				for _, ref := range polex.Spec.PolicyRefs {
					applies := ref.Kind == policieskyvernoio.ValidatingPolicyKind || ref.Kind == policieskyvernoio.NamespacedValidatingPolicyKind
					if applies {
						trli.Add(reconcile.Request{
							NamespacedName: client.ObjectKey{
								Name: ref.Name,
							},
						})
					}
				}
			},
		}
		vpolBuilder = vpolBuilder.Watches(&policiesv1beta1.PolicyException{}, exceptionHandlerFuncs)
		nvpolBuilder = nvpolBuilder.Watches(&policiesv1beta1.PolicyException{}, exceptionHandlerFuncs)
	}

	if err := vpolBuilder.Complete(reconciler); err != nil {
		return nil, fmt.Errorf("failed to construct validatingpolicy controller: %w", err)
	}
	if err := nvpolBuilder.Complete(reconciler); err != nil {
		return nil, fmt.Errorf("failed to construct namespacedvalidatingpolicy controller: %w", err)
	}

	return reconciler, nil
}
