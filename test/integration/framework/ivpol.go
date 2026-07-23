package framework

import (
	"github.com/kyverno/kyverno/pkg/cel/matching"
	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	"github.com/kyverno/kyverno/pkg/config"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

// NewIvpolEngine creates an ivpol engine using the real controller code path (NewKubeProvider),
// mirroring the production wiring in cmd/kyverno/main.go: KubeProvider → engine.
//
// Three arguments are unique to ivpol (vpol/mpol don't need them): a Secret lister for registry
// pull credentials, registry options, and an image-verify cache. Production passes a real Secrets
// lister and cache (main.go:781-783); here we pass the same Secrets lister for fidelity (it is only
// queried when a policy references pull secrets, so it is harmless for the public test images),
// nil registry options (anonymous access to public registries), and a disabled cache so results
// stay deterministic across runs. The returned provider exposes Fetch() to poll reconciliation.
func NewIvpolEngine(mgr ctrl.Manager, kubeClient kubernetes.Interface) (ivpolengine.Engine, ivpolengine.Provider, error) {
	provider, err := ivpolengine.NewKubeProvider(mgr, nil, false)
	if err != nil {
		return nil, nil, err
	}

	nsResolver := func(ns string) *corev1.Namespace { return nil }
	lister := kubeClient.CoreV1().Secrets(config.KyvernoNamespace())

	engine := ivpolengine.NewEngine(provider, nsResolver, matching.NewMatcher(), lister, nil, imageverifycache.DisabledImageVerifyCache())
	return engine, provider, nil
}

// NewIvpolEngineWithExceptions creates an ivpol engine with PolicyException support enabled.
// Reuses managerPolexLister (defined in vpol.go) so the controller watches and the lister share
// the manager's cache, avoiding the dual-cache race.
func NewIvpolEngineWithExceptions(mgr ctrl.Manager, kubeClient kubernetes.Interface) (ivpolengine.Engine, ivpolengine.Provider, error) {
	polexLister := &managerPolexLister{client: mgr.GetClient()}

	provider, err := ivpolengine.NewKubeProvider(mgr, polexLister, true)
	if err != nil {
		return nil, nil, err
	}

	nsResolver := func(ns string) *corev1.Namespace { return nil }
	lister := kubeClient.CoreV1().Secrets(config.KyvernoNamespace())

	engine := ivpolengine.NewEngine(provider, nsResolver, matching.NewMatcher(), lister, nil, imageverifycache.DisabledImageVerifyCache())
	return engine, provider, nil
}
