package policystatus

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
	"github.com/kyverno/kyverno/pkg/policy/auth"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
)

const (
	ControllerName string = "status-controller"
	Workers        int    = 3
	maxRetries     int    = 3
	enqueueDelay          = 3 * time.Second
)

type Controller interface {
	controllers.Controller
}

type controller struct {
	dclient dclient.Interface
	client  versioned.Interface

	vpolLister  kyvernov2alpha1listers.ValidatingPolicyLister
	queue       workqueue.TypedRateLimitingInterface[any]
	authChecker auth.AuthChecks
}

func NewController(dclient dclient.Interface, client versioned.Interface, informer kyvernov2alpha1informers.ValidatingPolicyInformer) Controller {
	c := &controller{
		dclient: dclient,
		client:  client,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: ControllerName}),
		vpolLister:  informer.Lister(),
		authChecker: auth.NewAuth(dclient, "", logger),
	}

	enqueueFunc := controllerutils.LogError(logger, controllerutils.Parse(controllerutils.MetaNamespaceKey, controllerutils.Queue(c.queue)))
	_, err := controllerutils.AddEventHandlers(
		informer.Informer(),
		controllerutils.AddFunc(logger, enqueueFunc),
		controllerutils.UpdateFunc(logger, enqueueFunc),
		nil,
	)
	if err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	return c
}

func (c controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c controller) reconcile(ctx context.Context, logger logr.Logger, key string, namespace string, name string) error {
	vpol, err := c.vpolLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.V(4).Info("validating policy not found", "name", name)
			return nil
		}
		return err
	}

	status := vpol.GetStatus()
	ready := true
	for _, condition := range status.Conditions {
		if condition.Status != metav1.ConditionTrue {
			ready = false
			break
		}
	}

	updateFunc := func(vpol *kyvernov2alpha1.ValidatingPolicy) error {
		status := vpol.GetStatus()
		if status.Ready == nil || *status.Ready != ready {
			status.Ready = &ready
		}
		return nil
	}

	err = controllerutils.UpdateStatus(ctx,
		vpol,
		c.client.KyvernoV2alpha1().ValidatingPolicies(),
		updateFunc,
		func(current, expect *kyvernov2alpha1.ValidatingPolicy) bool {
			if current.GetStatus().Ready == nil {
				return false
			}
			return current.GetStatus().Ready == expect.GetStatus().Ready
		},
	)

	return err
}
