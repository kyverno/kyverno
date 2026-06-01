package framework

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	gpolbg "github.com/kyverno/kyverno/pkg/background/gpol"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	gpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	gpolengine "github.com/kyverno/kyverno/pkg/cel/policies/gpol/engine"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
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

// NewGpolPolexLister creates an informer-backed PolicyException lister from a Kyverno clientset.
// Mirrors the production wiring in cmd/background-controller/main.go:
// kyvernoInformer.Policies().V1beta1().PolicyExceptions().Lister()
func NewGpolPolexLister(ctx context.Context, kyvernoClient kyvernoclient.Interface) celengine.PolicyExceptionLister {
	factory := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, 0)
	polexLister := factory.Policies().V1beta1().PolicyExceptions().Lister()
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	return polexLister
}

// NewGpolEngineWithExceptions creates a gpol engine and provider with PolicyException support.
// When no exceptions exist, behavior is identical to NewGpolEngine.
func NewGpolEngineWithExceptions(
	gpolLister policiesv1beta1listers.GeneratingPolicyLister,
	ngpolLister policiesv1beta1listers.NamespacedGeneratingPolicyLister,
	polexLister celengine.PolicyExceptionLister,
) (gpolengine.Engine, gpolengine.Provider) {
	compiler := gpolcompiler.NewCompiler()
	provider := gpolengine.NewFetchProvider(compiler, gpolLister, ngpolLister, polexLister, true)
	nsResolver := func(ns string) *corev1.Namespace { return nil }
	matcher := matching.NewMatcher()
	engine := gpolengine.NewEngine(nsResolver, matcher)
	return engine, provider
}

// NewURProcessor returns a closure that processes URSpecs through the gpol engine,
// creating downstream resources in envtest. Mirrors CELGenerateController.ProcessUR
// minus the sync watchers; use NewURProcessorWithSyncWatchers for sync scenarios.
func NewURProcessor(
	engine gpolengine.Engine,
	provider gpolengine.Provider,
	contextProvider libs.Context,
) func(kyvernov2.UpdateRequestSpec) error {
	return urProcessorClosure(engine, provider, contextProvider, nil)
}

// NewGpolWatchManager builds the production WatchManager against the framework's
// dclient and returns its StopWatchers for t.Cleanup. Must be called after
// TestEnv.Start because the WatchManager constructor reads dclient discovery.
func NewGpolWatchManager(client dclient.Interface, log logr.Logger) (*gpolbg.WatchManager, func()) {
	wm := gpolbg.NewWatchManager(log, client)
	return wm, wm.StopWatchers
}

// NewURProcessorWithSyncWatchers extends NewURProcessor to call DeleteDownstreams
// and SyncWatchers around engine.Handle, matching production at
// pkg/background/gpol/generate_controller.go:87-170.
func NewURProcessorWithSyncWatchers(
	engine gpolengine.Engine,
	provider gpolengine.Provider,
	contextProvider libs.Context,
	wm *gpolbg.WatchManager,
	log logr.Logger,
) func(kyvernov2.UpdateRequestSpec) error {
	return urProcessorClosure(engine, provider, contextProvider, wm)
}

// urProcessorClosure runs the gpol engine over a URSpec. When wm is non-nil it
// also performs the DeleteDownstreams + SyncWatchers wiring that the production
// CELGenerateController applies.
func urProcessorClosure(
	engine gpolengine.Engine,
	provider gpolengine.Provider,
	contextProvider libs.Context,
	wm *gpolbg.WatchManager,
) func(kyvernov2.UpdateRequestSpec) error {
	return func(spec kyvernov2.UpdateRequestSpec) error {
		for i := range spec.RuleContext {
			rc := &spec.RuleContext[i]
			if rc.DeleteDownstream {
				if wm != nil {
					wm.DeleteDownstreams(spec.GetPolicyKey(), &rc.Trigger)
				}
				continue
			}
			if rc.Synchronize && wm != nil {
				wm.DeleteDownstreams(spec.GetPolicyKey(), &rc.Trigger)
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
			resp, err := engine.Handle(request, policy, rc.CacheRestore)
			if err != nil {
				return fmt.Errorf("failed to process UR for policy %s: %w", spec.GetPolicyKey(), err)
			}
			if wm == nil {
				continue
			}
			isSync := policy.Policy.GetSpec().SynchronizationEnabled()
			if !isSync {
				continue
			}
			for _, res := range resp.Policies {
				if res.Result == nil {
					continue
				}
				generated := res.Result.GeneratedResources()
				if len(generated) == 0 {
					continue
				}
				if err := wm.SyncWatchers(spec.GetPolicyKey(), generated); err != nil {
					return fmt.Errorf("failed to sync watchers for policy %s: %w", spec.GetPolicyKey(), err)
				}
			}
		}
		return nil
	}
}
