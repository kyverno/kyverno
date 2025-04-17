package globalcontext

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov2alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2alpha1"
	kyvernov2alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/globalcontext/externalapi"
	"github.com/kyverno/kyverno/pkg/globalcontext/k8sresource"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 1
	ControllerName = "global-context"
	maxRetries     = 10
)

type controller struct {
	// listers
	gceLister kyvernov2alpha1listers.GlobalContextEntryLister

	// queue
	queue workqueue.TypedRateLimitingInterface[any]

	// state
	dclient            dclient.Interface
	kyvernoClient      versioned.Interface
	store              store.Store
	eventGen           event.Interface
	maxResponseLength  int64
	shouldUpdateStatus bool
}

func NewController(
	gceInformer kyvernov2alpha1informers.GlobalContextEntryInformer,
	dclient dclient.Interface,
	kyvernoClient versioned.Interface,
	storage store.Store,
	eventGen event.Interface,
	maxResponseLength int64,
	shouldUpdateStatus bool,
) controllers.Controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[any](), ControllerName)
	c := &controller{
		gceLister:          gceInformer.Lister(),
		queue:              queue,
		dclient:            dclient,
		kyvernoClient:      kyvernoClient,
		store:              storage,
		eventGen:           eventGen,
		maxResponseLength:  maxResponseLength,
		shouldUpdateStatus: shouldUpdateStatus,
	}

	if _, err := controllerutils.AddEventHandlersT(gceInformer.Informer(), c.addGTXEntry, c.updateGTXEntry, c.deleteGTXEntry); err != nil {
		logger.Error(err, "failed to register event handlers")
	}

	return c
}

func (c *controller) addGTXEntry(obj *kyvernov2alpha1.GlobalContextEntry) {
	logger.Info("globalcontextentry created", "uid", obj.GetUID(), "kind", obj.Kind, "name", obj.GetName())
	c.enqueueGCTXEntry(obj)
}

func (c *controller) updateGTXEntry(old, obj *kyvernov2alpha1.GlobalContextEntry) {
	if datautils.DeepEqual(old.Spec, obj.Spec) {
		return
	}
	logger.Info("globalcontextentry updated", "uid", obj.GetUID(), "kind", obj.Kind, "name", obj.GetName())
	c.enqueueGCTXEntry(obj)
}

func (c *controller) deleteGTXEntry(obj *kyvernov2alpha1.GlobalContextEntry) {
	logger.Info("globalcontextentry deleted", "uid", obj.GetUID(), "kind", obj.Kind, "name", obj.GetName())
	c.enqueueGCTXEntry(obj)
}

func (c *controller) enqueueGCTXEntry(gctxentry *kyvernov2alpha1.GlobalContextEntry) {
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
	// either it's a new entry or an existing entry changed
	// create a new element and set it in the store
	entry, err := c.makeStoreEntry(ctx, gce)
	if err != nil {
		return err
	}
	c.store.Set(name, entry)
	return nil
}

func (c *controller) getEntry(name string) (*kyvernov2alpha1.GlobalContextEntry, error) {
	return c.gceLister.Get(name)
}

func (c *controller) makeStoreEntry(ctx context.Context, gce *kyvernov2alpha1.GlobalContextEntry) (store.Entry, error) {
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
			c.kyvernoClient,
			logger,
			gvr,
			gce.Spec.KubernetesResource.Namespace,
			c.shouldUpdateStatus,
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
		c.shouldUpdateStatus,
	)
}
