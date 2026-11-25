package framework

import (
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	ctrl "sigs.k8s.io/controller-runtime"
)

// NewVpolEngine creates a vpol engine using the real controller code path (NewKubeProvider).
// This mirrors the production wiring in cmd/kyverno/main.go: compiler → KubeProvider → engine,
// including the namespace resolver that backs namespaceSelector matching and namespaceObject.
// The returned provider exposes Fetch() to check reconciliation status in tests.
func NewVpolEngine(mgr ctrl.Manager) (vpolengine.Engine, vpolengine.Provider, error) {
	compiler := vpolcompiler.NewCompiler()
	provider, err := vpolengine.NewKubeProvider(compiler, mgr, nil, false)
	if err != nil {
		return nil, nil, err
	}

	nsResolver, err := namespaceResolverForManager(mgr)
	if err != nil {
		return nil, nil, err
	}
	matcher := matching.NewMatcher()

	return vpolengine.NewEngine(provider, nsResolver, matcher), provider, nil
}

// NewVpolEngineWithExceptions creates a vpol engine with PolicyException support enabled.
// Uses the manager's cache as the exception lister (celengine.NewManagerPolicyExceptionLister,
// the same helper production wiring uses) so the controller watches and lister share
// one cache — no dual-cache race conditions.
func NewVpolEngineWithExceptions(mgr ctrl.Manager) (vpolengine.Engine, vpolengine.Provider, error) {
	polexLister := celengine.NewManagerPolicyExceptionLister(mgr.GetClient(), "")

	compiler := vpolcompiler.NewCompiler()
	provider, err := vpolengine.NewKubeProvider(compiler, mgr, polexLister, true)
	if err != nil {
		return nil, nil, err
	}

	nsResolver, err := namespaceResolverForManager(mgr)
	if err != nil {
		return nil, nil, err
	}
	matcher := matching.NewMatcher()

	return vpolengine.NewEngine(provider, nsResolver, matcher), provider, nil
}
