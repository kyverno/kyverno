// Package provider keeps an in-memory set of compiled AuthorizingPolicy
// resources, reconciled from the cluster through controller-runtime.
package provider

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

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
	lock     sync.Mutex
	policies map[string]apolengine.CompiledPolicy
	// snapshot is rebuilt under lock after every change and read lock-free.
	// It is nil until WarmUp completes, so the engine fails closed before the
	// cache is warm instead of missing Deny policies.
	snapshot   atomic.Pointer[apolengine.Snapshot]
	generation uint64
	warm       bool
}

// publish rebuilds the immutable snapshot. Callers hold r.lock.
func (r *reconciler) publish() {
	if !r.warm {
		return
	}
	r.generation++
	policies := make([]apolengine.CompiledPolicy, 0, len(r.policies))
	for _, p := range r.policies {
		policies = append(policies, p)
	}
	r.snapshot.Store(apolengine.NewSnapshot(r.generation, policies...))
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
	// The cache read and the map update happen under one lock, so they are
	// serialized with WarmUp: whichever runs second sees the newer cache state.
	r.lock.Lock()
	if err := r.client.Get(ctx, req.NamespacedName, &policy); err != nil {
		if errors.IsNotFound(err) {
			delete(r.policies, key)
			r.publish()
			r.lock.Unlock()
			logger.V(4).Info("removed AuthorizingPolicy from cache", "policy", req.Name)
			return ctrl.Result{}, nil
		}
		r.lock.Unlock()
		return ctrl.Result{}, err
	}
	// Compile and validate at reconcile time, never on the request path.
	compiled := apolengine.NewCompiledPolicy(r.compiler, policy.DeepCopy())
	// A policy that fails to compile stays in the cache: its Deny and NoOpinion
	// rules then apply as errors (fail closed), see the authorization contract.
	r.policies[key] = compiled
	r.publish()
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

// WarmUp lists the cache under r.lock, the same lock Reconcile holds across its
// cache read, and replaces the policy set with the listed policies. A policy
// deleted before the list is therefore never re-added, and any change after
// the list is applied by the reconcile its cache event triggers.
func (r *reconciler) WarmUp(ctx context.Context) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	var list policiesv1alpha1.AuthorizingPolicyList
	if err := r.client.List(ctx, &list); err != nil {
		return err
	}
	policies := make(map[string]apolengine.CompiledPolicy, len(list.Items))
	for i := range list.Items {
		policy := &list.Items[i]
		policies[client.ObjectKeyFromObject(policy).String()] = apolengine.NewCompiledPolicy(r.compiler, policy)
	}
	r.policies = policies
	r.warm = true
	r.publish()
	return nil
}

func (r *reconciler) Snapshot(context.Context) (*apolengine.Snapshot, error) {
	if s := r.snapshot.Load(); s != nil {
		return s, nil
	}
	return nil, apolengine.ErrNoSnapshot
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
