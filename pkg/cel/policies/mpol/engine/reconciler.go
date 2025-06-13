package engine

import (
	"context"
	"sync"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type reconciler struct {
	client       client.Client
	compiler     compiler.Compiler
	lock         *sync.RWMutex
	policies     map[string][]Policy
	polexLister  policiesv1alpha1listers.PolicyExceptionLister
	polexEnabled bool
}

func newReconciler(
	client client.Client,
	compiler compiler.Compiler,
	polexLister policiesv1alpha1listers.PolicyExceptionLister,
	polexEnabled bool,
) *reconciler {
	return &reconciler{
		client:       client,
		compiler:     compiler,
		lock:         &sync.RWMutex{},
		policies:     map[string][]Policy{},
		polexLister:  polexLister,
		polexEnabled: polexEnabled,
	}
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var policy policiesv1alpha1.MutatingPolicy
	err := r.client.Get(ctx, req.NamespacedName, &policy)
	if errors.IsNotFound(err) {
		r.lock.Lock()
		delete(r.policies, req.NamespacedName.String())
		r.lock.Unlock()
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	if policy.GetStatus().Generated {
		r.lock.Lock()
		delete(r.policies, req.NamespacedName.String())
		r.lock.Unlock()
		return ctrl.Result{}, nil
	}

	var exceptions []*policiesv1alpha1.PolicyException
	if r.polexEnabled {
		exceptions, err = engine.ListExceptions(r.polexLister, policy.GetKind(), policy.GetName())
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	compiled, errs := r.compiler.Compile(&policy, exceptions)
	if len(errs) > 0 {
		return ctrl.Result{}, errs[0]
	}
	policies := []Policy{{
		Policy:         policy,
		CompiledPolicy: compiled,
	}}

	r.lock.Lock()
	r.policies[req.NamespacedName.String()] = policies
	r.lock.Unlock()
	return ctrl.Result{}, nil
}
