package framework

import (
	"context"

	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
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
// Uses celengine.NewManagerPolicyExceptionLister (mirroring production wiring in
// cmd/kyverno/main.go) so the controller watches and the lister share the manager's
// cache, avoiding the dual-cache race described in issue #16989.
func NewMpolEngineWithExceptions(ctx context.Context, mgr ctrl.Manager, kubeClient kubernetes.Interface, contextProvider libs.Context) (mpolengine.Engine, mpolengine.Provider, error) {
	polexLister := celengine.NewManagerPolicyExceptionLister(mgr.GetClient(), "")
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
