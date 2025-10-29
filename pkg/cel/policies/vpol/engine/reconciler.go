package engine

import (
	"context"
	"fmt"
	"maps"
	"sync"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1beta1"
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
	var policy policiesv1beta1.ValidatingPolicyLike
	if req.NamespacedName.Namespace == "" {
		var vp policiesv1beta1.ValidatingPolicy
		err := r.client.Get(ctx, req.NamespacedName, &vp)
		if err != nil {
			if errors.IsNotFound(err) {
				r.lock.Lock()
				defer r.lock.Unlock()
				delete(r.policies, req.NamespacedName.String())
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}
		policy = &vp
	} else {
		var nvp policiesv1beta1.NamespacedValidatingPolicy
		err := r.client.Get(ctx, req.NamespacedName, &nvp)
		if err != nil {
			if errors.IsNotFound(err) {
				r.lock.Lock()
				defer r.lock.Unlock()
				delete(r.policies, req.NamespacedName.String())
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}
		policy = &nvp
	}
	if policy.GetStatus().Generated {
		r.lock.Lock()
		defer r.lock.Unlock()
		delete(r.policies, req.NamespacedName.String())
		return ctrl.Result{}, nil
	}
	// get exceptions that match the policy
	var exceptions []*policiesv1alpha1.PolicyException
	var err error
	if r.polexEnabled {
		exceptions, err = engine.ListExceptions(r.polexLister, policy.GetKind(), policy.GetName())
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	compiled, errs := r.compiler.Compile(policy, exceptions)
	if len(errs) > 0 {
		fmt.Println(errs)
		return ctrl.Result{}, nil
	}
	spec := policy.GetValidatingPolicySpec()
	actions := sets.New(spec.ValidationActions()...)
	policies := []Policy{{
		Actions:        actions,
		Policy:         policy,
		CompiledPolicy: compiled,
	}}
	generated, err := autogen.Autogen(policy)
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, autogen := range generated {
		tempPolicy := policy
		if vp, ok := policy.(*policiesv1beta1.ValidatingPolicy); ok {
			vpCopy := vp.DeepCopy()
			vpCopy.Spec = *autogen.Spec
			tempPolicy = vpCopy
		} else if nvp, ok := policy.(*policiesv1beta1.NamespacedValidatingPolicy); ok {
			nvpCopy := nvp.DeepCopy()
			nvpCopy.Spec = *autogen.Spec
			tempPolicy = nvpCopy
		}

		compiled, errs := r.compiler.Compile(tempPolicy, exceptions)
		if len(errs) > 0 {
			fmt.Println(errs)
			return ctrl.Result{}, nil
		}
		policies = append(policies, Policy{
			Actions:        actions,
			Policy:         tempPolicy,
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
