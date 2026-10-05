package engine

import (
	"context"
	"fmt"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celautogen "github.com/kyverno/kyverno/pkg/cel/autogen"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/autogen"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	"github.com/kyverno/kyverno/pkg/logging"
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
		for config, generatedPolicy := range generated {
			autogenPolicy := policy.DeepCopyObject().(policiesv1beta1.ValidatingPolicyLike)
			*autogenPolicy.GetValidatingPolicySpec() = *generatedPolicy.Spec
			compiled, errs := compiler.Compile(autogenPolicy, matchedExceptions)
			if len(errs) > 0 {
				return nil, fmt.Errorf("failed to compile policy %s (%w)", autogenPolicy.GetName(), errs.ToAggregate())
			}
			out = append(out, Policy{
				Actions:        actions,
				Policy:         autogenPolicy,
				CompiledPolicy: compiled,
				ExtractionMode: config == autogen.ExtractionReplacementsRef,
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
	// Lets an unrecognized bare controller name (e.g. "jobsets") resolve via
	// live discovery instead of requiring the explicit
	// "<resource>.<version>.<group>" format - see autogen.BareNameResolver.
	// Also corrects the explicit format's best-effort Kind guess (see
	// autogen.KindResolver) for irregular plurals like "jobsets" -> "JobSet".
	celautogen.BareNameResolver = celautogen.RESTMapperBareNameResolver(mgr.GetRESTMapper())
	celautogen.KindResolver = celautogen.RESTMapperKindResolver(mgr.GetRESTMapper())
	reconciler := newReconciler(compiler, mgr.GetClient(), polexLister, polexEnabled)

	vpolBuilder := ctrl.NewControllerManagedBy(mgr).For(&policiesv1beta1.ValidatingPolicy{})
	nvpolBuilder := ctrl.NewControllerManagedBy(mgr).For(&policiesv1beta1.NamespacedValidatingPolicy{})

	if polexEnabled {
		exceptionHandlerFuncs := newPolicyExceptionHandler(mgr.GetClient())
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

type policyExceptionQueue = workqueue.TypedRateLimitingInterface[reconcile.Request]

// newPolicyExceptionHandler requeues the policies a PolicyException refers to.
// Policy refs carry no namespace, so a ref to a namespaced policy requeues every
// NamespacedValidatingPolicy with that name.
func newPolicyExceptionHandler(c client.Client) *handler.Funcs {
	enqueue := func(ctx context.Context, obj client.Object, q policyExceptionQueue) {
		polex, ok := obj.(*policiesv1beta1.PolicyException)
		if !ok {
			return
		}
		for _, ref := range polex.Spec.PolicyRefs {
			switch ref.Kind {
			case policieskyvernoio.ValidatingPolicyKind:
				q.Add(reconcile.Request{NamespacedName: client.ObjectKey{Name: ref.Name}})
			case policieskyvernoio.NamespacedValidatingPolicyKind:
				var policies policiesv1beta1.NamespacedValidatingPolicyList
				if err := c.List(ctx, &policies); err != nil {
					logging.Error(err, "failed to list namespaced validating policies", "policy", ref.Name)
					continue
				}
				for _, policy := range policies.Items {
					if policy.Name == ref.Name {
						q.Add(reconcile.Request{NamespacedName: client.ObjectKey{Namespace: policy.Namespace, Name: policy.Name}})
					}
				}
			}
		}
	}
	return &handler.Funcs{
		CreateFunc: func(ctx context.Context, e event.TypedCreateEvent[client.Object], q policyExceptionQueue) {
			enqueue(ctx, e.Object, q)
		},
		UpdateFunc: func(ctx context.Context, e event.TypedUpdateEvent[client.Object], q policyExceptionQueue) {
			enqueue(ctx, e.ObjectNew, q)
			enqueue(ctx, e.ObjectOld, q)
		},
		DeleteFunc: func(ctx context.Context, e event.TypedDeleteEvent[client.Object], q policyExceptionQueue) {
			enqueue(ctx, e.Object, q)
		},
	}
}
