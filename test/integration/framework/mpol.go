package framework

import (
	"context"

	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

// NewMpolEngine creates an mpol engine using the real controller code path (NewKubeProvider).
// This mirrors the production wiring in cmd/kyverno/main.go:
// compiler → KubeProvider(openapi) → engine(typeConverter, contextProvider).
func NewMpolEngine(ctx context.Context, mgr ctrl.Manager, kubeClient kubernetes.Interface, contextProvider libs.Context) (mpolengine.Engine, mpolengine.Provider, error) {
	return newMpolEngine(ctx, mgr, kubeClient, contextProvider, nil, false)
}

// NewMpolEngineWithExceptions creates an mpol engine with PolicyException support enabled.
// This mirrors production wiring with celExceptionLister passed to NewKubeProvider.
func NewMpolEngineWithExceptions(ctx context.Context, mgr ctrl.Manager, kubeClient kubernetes.Interface, kyvernoClient kyvernoclient.Interface, contextProvider libs.Context) (mpolengine.Engine, mpolengine.Provider, error) {
	polexLister := NewPolexLister(ctx, kyvernoClient)
	return newMpolEngine(ctx, mgr, kubeClient, contextProvider, polexLister, true)
}

func newMpolEngine(ctx context.Context, mgr ctrl.Manager, kubeClient kubernetes.Interface, contextProvider libs.Context, polexLister celengine.PolicyExceptionLister, polexEnabled bool) (mpolengine.Engine, mpolengine.Provider, error) {
	compiler := mpolcompiler.NewCompiler()
	openapiClient := kubeClient.Discovery().OpenAPIV3()

	provider, typeConverter, err := mpolengine.NewKubeProvider(ctx, compiler, contextProvider, mgr, openapiClient, polexLister, polexEnabled)
	if err != nil {
		return nil, nil, err
	}

	nsResolver := func(ns string) *corev1.Namespace { return nil }
	matcher := matching.NewMatcher()

	return mpolengine.NewEngine(provider, nsResolver, matcher, typeConverter, contextProvider), provider, nil
}

// NewPolexLister creates a real informer-backed PolicyException lister,
// mirroring the production wiring: kyvernoClient → SharedInformerFactory → Lister.
func NewPolexLister(ctx context.Context, kyvernoClient kyvernoclient.Interface) celengine.PolicyExceptionLister {
	factory := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, 0)
	lister := factory.Policies().V1beta1().PolicyExceptions().Lister()
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	return celengine.NewPolicyExceptionLister(lister, "")
}
