package engine

import (
	"context"
	"fmt"
	"sync"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policy"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"golang.org/x/exp/maps"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type CompiledPolicy struct {
	Actions        sets.Set[admissionregistrationv1.ValidationAction]
	Policy         policiesv1alpha1.ValidatingPolicy
	CompiledPolicy policy.CompiledPolicy
}

type Provider interface {
	CompiledPolicies(context.Context) ([]CompiledPolicy, error)
}

type ProviderFunc func(context.Context) ([]CompiledPolicy, error)

func (f ProviderFunc) CompiledPolicies(ctx context.Context) ([]CompiledPolicy, error) {
	return f(ctx)
}

func NewProvider(compiler policy.Compiler, policies []policiesv1alpha1.ValidatingPolicy, exceptions []*policiesv1alpha1.CELPolicyException) (ProviderFunc, error) {
	compiled := make([]CompiledPolicy, 0, len(policies))
	for _, vp := range policies {
		var matchedExceptions []policiesv1alpha1.CELPolicyException
		for _, polex := range exceptions {
			for _, ref := range polex.Spec.PolicyRefs {
				if ref.Name == vp.GetName() {
					matchedExceptions = append(matchedExceptions, *polex)
				}
			}
		}
		policy, err := compiler.CompileValidating(&vp, matchedExceptions)
		if err != nil {
			return nil, fmt.Errorf("failed to compile policy %s (%w)", vp.GetName(), err.ToAggregate())
		}
		actions := sets.New(vp.Spec.ValidationAction...)
		if len(actions) == 0 {
			actions.Insert(admissionregistrationv1.Deny)
		}
		compiled = append(compiled, CompiledPolicy{
			Actions:        actions,
			Policy:         vp,
			CompiledPolicy: policy,
		})
	}
	provider := func(context.Context) ([]CompiledPolicy, error) {
		return compiled, nil
	}
	return provider, nil
}

func NewKubeProvider(
	compiler policy.Compiler,
	mgr ctrl.Manager,
	polexLister policiesv1alpha1listers.CELPolicyExceptionLister,
) (Provider, error) {
	r := newPolicyReconciler(compiler, mgr.GetClient(), polexLister)
	err := ctrl.NewControllerManagedBy(mgr).
		For(&policiesv1alpha1.ValidatingPolicy{}).
		Watches(&policiesv1alpha1.CELPolicyException{}, &handler.Funcs{
			CreateFunc: func(
				ctx context.Context,
				tce event.TypedCreateEvent[client.Object],
				trli workqueue.TypedRateLimitingInterface[reconcile.Request],
			) {
				polex := tce.Object.(*policiesv1alpha1.CELPolicyException)
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
				polex := tue.ObjectNew.(*policiesv1alpha1.CELPolicyException)
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
				polex := tde.Object.(*policiesv1alpha1.CELPolicyException)
				for _, ref := range polex.Spec.PolicyRefs {
					trli.Add(reconcile.Request{
						NamespacedName: client.ObjectKey{
							Name: ref.Name,
						},
					})
				}
			},
		}).
		Complete(r)
	if err != nil {
		return nil, fmt.Errorf("failed to construct manager: %w", err)
	}
	return r, nil
}

type policyReconciler struct {
	client      client.Client
	compiler    policy.Compiler
	lock        *sync.RWMutex
	policies    map[string]CompiledPolicy
	polexLister policiesv1alpha1listers.CELPolicyExceptionLister
}

func newPolicyReconciler(
	compiler policy.Compiler,
	client client.Client,
	polexLister policiesv1alpha1listers.CELPolicyExceptionLister,
) *policyReconciler {
	return &policyReconciler{
		client:      client,
		compiler:    compiler,
		lock:        &sync.RWMutex{},
		policies:    map[string]CompiledPolicy{},
		polexLister: polexLister,
	}
}

func (r *policyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var policy policiesv1alpha1.ValidatingPolicy
	err := r.client.Get(ctx, req.NamespacedName, &policy)
	if errors.IsNotFound(err) {
		r.lock.Lock()
		defer r.lock.Unlock()
		delete(r.policies, req.NamespacedName.String())
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	if policy.GetStatus().Generated {
		r.lock.Lock()
		defer r.lock.Unlock()
		delete(r.policies, req.NamespacedName.String())
		return ctrl.Result{}, nil
	}
	// get exceptions that match the policy
	exceptions, err := r.ListExceptions(policy.GetName())
	if err != nil {
		return ctrl.Result{}, err
	}
	compiled, errs := r.compiler.CompileValidating(&policy, exceptions)
	if len(errs) > 0 {
		fmt.Println(errs)
		// No need to retry it
		return ctrl.Result{}, nil
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	actions := sets.New(policy.Spec.ValidationAction...)
	if len(actions) == 0 {
		actions.Insert(admissionregistrationv1.Deny)
	}
	r.policies[req.NamespacedName.String()] = CompiledPolicy{
		Actions:        actions,
		Policy:         policy,
		CompiledPolicy: compiled,
	}
	return ctrl.Result{}, nil
}

func (r *policyReconciler) CompiledPolicies(ctx context.Context) ([]CompiledPolicy, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return maps.Values(r.policies), nil
}

func (r *policyReconciler) ListExceptions(policyName string) ([]policiesv1alpha1.CELPolicyException, error) {
	polexList, err := r.polexLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var exceptions []policiesv1alpha1.CELPolicyException
	for _, polex := range polexList {
		for _, ref := range polex.Spec.PolicyRefs {
			if ref.Name == policyName {
				exceptions = append(exceptions, *polex)
			}
		}
	}
	return exceptions, nil
}
