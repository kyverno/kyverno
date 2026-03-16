package framework

import (
	"context"

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
	compiler := mpolcompiler.NewCompiler()
	openapiClient := kubeClient.Discovery().OpenAPIV3()

	provider, typeConverter, err := mpolengine.NewKubeProvider(ctx, compiler, mgr, openapiClient, nil, false)
	if err != nil {
		return nil, nil, err
	}

	nsResolver := func(ns string) *corev1.Namespace { return nil }
	matcher := matching.NewMatcher()

	return mpolengine.NewEngine(provider, nsResolver, matcher, typeConverter, contextProvider), provider, nil
}
