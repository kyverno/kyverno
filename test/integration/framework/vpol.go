package framework

import (
	"context"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/polex"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewVpolEngine creates a vpol engine using the real controller code path (NewKubeProvider).
// This mirrors the production wiring in cmd/kyverno/main.go: compiler → KubeProvider → engine.
// The returned provider exposes Fetch() to check reconciliation status in tests.
func NewVpolEngine(mgr ctrl.Manager) (vpolengine.Engine, vpolengine.Provider, error) {
	compiler := vpolcompiler.NewCompiler()
	provider, err := vpolengine.NewKubeProvider(compiler, mgr)
	if err != nil {
		return nil, nil, err
	}
	polexCompiler := polex.NewCompiler()
	polexProvider, err := polex.NewKubeProvider(polexCompiler, mgr)
	if err != nil {
		return nil, nil, err
	}
	nsResolver := func(ns string) *corev1.Namespace { return nil }
	matcher := matching.NewMatcher()

	return vpolengine.NewEngine(provider, polexProvider, nsResolver, matcher), provider, nil
}

// managerPolexLister implements engine.PolicyExceptionLister using the manager's
// cache client. This avoids the dual-cache race condition that occurs when using
// a separate informer factory — the controller watches and the lister both read
// from the same cache, so exceptions are visible when reconciliation triggers.
type managerPolexLister struct {
	client client.Reader
}

func (l *managerPolexLister) List(selector labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	var list policiesv1beta1.PolicyExceptionList
	if err := l.client.List(context.Background(), &list); err != nil {
		return nil, err
	}
	result := make([]*policiesv1beta1.PolicyException, 0, len(list.Items))
	for i := range list.Items {
		if selector.Matches(labels.Set(list.Items[i].Labels)) {
			result = append(result, &list.Items[i])
		}
	}
	return result, nil
}

// Ensure managerPolexLister satisfies the interface at compile time.
var _ engine.PolicyExceptionLister = (*managerPolexLister)(nil)

// NewVpolEngineWithExceptions creates a vpol engine with PolicyException support enabled.
// Uses the manager's cache as the exception lister so the controller watches and
// lister share one cache — no dual-cache race conditions.
func NewVpolEngineWithExceptions(mgr ctrl.Manager) (vpolengine.Engine, vpolengine.Provider, error) {
	polexLister := &managerPolexLister{client: mgr.GetClient()}

	compiler := vpolcompiler.NewCompiler()
	provider, err := vpolengine.NewKubeProvider(compiler, mgr, polexLister, true)
	if err != nil {
		return nil, nil, err
	}

	nsResolver := func(ns string) *corev1.Namespace { return nil }
	matcher := matching.NewMatcher()

	return vpolengine.NewEngine(provider, nsResolver, matcher), provider, nil
}
