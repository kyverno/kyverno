package engine

import (
	"context"
	"fmt"
	"sync"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	"github.com/kyverno/kyverno/pkg/logging"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	workqueue "k8s.io/client-go/util/workqueue"
)

// policyKey returns the cache key for a NamespacedName. For cluster-scoped
// resources (empty namespace) it is just the name; for namespaced resources it
// is "namespace/name". This matches the format used by the webhook handler when
// it sets ur.Spec.Policy (GetPolicyKey).
func policyKey(nn types.NamespacedName) string {
	if nn.Namespace == "" {
		return nn.Name
	}
	return nn.Namespace + "/" + nn.Name
}

type reconciler struct {
	client       client.Client
	compiler     compiler.Compiler
	lock         sync.RWMutex
	policies     map[string]Policy
	polexLister  celengine.PolicyExceptionLister
	polexEnabled bool
}

func newReconciler(
	comp compiler.Compiler,
	c client.Client,
	polexLister celengine.PolicyExceptionLister,
	polexEnabled bool,
) *reconciler {
	return &reconciler{
		client:       c,
		compiler:     comp,
		policies:     make(map[string]Policy),
		polexLister:  polexLister,
		polexEnabled: polexEnabled,
	}
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var policy policiesv1beta1.GeneratingPolicyLike

	if req.NamespacedName.Namespace == "" {
		var gp policiesv1beta1.GeneratingPolicy
		if err := r.client.Get(ctx, req.NamespacedName, &gp); err != nil {
			if errors.IsNotFound(err) {
				r.lock.Lock()
				delete(r.policies, policyKey(req.NamespacedName))
				r.lock.Unlock()
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}
		policy = &gp
	} else {
		var ngp policiesv1beta1.NamespacedGeneratingPolicy
		if err := r.client.Get(ctx, req.NamespacedName, &ngp); err != nil {
			if errors.IsNotFound(err) {
				r.lock.Lock()
				delete(r.policies, policyKey(req.NamespacedName))
				r.lock.Unlock()
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}
		policy = &ngp
	}

	var exceptions []*policiesv1beta1.PolicyException
	if r.polexEnabled {
		var err error
		exceptions, err = celengine.ListExceptions(r.polexLister, policy.GetKind(), policy.GetName())
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	compiled, errs := r.compiler.Compile(policy, exceptions)
	if errs != nil {
		logging.V(4).Info("failed to compile generating policy", "policy", policyKey(req.NamespacedName), "errors", errs)
		return ctrl.Result{}, nil
	}

	r.lock.Lock()
	r.policies[policyKey(req.NamespacedName)] = Policy{
		Policy:         policy,
		Exceptions:     exceptions,
		CompiledPolicy: compiled,
	}
	r.lock.Unlock()
	return ctrl.Result{}, nil
}

// Get serves a compiled Policy from the in-memory cache. Returns an error if
// the policy has not been reconciled yet.
func (r *reconciler) Get(_ context.Context, name string) (Policy, error) {
	r.lock.RLock()
	policy, ok := r.policies[name]
	r.lock.RUnlock()
	if !ok {
		return Policy{}, fmt.Errorf("generating policy %q not found in cache", name)
	}
	return policy, nil
}

// NewKubeProvider creates a reconciler-backed Provider that pre-compiles
// GeneratingPolicy and NamespacedGeneratingPolicy objects and serves Get()
// calls from an in-memory cache. Policies are recompiled only when the
// policy object or a referencing PolicyException changes.
func NewKubeProvider(
	comp compiler.Compiler,
	mgr ctrl.Manager,
	polexLister celengine.PolicyExceptionLister,
	polexEnabled bool,
) (Provider, error) {
	r := newReconciler(comp, mgr.GetClient(), polexLister, polexEnabled)

	gpolBuilder := ctrl.NewControllerManagedBy(mgr).For(&policiesv1beta1.GeneratingPolicy{})
	ngpolBuilder := ctrl.NewControllerManagedBy(mgr).For(&policiesv1beta1.NamespacedGeneratingPolicy{})

	if polexEnabled {
		type object = client.Object
		type createEvent = event.TypedCreateEvent[object]
		type updateEvent = event.TypedUpdateEvent[object]
		type deleteEvent = event.TypedDeleteEvent[object]
		type queue = workqueue.TypedRateLimitingInterface[reconcile.Request]

		exceptionHandler := &handler.Funcs{
			CreateFunc: func(ctx context.Context, tce createEvent, trli queue) {
				polex := tce.Object.(*policiesv1beta1.PolicyException)
				for _, ref := range polex.Spec.PolicyRefs {
					if ref.Kind == policieskyvernoio.GeneratingPolicyKind || ref.Kind == policieskyvernoio.NamespacedGeneratingPolicyKind {
						trli.Add(reconcile.Request{NamespacedName: client.ObjectKey{Name: ref.Name}})
					}
				}
			},
			UpdateFunc: func(ctx context.Context, tue updateEvent, trli queue) {
				newPolex := tue.ObjectNew.(*policiesv1beta1.PolicyException)
				for _, ref := range newPolex.Spec.PolicyRefs {
					if ref.Kind == policieskyvernoio.GeneratingPolicyKind || ref.Kind == policieskyvernoio.NamespacedGeneratingPolicyKind {
						trli.Add(reconcile.Request{NamespacedName: client.ObjectKey{Name: ref.Name}})
					}
				}
				oldPolex := tue.ObjectOld.(*policiesv1beta1.PolicyException)
				for _, ref := range oldPolex.Spec.PolicyRefs {
					if ref.Kind == policieskyvernoio.GeneratingPolicyKind || ref.Kind == policieskyvernoio.NamespacedGeneratingPolicyKind {
						trli.Add(reconcile.Request{NamespacedName: client.ObjectKey{Name: ref.Name}})
					}
				}
			},
			DeleteFunc: func(ctx context.Context, tde deleteEvent, trli queue) {
				polex := tde.Object.(*policiesv1beta1.PolicyException)
				for _, ref := range polex.Spec.PolicyRefs {
					if ref.Kind == policieskyvernoio.GeneratingPolicyKind || ref.Kind == policieskyvernoio.NamespacedGeneratingPolicyKind {
						trli.Add(reconcile.Request{NamespacedName: client.ObjectKey{Name: ref.Name}})
					}
				}
			},
		}
		gpolBuilder.Watches(&policiesv1beta1.PolicyException{}, exceptionHandler)
		ngpolBuilder.Watches(&policiesv1beta1.PolicyException{}, exceptionHandler)
	}

	if err := gpolBuilder.Complete(r); err != nil {
		return nil, fmt.Errorf("failed to construct generatingpolicy controller: %w", err)
	}
	if err := ngpolBuilder.Complete(r); err != nil {
		return nil, fmt.Errorf("failed to construct namespacedgeneratingpolicy controller: %w", err)
	}

	return r, nil
}
