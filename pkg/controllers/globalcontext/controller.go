package globalcontext

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov2beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2beta1"
	kyvernov2beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/globalcontext/externalapi"
	"github.com/kyverno/kyverno/pkg/globalcontext/k8sresource"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"go.opentelemetry.io/otel/metric"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers               = 1
	ControllerName        = "global-context"
	maxRetries            = 10
	storeFullRequeueDelay = 1 * time.Minute
)

type controller struct {
	// listers
	gceLister kyvernov2beta1listers.GlobalContextEntryLister

	// queue
	queue workqueue.TypedRateLimitingInterface[any]

	// state
	kubeClient         kubernetes.Interface
	dclient            dclient.Interface
	kyvernoClient      versioned.Interface
	store              store.Store
	eventGen           event.Interface
	maxResponseLength  int64
	apiCallTimeout     time.Duration
	shouldUpdateStatus bool
	jp                 jmespath.Interface
}

func NewController(
	gceInformer kyvernov2beta1informers.GlobalContextEntryInformer,
	kubeClient kubernetes.Interface,
	dclient dclient.Interface,
	kyvernoClient versioned.Interface,
	storage store.Store,
	eventGen event.Interface,
	maxResponseLength int64,
	apiCallTimeout time.Duration,
	shouldUpdateStatus bool,
	jp jmespath.Interface,
) controllers.Controller {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: ControllerName},
	)
	c := &controller{
		gceLister:          gceInformer.Lister(),
		queue:              queue,
		kubeClient:         kubeClient,
		dclient:            dclient,
		kyvernoClient:      kyvernoClient,
		store:              storage,
		eventGen:           eventGen,
		maxResponseLength:  maxResponseLength,
		apiCallTimeout:     apiCallTimeout,
		shouldUpdateStatus: shouldUpdateStatus,
		jp:                 jp,
	}

	if _, err := controllerutils.AddEventHandlersT(gceInformer.Informer(), c.addGTXEntry, c.updateGTXEntry, c.deleteGTXEntry); err != nil {
		logger.Error(err, "failed to register event handlers")
	}

	if m := metrics.GetGlobalContextMetrics(); m != nil {
		if _, err := m.RegisterCallback(c.reportProjectionMetrics); err != nil {
			logger.Error(err, "failed to register callback for global context projection metrics")
		}
	}

	return c
}

func (c *controller) addGTXEntry(obj *kyvernov2beta1.GlobalContextEntry) {
	logger.V(4).Info("globalcontextentry created", "uid", obj.GetUID(), "kind", obj.Kind, "name", obj.GetName())
	c.enqueueGCTXEntry(obj)
}

func (c *controller) updateGTXEntry(old, obj *kyvernov2beta1.GlobalContextEntry) {
	if datautils.DeepEqual(old.Spec, obj.Spec) {
		return
	}
	logger.V(4).Info("globalcontextentry updated", "uid", obj.GetUID(), "kind", obj.Kind, "name", obj.GetName())
	c.enqueueGCTXEntry(obj)
}

func (c *controller) deleteGTXEntry(obj *kyvernov2beta1.GlobalContextEntry) {
	logger.V(4).Info("globalcontextentry deleted", "uid", obj.GetUID(), "kind", obj.Kind, "name", obj.GetName())
	c.enqueueGCTXEntry(obj)
}

func (c *controller) enqueueGCTXEntry(gctxentry *kyvernov2beta1.GlobalContextEntry) {
	key, err := cache.MetaNamespaceKeyFunc(gctxentry)
	if err != nil {
		logger.Error(err, "failed to enqueue global context entry")
		return
	}
	c.queue.Add(key)
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, _, name string) error {
	gce, err := c.getEntry(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// entry was deleted, remove it from the store
			c.store.Delete(name)
			return nil
		}
		return err
	}
	if err := c.store.CheckCapacity(name); err != nil {
		if errors.Is(err, store.ErrStoreFull) {
			c.requeueStoreFull(logger, key)
			return nil
		}
		return err
	}
	// either it's a new entry or an existing entry changed
	// create a new element and set it in the store
	entry, err := c.makeStoreEntry(ctx, gce)
	if err != nil {
		return err
	}
	if err := c.store.Set(name, entry); err != nil {
		entry.Stop()
		if errors.Is(err, store.ErrStoreFull) {
			c.requeueStoreFull(logger, key)
			return nil
		}
		return err
	}
	return nil
}

// reportProjectionMetrics is the metrics.GlobalContextMetrics scrape callback, registered once
// in NewController. It recomputes, from every current GlobalContextEntry, how many projections
// use each language (jmespath vs cel) and - for jmespath projections - each of Kyverno's known
// custom JMESPath functions, and reports those as gauges. This is a scrape-time snapshot of
// current spec state, not a per-reconcile counter: the globalcontext controller is not
// leader-elected (runs on every replica), so a counter incremented per reconcile would multiply
// by replica count, while every replica recomputing the same gauge value from the same lister
// is harmless.
func (c *controller) reportProjectionMetrics(ctx context.Context, observer metric.Observer) error {
	m := metrics.GetGlobalContextMetrics()
	if m == nil {
		return nil
	}
	gces, err := c.gceLister.List(labels.Everything())
	if err != nil {
		logger.Error(err, "failed to list global context entries for projection metrics")
		return err
	}
	languageCounts := map[string]int{
		deprecations.ProjectionLanguageJMESPath: 0,
		deprecations.ProjectionLanguageCEL:      0,
	}
	functionCounts := map[string]int{}
	for _, gce := range gces {
		for _, p := range gce.Spec.Projections {
			language := deprecations.ProjectionLanguage(p.JMESPath, p.Expression)
			languageCounts[language]++
			if language != deprecations.ProjectionLanguageJMESPath {
				continue
			}
			for _, fn := range jmespath.UsedFunctionNames(p.JMESPath) {
				functionCounts[fn]++
			}
		}
	}
	for language, count := range languageCounts {
		m.ObserveProjectionLanguage(ctx, observer, language, count)
	}
	for fn, count := range functionCounts {
		m.ObserveJMESPathFunctionUsage(ctx, observer, fn, count)
	}
	return nil
}

func (c *controller) requeueStoreFull(logger logr.Logger, key any) {
	logger.V(3).Info("global context store is full, requeueing after delay", "key", key)
	c.queue.AddAfter(key, storeFullRequeueDelay)
}

func (c *controller) getEntry(name string) (*kyvernov2beta1.GlobalContextEntry, error) {
	return c.gceLister.Get(name)
}

func (c *controller) makeStoreEntry(ctx context.Context, gce *kyvernov2beta1.GlobalContextEntry) (store.Entry, error) {
	if gce.Spec.KubernetesResource != nil {
		gvr := schema.GroupVersionResource{
			Group:    gce.Spec.KubernetesResource.Group,
			Version:  gce.Spec.KubernetesResource.Version,
			Resource: gce.Spec.KubernetesResource.Resource,
		}
		return k8sresource.New(
			ctx,
			gce,
			c.eventGen,
			c.dclient.GetDynamicInterface(),
			logger,
			gvr,
			gce.Spec.KubernetesResource.Namespace,
			c.jp,
		)
	}
	return externalapi.New(
		ctx,
		gce,
		c.eventGen,
		c.kyvernoClient,
		c.gceLister,
		logger,
		adapters.Client(c.dclient),
		gce.Spec.APICall.APICall,
		gce.Spec.APICall.RefreshInterval.Duration,
		c.maxResponseLength,
		c.apiCallTimeout,
		c.shouldUpdateStatus,
		c.jp,
	)
}
