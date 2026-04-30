package framework

import (
	"context"
	"sync"

	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	dpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	dpolengine "github.com/kyverno/kyverno/pkg/cel/policies/dpol/engine"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	kyvernov1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/controllers/deleting"
	"github.com/kyverno/kyverno/pkg/event"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// ThreadSafeEventCapture captures events for test assertions.
// The deleting controller has multiple workers, so this must be thread-safe.
type ThreadSafeEventCapture struct {
	mu     sync.Mutex
	events []event.Info
}

func (c *ThreadSafeEventCapture) Add(infoList ...event.Info) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, infoList...)
}

func (c *ThreadSafeEventCapture) GetEvents() []event.Info {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]event.Info, len(c.events))
	copy(result, c.events)
	return result
}

func (c *ThreadSafeEventCapture) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = nil
}

// DpolDeps holds all dependencies created for dpol controller integration testing.
type DpolDeps struct {
	DpolLister   policiesv1beta1listers.DeletingPolicyLister
	NdpolLister  policiesv1beta1listers.NamespacedDeletingPolicyLister
	NsLister     corev1listers.NamespaceLister
	Controller   controllers.Controller
	EventCapture *ThreadSafeEventCapture
}

// NewDpolDeps creates the complete dependency graph for the dpol controller,
// mirroring cmd/cleanup-controller/main.go wiring. Uses a single shared
// kyvernoinformer factory so the controller's event handlers and FetchProvider
// share the same cache.
func NewDpolDeps(
	ctx context.Context,
	dc dclient.Interface,
	kyvernoClient kyvernoclient.Interface,
	kubeClient kubernetes.Interface,
	restMapper meta.RESTMapper,
	contextProvider libs.Context,
) *DpolDeps {
	// Kyverno informer factory (shared between controller informers and provider listers)
	kyvernoFactory := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, 0)
	dpolInformer := kyvernoFactory.Policies().V1beta1().DeletingPolicies()
	ndpolInformer := kyvernoFactory.Policies().V1beta1().NamespacedDeletingPolicies()

	// Provider: compiles policies from listers on each Get()
	compiler := dpolcompiler.NewCompiler()
	provider := dpolengine.NewFetchProvider(compiler, dpolInformer.Lister(), ndpolInformer.Lister(), nil, false)

	// Kube informer factory for namespace lister
	kubeFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	nsLister := kubeFactory.Core().V1().Namespaces().Lister()

	nsResolver := func(name string) *corev1.Namespace {
		ns, err := nsLister.Get(name)
		if err != nil {
			return nil
		}
		return ns
	}

	engine := dpolengine.NewEngine(nsResolver, restMapper, contextProvider, matching.NewMatcher())
	eventCapture := &ThreadSafeEventCapture{}

	controller := deleting.NewController(
		dc,
		kyvernoClient,
		dpolInformer,
		ndpolInformer,
		provider,
		engine,
		nsLister,
		config.NewDefaultConfiguration(false),
		nil, // cmResolver not used by deleting controller
		eventCapture,
	)

	kyvernoFactory.Start(ctx.Done())
	kubeFactory.Start(ctx.Done())
	for _, synced := range kyvernoFactory.WaitForCacheSync(ctx.Done()) {
		if !synced {
			panic("failed to sync kyverno informer caches")
		}
	}
	for _, synced := range kubeFactory.WaitForCacheSync(ctx.Done()) {
		if !synced {
			panic("failed to sync kube informer caches")
		}
	}

	return &DpolDeps{
		DpolLister:   dpolInformer.Lister(),
		NdpolLister:  ndpolInformer.Lister(),
		NsLister:     nsLister,
		Controller:   controller,
		EventCapture: eventCapture,
	}
}

// NewDpolDepsWithExceptions creates dpol deps with PolicyException support enabled.
func NewDpolDepsWithExceptions(
	ctx context.Context,
	dc dclient.Interface,
	kyvernoClient kyvernoclient.Interface,
	kubeClient kubernetes.Interface,
	restMapper meta.RESTMapper,
	contextProvider libs.Context,
) (*DpolDeps, kyvernov1beta1informers.PolicyExceptionInformer) {
	kyvernoFactory := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, 0)
	dpolInformer := kyvernoFactory.Policies().V1beta1().DeletingPolicies()
	ndpolInformer := kyvernoFactory.Policies().V1beta1().NamespacedDeletingPolicies()
	polexInformer := kyvernoFactory.Policies().V1beta1().PolicyExceptions()

	compiler := dpolcompiler.NewCompiler()
	provider := dpolengine.NewFetchProvider(compiler, dpolInformer.Lister(), ndpolInformer.Lister(), polexInformer.Lister(), true)

	kubeFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	nsLister := kubeFactory.Core().V1().Namespaces().Lister()

	nsResolver := func(name string) *corev1.Namespace {
		ns, err := nsLister.Get(name)
		if err != nil {
			return nil
		}
		return ns
	}

	engine := dpolengine.NewEngine(nsResolver, restMapper, contextProvider, matching.NewMatcher())
	eventCapture := &ThreadSafeEventCapture{}

	controller := deleting.NewController(
		dc,
		kyvernoClient,
		dpolInformer,
		ndpolInformer,
		provider,
		engine,
		nsLister,
		config.NewDefaultConfiguration(false),
		nil,
		eventCapture,
	)

	kyvernoFactory.Start(ctx.Done())
	kubeFactory.Start(ctx.Done())
	for _, synced := range kyvernoFactory.WaitForCacheSync(ctx.Done()) {
		if !synced {
			panic("failed to sync kyverno informer caches")
		}
	}
	for _, synced := range kubeFactory.WaitForCacheSync(ctx.Done()) {
		if !synced {
			panic("failed to sync kube informer caches")
		}
	}

	return &DpolDeps{
		DpolLister:   dpolInformer.Lister(),
		NdpolLister:  ndpolInformer.Lister(),
		NsLister:     nsLister,
		Controller:   controller,
		EventCapture: eventCapture,
	}, polexInformer
}
