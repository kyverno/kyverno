package engine

import (
	"context"
	"fmt"
	"sync"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type reconciler struct {
	client      client.Client
	compiler    compiler.Compiler
	lock        *sync.RWMutex
	policies    map[string]Policy
	polexLister policiesv1alpha1listers.PolicyExceptionLister
}

func newReconciler(
	compiler compiler.Compiler,
	client client.Client,
	polexLister policiesv1alpha1listers.PolicyExceptionLister,
) *reconciler {
	return &reconciler{
		client:      client,
		compiler:    compiler,
		lock:        &sync.RWMutex{},
		policies:    map[string]Policy{},
		polexLister: polexLister,
	}
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
	exceptions, err := listExceptions(r.polexLister, policy.GetName(), policy.GetKind())
	if err != nil {
		return ctrl.Result{}, err
	}
	compiled, errs := r.compiler.Compile(&policy, exceptions)
	if len(errs) > 0 {
		fmt.Println(errs)
		// No need to retry it
		return ctrl.Result{}, nil
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	actions := sets.New(policy.Spec.ValidationActions()...)
	r.policies[req.NamespacedName.String()] = Policy{
		Actions:        actions,
		Policy:         policy,
		CompiledPolicy: compiled,
	}
	return ctrl.Result{}, nil
}

func (r *reconciler) Fetch(ctx context.Context) ([]Policy, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return maps.Values(r.policies), nil
}
