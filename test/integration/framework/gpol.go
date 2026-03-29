package framework

import (
	"context"
	"fmt"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	gpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	gpolengine "github.com/kyverno/kyverno/pkg/cel/policies/gpol/engine"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// NewGpolListers creates real informer-backed listers from a Kyverno clientset,
// mirroring the production wiring in cmd/kyverno/main.go:
// kyvernoClient → SharedInformerFactory → Listers.
func NewGpolListers(ctx context.Context, kyvernoClient kyvernoclient.Interface) (
	policiesv1beta1listers.GeneratingPolicyLister,
	policiesv1beta1listers.NamespacedGeneratingPolicyLister,
) {
	factory := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, 0)
	gpolLister := factory.Policies().V1beta1().GeneratingPolicies().Lister()
	ngpolLister := factory.Policies().V1beta1().NamespacedGeneratingPolicies().Lister()
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	return gpolLister, ngpolLister
}

// NewGpolEngine creates a gpol engine and provider using informer-backed listers.
// Mirrors the production wiring in cmd/background-controller/main.go:
// compiler → NewFetchProvider(listers) → NewEngine(nsResolver, matcher).
func NewGpolEngine(
	gpolLister policiesv1beta1listers.GeneratingPolicyLister,
	ngpolLister policiesv1beta1listers.NamespacedGeneratingPolicyLister,
) (gpolengine.Engine, gpolengine.Provider) {
	compiler := gpolcompiler.NewCompiler()
	provider := gpolengine.NewFetchProvider(compiler, gpolLister, ngpolLister, nil, false)
	nsResolver := func(ns string) *corev1.Namespace { return nil }
	matcher := matching.NewMatcher()
	engine := gpolengine.NewEngine(nsResolver, matcher)
	return engine, provider
}

// NewURProcessor returns a function that processes URSpecs through the gpol engine,
// creating downstream resources in the envtest cluster. This simulates what the
// background controller's CELGenerateController.ProcessUR does in production:
// fetch policy → build engine request from admission request → engine.Handle()
// → generator.Apply() CEL function → ContextProvider.CreateResource() via dclient.
func NewURProcessor(
	engine gpolengine.Engine,
	provider gpolengine.Provider,
	contextProvider libs.Context,
) func(kyvernov2.UpdateRequestSpec) error {
	return func(spec kyvernov2.UpdateRequestSpec) error {
		for _, rc := range spec.RuleContext {
			if rc.DeleteDownstream {
				continue
			}
			admissionRequest := spec.Context.AdmissionRequestInfo.AdmissionRequest
			if admissionRequest == nil {
				return fmt.Errorf("no admission request in UR context for policy %s", spec.Policy)
			}
			contextProvider.ClearGeneratedResources()
			request := celengine.RequestFromAdmission(contextProvider, *admissionRequest)
			policy, err := provider.Get(context.TODO(), spec.GetPolicyKey())
			if err != nil {
				return fmt.Errorf("failed to fetch policy %s: %w", spec.GetPolicyKey(), err)
			}
			_, err = engine.Handle(request, policy, rc.CacheRestore)
			if err != nil {
				return fmt.Errorf("failed to process UR for policy %s: %w", spec.GetPolicyKey(), err)
			}
		}
		return nil
	}
}
