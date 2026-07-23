package framework

import (
	"github.com/kyverno/kyverno/pkg/cel/matching"
	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// secretLister builds the SecretLister the image verification engine expects. The engine only
// consults it to resolve registry pull credentials; the framework's test images are public, so an
// empty informer-backed lister is sufficient. Mirrors the lister setup.go builds for the context
// provider.
func secretLister(kubeClient kubernetes.Interface) corev1listers.SecretLister {
	return informers.NewSharedInformerFactory(kubeClient, 0).Core().V1().Secrets().Lister()
}

// NewIvpolEngine creates an ivpol engine using the real controller code path (NewKubeProvider),
// mirroring the production wiring in cmd/kyverno/main.go: KubeProvider → engine. It passes a Secret
// lister (for registry pull credentials, empty here since the test images are public) and a disabled
// image-verify cache so results stay deterministic. The returned provider exposes Fetch() to poll
// reconciliation.
func NewIvpolEngine(mgr ctrl.Manager, kubeClient kubernetes.Interface) (ivpolengine.Engine, ivpolengine.Provider, error) {
	provider, err := ivpolengine.NewKubeProvider(mgr, nil, false)
	if err != nil {
		return nil, nil, err
	}

	nsResolver := func(ns string) *corev1.Namespace { return nil }
	engine := ivpolengine.NewEngine(provider, nsResolver, matching.NewMatcher(), secretLister(kubeClient), imageverifycache.DisabledImageVerifyCache())
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
	engine := ivpolengine.NewEngine(provider, nsResolver, matching.NewMatcher(), secretLister(kubeClient), imageverifycache.DisabledImageVerifyCache())
	return engine, provider, nil
}
