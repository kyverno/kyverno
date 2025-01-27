package engine

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policy"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CompiledPolicy struct {
	Policy         kyvernov2alpha1.ValidatingPolicy
	CompiledPolicy policy.CompiledPolicy
}

type Provider interface {
	CompiledPolicies(context.Context) ([]CompiledPolicy, error)
}

type ProviderFunc func(context.Context) ([]CompiledPolicy, error)

func (f ProviderFunc) CompiledPolicies(ctx context.Context) ([]CompiledPolicy, error) {
	return f(ctx)
}

func NewProvider(compiler policy.Compiler, policies ...kyvernov2alpha1.ValidatingPolicy) (ProviderFunc, error) {
	compiled := make([]CompiledPolicy, 0, len(policies))
	for _, vp := range policies {
		policy, err := compiler.Compile(&vp)
		if err != nil {
			return nil, fmt.Errorf("failed to compile policy %s (%w)", vp.GetName(), err.ToAggregate())
		}
		compiled = append(compiled, CompiledPolicy{
			Policy:         vp,
			CompiledPolicy: policy,
		})
	}
	provider := func(context.Context) ([]CompiledPolicy, error) {
		return compiled, nil
	}
	return provider, nil
}

func NewKubeProvider(compiler policy.Compiler, mgr ctrl.Manager) (Provider, error) {
	r := newPolicyReconciler(compiler, mgr.GetClient())
	if err := ctrl.NewControllerManagedBy(mgr).For(&kyvernov2alpha1.ValidatingPolicy{}).Complete(r); err != nil {
		return nil, fmt.Errorf("failed to construct manager: %w", err)
	}
	return r, nil
}

type policyReconciler struct {
	client       client.Client
	compiler     policy.Compiler
	lock         *sync.Mutex
	policies     map[string]CompiledPolicy
	sortPolicies func() []CompiledPolicy
}

func newPolicyReconciler(compiler policy.Compiler, client client.Client) *policyReconciler {
	return &policyReconciler{
		client:   client,
		compiler: compiler,
		lock:     &sync.Mutex{},
		policies: map[string]CompiledPolicy{},
		sortPolicies: func() []CompiledPolicy {
			return nil
		},
	}
}

func (r *policyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var policy kyvernov2alpha1.ValidatingPolicy
	// Reset the sorted func on every reconcile so the policies get resorted in next call
	resetSortPolicies := func() {
		r.sortPolicies = sync.OnceValue(func() []CompiledPolicy {
			r.lock.Lock()
			defer r.lock.Unlock()
			return mapToSortedSlice(r.policies)
		})
	}
	err := r.client.Get(ctx, req.NamespacedName, &policy)
	if errors.IsNotFound(err) {
		r.lock.Lock()
		defer r.lock.Unlock()
		defer resetSortPolicies()
		delete(r.policies, req.NamespacedName.String())
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	compiled, errs := r.compiler.Compile(&policy)
	if len(errs) > 0 {
		fmt.Println(errs)
		// No need to retry it
		return ctrl.Result{}, nil
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	r.policies[req.NamespacedName.String()] = CompiledPolicy{
		Policy:         policy,
		CompiledPolicy: compiled,
	}
	resetSortPolicies()
	return ctrl.Result{}, nil
}

func (r *policyReconciler) CompiledPolicies(ctx context.Context) ([]CompiledPolicy, error) {
	return slices.Clone(r.sortPolicies()), nil
}

func mapToSortedSlice[K cmp.Ordered, V any](in map[K]V) []V {
	if in == nil {
		return nil
	}
	out := make([]V, 0, len(in))
	keys := maps.Keys(in)
	slices.Sort(keys)
	for _, key := range keys {
		out = append(out, in[key])
	}
	return out
}
