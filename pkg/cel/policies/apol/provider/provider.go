package provider

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies/v1alpha1"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	"github.com/kyverno/kyverno/pkg/logging"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var logger = logging.WithName("apol-provider")

type Provider interface {
	Fetch(ctx context.Context) ([]*apolcompiler.Policy, error)
}

type reconciler struct {
	client   client.Client
	compiler apolcompiler.Compiler
	lock     sync.RWMutex
	policies map[string]*apolcompiler.Policy
}

func NewKubeProvider(compiler apolcompiler.Compiler, mgr ctrl.Manager) (Provider, error) {
	r := &reconciler{
		client:   mgr.GetClient(),
		compiler: compiler,
		policies: make(map[string]*apolcompiler.Policy),
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&policiesv1alpha1.AuthorizingPolicy{}).
		Complete(r); err != nil {
		return nil, fmt.Errorf("failed to construct authorizingpolicy controller: %w", err)
	}
	return r, nil
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cacheKey := req.NamespacedName.String()
	var policy policiesv1alpha1.AuthorizingPolicy
	if err := r.client.Get(ctx, req.NamespacedName, &policy); err != nil {
		if errors.IsNotFound(err) {
			r.lock.Lock()
			_, existed := r.policies[cacheKey]
			delete(r.policies, cacheKey)
			cacheSize := len(r.policies)
			r.lock.Unlock()
			logger.V(4).Info("removed AuthorizingPolicy from cache after delete", "policy", req.Name, "cacheKey", cacheKey, "existed", existed, "cacheSize", cacheSize)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	originalStatus := policy.Status
	r.lock.RLock()
	_, existed := r.policies[cacheKey]
	r.lock.RUnlock()
	logger.V(4).Info("reconciling AuthorizingPolicy", "policy", policy.Name, "cacheKey", cacheKey, "cached", existed, "generation", policy.Generation)

	compiled, errs := r.compiler.Compile(&policy)
	if len(errs) > 0 {
		logger.V(4).Info("failed to compile AuthorizingPolicy", "policy", policy.Name, "errors", errs)
		r.lock.Lock()
		_, existed := r.policies[cacheKey]
		delete(r.policies, cacheKey)
		cacheSize := len(r.policies)
		r.lock.Unlock()
		logger.V(4).Info("evicted AuthorizingPolicy from cache after compile failure", "policy", policy.Name, "cacheKey", cacheKey, "existed", existed, "cacheSize", cacheSize)
		reconcileConditionStatus(&policy, false, errs.ToAggregate().Error())
		if !reflect.DeepEqual(originalStatus, policy.Status) {
			if err := r.client.Status().Update(ctx, &policy); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	r.lock.Lock()
	r.policies[cacheKey] = compiled
	cacheSize := len(r.policies)
	r.lock.Unlock()
	if existed {
		logger.V(4).Info("updated AuthorizingPolicy in cache", "policy", policy.Name, "cacheSize", cacheSize, "ruleCount", len(compiled.Rules))
	} else {
		logger.V(4).Info("added AuthorizingPolicy to cache", "policy", policy.Name, "cacheSize", cacheSize, "ruleCount", len(compiled.Rules))
	}

	reconcileConditionStatus(&policy, true, "")
	if !reflect.DeepEqual(originalStatus, policy.Status) {
		if err := r.client.Status().Update(ctx, &policy); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *reconciler) Fetch(_ context.Context) ([]*apolcompiler.Policy, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	out := make([]*apolcompiler.Policy, 0, len(r.policies))
	for _, p := range r.policies {
		out = append(out, p)
	}
	return out, nil
}

func reconcileConditionStatus(policy *policiesv1alpha1.AuthorizingPolicy, compiled bool, message string) {
	status := &policy.Status.ConditionStatus
	if compiled {
		status.SetReadyByCondition(policiesv1alpha1.PolicyConditionTypePolicyCached, metav1.ConditionTrue, "Policy compiled and cached.")
		status.Message = ""
	} else {
		status.SetReadyByCondition(policiesv1alpha1.PolicyConditionTypePolicyCached, metav1.ConditionFalse, message)
		status.Message = message
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
