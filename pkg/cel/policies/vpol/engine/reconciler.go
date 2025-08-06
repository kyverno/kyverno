package engine

import (
	"context"
	"fmt"
	"maps"
	"sync"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/autogen"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
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
	compiler compiler.Compiler,
	client client.Client,
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
	var exceptions []*policiesv1alpha1.PolicyException
	if r.polexEnabled {
		exceptions, err = engine.ListExceptions(r.polexLister, policy.GetKind(), policy.GetName())
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	compiled, errs := r.compiler.Compile(&policy, exceptions)
	if len(errs) > 0 {
		fmt.Println(errs)
		// No need to retry it
		return ctrl.Result{}, nil
	}
	actions := sets.New(policy.Spec.ValidationActions()...)
	policies := []Policy{{
		Actions:        actions,
		Policy:         policy,
		CompiledPolicy: compiled,
	}}
	generated, err := autogen.Autogen(&policy)
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, autogen := range generated {
		policy.Spec = *autogen.Spec
		compiled, errs := r.compiler.Compile(&policy, exceptions)
		if len(errs) > 0 {
			fmt.Println(errs)
			// No need to retry it
			return ctrl.Result{}, nil
		}
		policies = append(policies, Policy{
			Actions:        actions,
			Policy:         policy,
			CompiledPolicy: compiled,
		})
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	r.policies[req.NamespacedName.String()] = policies
	return ctrl.Result{}, nil
}

func (r *reconciler) Fetch(ctx context.Context) ([]Policy, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	policies := make([]Policy, 0, len(r.policies))
	for value := range maps.Values(r.policies) {
		policies = append(policies, value...)
	}
	return policies, nil
}
