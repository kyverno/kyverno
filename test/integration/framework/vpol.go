package framework

import (
	"github.com/kyverno/kyverno/pkg/cel/matching"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
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

	nsResolver := func(ns string) *corev1.Namespace { return nil }
	matcher := matching.NewMatcher()

	return vpolengine.NewEngine(provider, nsResolver, matcher), provider, nil
}
