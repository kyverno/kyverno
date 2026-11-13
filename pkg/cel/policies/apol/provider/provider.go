// Package provider keeps an in-memory set of compiled AuthorizingPolicy
// resources, reconciled from the cluster through controller-runtime.
package provider

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	apolengine "github.com/kyverno/kyverno/pkg/cel/policies/apol/engine"
	"github.com/kyverno/kyverno/pkg/logging"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var logger = logging.WithName("apol-provider")

type reconciler struct {
	client   client.Client
	compiler apolcompiler.Compiler
	lock     sync.RWMutex
	policies map[string]apolengine.CompiledPolicy
}

// Provider is an engine provider that can be warmed up before serving.
type Provider interface {
	apolengine.Provider
	// WarmUp synchronously compiles all policies already in the manager cache.
	// Call it after the cache has synced and before serving authorization
	// requests; otherwise Deny policies could be missing for a short window.
	WarmUp(ctx context.Context) error
}

// NewKubeProvider registers an AuthorizingPolicy controller with the manager
// and returns a provider backed by its cache.
func NewKubeProvider(compiler apolcompiler.Compiler, mgr ctrl.Manager) (Provider, error) {
	r := &reconciler{
		client:   mgr.GetClient(),
		compiler: compiler,
		policies: map[string]apolengine.CompiledPolicy{},
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&policiesv1alpha1.AuthorizingPolicy{}).
		Complete(r); err != nil {
		return nil, fmt.Errorf("failed to construct authorizingpolicy controller: %w", err)
	}
	return r, nil
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	key := req.NamespacedName.String()
	var policy policiesv1alpha1.AuthorizingPolicy
	if err := r.client.Get(ctx, req.NamespacedName, &policy); err != nil {
		if errors.IsNotFound(err) {
			r.lock.Lock()
			delete(r.policies, key)
			r.lock.Unlock()
			logger.V(4).Info("removed AuthorizingPolicy from cache", "policy", req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	compiled := apolengine.NewCompiledPolicy(r.compiler, &policy)
	// A policy that fails to compile stays in the cache: its Deny and NoOpinion
	// rules then apply as errors (fail closed), see the authorization contract.
	r.lock.Lock()
	r.policies[key] = compiled
	r.lock.Unlock()
	original := policy.Status.DeepCopy()
	SetStatus(&policy, compiled.CompileError())
	if !reflect.DeepEqual(*original, policy.Status) {
		if err := r.client.Status().Update(ctx, &policy); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *reconciler) WarmUp(ctx context.Context) error {
	var list policiesv1alpha1.AuthorizingPolicyList
	if err := r.client.List(ctx, &list); err != nil {
		return err
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	for i := range list.Items {
		policy := &list.Items[i]
		key := client.ObjectKeyFromObject(policy).String()
		if _, ok := r.policies[key]; !ok {
			r.policies[key] = apolengine.NewCompiledPolicy(r.compiler, policy)
		}
	}
	return nil
}

func (r *reconciler) Fetch(context.Context) ([]apolengine.CompiledPolicy, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	out := make([]apolengine.CompiledPolicy, 0, len(r.policies))
	for _, p := range r.policies {
		out = append(out, p)
	}
	return out, nil
}

// SetStatus records the compile result in the policy status.
func SetStatus(policy *policiesv1alpha1.AuthorizingPolicy, compileErr error) {
	status := &policy.Status.ConditionStatus
	if compileErr == nil {
		status.SetReadyByCondition(policiesv1alpha1.PolicyConditionTypePolicyCached, metav1.ConditionTrue, "Policy compiled and cached.")
		status.Message = ""
	} else {
		status.SetReadyByCondition(policiesv1alpha1.PolicyConditionTypePolicyCached, metav1.ConditionFalse, compileErr.Error())
		status.Message = compileErr.Error()
	}
	ready := true
	for _, condition := range status.Conditions {
		if condition.Status != metav1.ConditionTrue {
			ready = false
			break
		}
	}
	status.Ready = &ready
}
