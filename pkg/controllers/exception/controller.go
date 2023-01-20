package exception

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	kyvernov2alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2alpha1"
	kyvernov2alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/controllers"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 3
	ControllerName = "exception-controller"
	maxRetries     = 10
)

type controller struct {
	// clients
	polexClients func(string) controllerutils.StatusClient[*kyvernov2alpha1.PolicyException]

	// listers
	polexLister kyvernov2alpha1listers.PolicyExceptionLister

	// queue
	queue workqueue.RateLimitingInterface

	// config
	enabled   bool
	namespace string
}

func NewController(
	polexClients func(string) controllerutils.StatusClient[*kyvernov2alpha1.PolicyException],
	polexInformer kyvernov2alpha1informers.PolicyExceptionInformer,
	enabled bool,
	namespace string,
) controllers.Controller {
	c := controller{
		polexClients: polexClients,
		polexLister:  polexInformer.Lister(),
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
		enabled:      enabled,
		namespace:    namespace,
	}
	controllerutils.AddDefaultEventHandlers(logger, polexInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	polex, err := c.polexLister.PolicyExceptions(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		logger.Error(err, "unable to lookup policy exception")
		return err
	}
	if _, err := controllerutils.UpdateStatus(ctx, polex, c.polexClients(namespace), func(polex *kyvernov2alpha1.PolicyException) error {
		ready := true
		if !c.enabled {
			ready = false
		} else if c.namespace != "" && c.namespace != namespace {
			ready = false
		}
		polex.Status.SetReady(ready)
		return nil
	}); err != nil {
		return err
	}
	return nil
}
