package globalcontext

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	kyvernov2alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2alpha1"
	kyvernov2alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	"github.com/kyverno/kyverno/pkg/engine/globalcontext/externalapi"
	"github.com/kyverno/kyverno/pkg/engine/globalcontext/k8sresource"
	"github.com/kyverno/kyverno/pkg/engine/globalcontext/store"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	queue workqueue.RateLimitingInterface

	// state
	dclient dclient.Interface
	store   store.Store
}

func NewController(
	gceInformer kyvernov2alpha1informers.GlobalContextEntryInformer,
	dclient dclient.Interface,
	storage store.Store,
) controllers.Controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName)
	_, _, err := controllerutils.AddDefaultEventHandlers(logger, gceInformer.Informer(), queue)
	if err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	return &controller{
		gceLister: gceInformer.Lister(),
		queue:     queue,
		dclient:   dclient,
		store:     storage,
	}
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
	// TODO: should be done at validation time
	if gce.Spec.KubernetesResource == nil && gce.Spec.APICall == nil {
		return nil, errors.New("global context entry neither has K8sResource nor APICall")
	}
	// TODO: should be done at validation time
	if gce.Spec.KubernetesResource != nil && gce.Spec.APICall != nil {
		return nil, errors.New("global context entry has both K8sResource and APICall")
	}
	if gce.Spec.KubernetesResource != nil {
		gvr := schema.GroupVersionResource{
			Group:    gce.Spec.KubernetesResource.Group,
			Version:  gce.Spec.KubernetesResource.Version,
			Resource: gce.Spec.KubernetesResource.Resource,
		}
		return k8sresource.New(ctx, c.dclient.GetDynamicInterface(), gvr, gce.Spec.KubernetesResource.Namespace)
	}
	return externalapi.New(ctx, logger, adapters.Client(c.dclient), gce.Spec.APICall.APICall, time.Duration(gce.Spec.APICall.RefreshIntervalSeconds))
}
