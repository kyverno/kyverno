package deleting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/cel/policies/dpol/engine"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/logging"
	pkgmetrics "github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/toggle"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"github.com/kyverno/kyverno/pkg/utils/restmapper"
	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
)

type controller struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface
	provider      engine.Provider
	engine        *engine.Engine

	// listers
	nsLister corev1listers.NamespaceLister

	// queue
	queue   workqueue.TypedRateLimitingInterface[any]
	enqueue controllerutils.EnqueueFuncT[*v1alpha1.DeletingPolicy]

	// config
	configuration config.Configuration
	cmResolver    engineapi.ConfigmapResolver
	eventGen      event.Interface
	metrics       pkgmetrics.DeletingMetrics
}

const (
	maxRetries     = 10
	Workers        = 3
	ControllerName = "deleting-controller"
)

func NewController(
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	polInformer kyvernov1alpha1informers.DeletingPolicyInformer,
	provider engine.Provider,
	engine *engine.Engine,
	nsLister corev1listers.NamespaceLister,
	configuration config.Configuration,
	cmResolver engineapi.ConfigmapResolver,
	eventGen event.Interface,
) controllers.Controller {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: ControllerName},
	)
	keyFunc := controllerutils.MetaNamespaceKeyT[*v1alpha1.DeletingPolicy]
	baseEnqueueFunc := controllerutils.LogError(logger, controllerutils.Parse(keyFunc, controllerutils.Queue(queue)))
	enqueueFunc := func(logger logr.Logger, operation, kind string) controllerutils.EnqueueFuncT[*v1alpha1.DeletingPolicy] {
		logger = logger.WithValues("kind", kind, "operation", operation)
		return func(obj *v1alpha1.DeletingPolicy) error {
			logger := logger.WithValues("name", obj.GetName())
			if obj.GetNamespace() != "" {
				logger = logger.WithValues("namespace", obj.GetNamespace())
			}
			logger.V(2).Info(operation)
			if err := baseEnqueueFunc(obj); err != nil {
				logger.Error(err, "failed to enqueue object", "obj", obj)
				return err
			}
			return nil
		}
	}
	c := &controller{
		client:        client,
		kyvernoClient: kyvernoClient,
		nsLister:      nsLister,
		queue:         queue,
		enqueue:       baseEnqueueFunc,
		configuration: configuration,
		cmResolver:    cmResolver,
		eventGen:      eventGen,
		metrics:       pkgmetrics.GetDeletingMetrics(),
		provider:      provider,
		engine:        engine,
	}
	if _, err := controllerutils.AddEventHandlersT(
		polInformer.Informer(),
		controllerutils.AddFuncT(logger, enqueueFunc(logger, "added", "DeletingPolicy")),
		controllerutils.UpdateFuncT(logger, enqueueFunc(logger, "updated", "DeletingPolicy")),
		controllerutils.DeleteFuncT(logger, enqueueFunc(logger, "deleted", "DeletingPolicy")),
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	return c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger.V(3), ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) deleting(ctx context.Context, logger logr.Logger, ePolicy engine.Policy) error {
	spec := ePolicy.Policy.GetDeletingPolicySpec()
	policy := ePolicy.Policy

	debug := logger.V(4)
	var errs []error
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: spec.DeletionPropagationPolicy,
	}

	if spec.MatchConstraints == nil {
		return errors.New("matchConstraints is required")
	}

	restMapper, err := restmapper.GetRESTMapper(c.client, false)
	if err != nil {
		return err
	}

	kinds := admissionpolicy.GetKinds(spec.MatchConstraints, restMapper)

	for _, kind := range kinds {
		debug := debug.WithValues("kind", kind)
		debug.Info("processing...")
		list, err := c.client.ListResource(ctx, "", kind, "", spec.MatchConstraints.ObjectSelector)
		if err != nil {
			debug.Error(err, "failed to list resources")
			errs = append(errs, err)
			// record failure metric
			if c.metrics != nil {
				c.metrics.RecordDeletingFailure(ctx, kind, "", policy, deleteOptions.PropagationPolicy)
			}
			// Check if this is a recoverable error (permission denied, resource not found, etc.)
			if dclient.IsRecoverableError(err) {
				logger.V(2).Info("skipping resource kind due to access restrictions", "kind", kind, "error", err.Error())
			} else {
				// For non-recoverable errors (connectivity issues, etc.), add to errors slice
				errs = append(errs, err)
			}

			continue
		}

		for i := range list.Items {
			resource := list.Items[i]

			namespace := resource.GetNamespace()
			name := resource.GetName()
			debug := logger.WithValues("name", name, "namespace", namespace)
			gvk := resource.GroupVersionKind()
			// Skip if resource matches resourceFilters from config
			if c.configuration.ToFilter(gvk, resource.GetKind(), namespace, name) {
				debug.Info("skipping resource due to resourceFilters in ConfigMap")
				continue
			}
			// check if the resource is owned by Kyverno
			if controllerutils.IsManagedByKyverno(&resource) && toggle.FromContext(ctx).ProtectManagedResources() {
				continue
			}

			engineResult, err := c.engine.Handle(ctx, ePolicy, resource)
			if err != nil {
				debug.Error(err, "failed to process resource")
				errs = append(errs, err)
				continue
			}

			if !engineResult.Match {
				debug.Error(err, "policy did not match match")
				errs = append(errs, err)
				continue
			}

			logger.WithValues("name", name, "namespace", namespace).Info("resource matched, it will be deleted...")
			if err := c.client.DeleteResource(ctx, resource.GetAPIVersion(), resource.GetKind(), namespace, name, false, deleteOptions); err != nil {
				if c.metrics != nil {
					c.metrics.RecordDeletingFailure(ctx, kind, namespace, policy, deleteOptions.PropagationPolicy)
				}
				debug.Error(err, "failed to delete resource")
				errs = append(errs, err)
				e := event.NewDeletingPolicyEvent(ePolicy.Policy, resource, err)
				c.eventGen.Add(e)
			} else {
				if c.metrics != nil {
					c.metrics.RecordDeletedObject(ctx, kind, namespace, policy, deleteOptions.PropagationPolicy)
				}
				debug.Info("resource deleted")
				e := event.NewDeletingPolicyEvent(ePolicy.Policy, resource, nil)
				c.eventGen.Add(e)
			}
		}
	}
	return multierr.Combine(errs...)
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	policy, err := c.provider.Get(ctx, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		logger.Error(err, "unable to get the policy from policy informer")
		return err
	}

	var nextExecutionTime *time.Time
	executionTime, err := policy.Policy.GetExecutionTime()
	if err != nil {
		logger.Error(err, "failed to get the policy execution time")
		return err
	}

	// In case it is the time to do the cleanup process
	if time.Now().After(*executionTime) {
		err := c.deleting(ctx, logger, policy)
		if err != nil {
			return err
		}
		if err := c.updateDeletingPolicyStatus(ctx, policy.Policy, *executionTime); err != nil {
			logger.Error(err, "failed to update the cleanup policy status")
			return err
		}
		nextExecutionTime, err = policy.Policy.GetNextExecutionTime(*executionTime)
		if err != nil {
			logger.Error(err, "failed to get the policy next execution time")
			return err
		}
	} else {
		nextExecutionTime = executionTime
	}
	// calculate the remaining time until deletion.
	timeRemaining := time.Until(*nextExecutionTime)
	// add the item back to the queue after the remaining time.
	c.queue.AddAfter(key, timeRemaining)
	return nil
}

func (c *controller) updateDeletingPolicyStatus(ctx context.Context, policy v1alpha1.DeletingPolicyLike, time time.Time) error {
	switch p := policy.(type) {
	case *v1alpha1.DeletingPolicy:
		err := controllerutils.UpdateStatus(ctx, p, c.kyvernoClient.PoliciesV1alpha1().DeletingPolicies(), func(p *v1alpha1.DeletingPolicy) error {
			p.Status = v1alpha1.DeletingPolicyStatus{
				LastExecutionTime: metav1.NewTime(time),
			}
			return nil
		}, func(current, expect *v1alpha1.DeletingPolicy) bool {
			return datautils.DeepEqual(current.Status, expect.Status)
		})
		if err != nil {
			return err
		}
		logging.Info("updated deleting policy status", "name", p.GetName(), "namespace", p.GetNamespace(), "status", p.Status)
	case *v1alpha1.NamespacedDeletingPolicy:
		err := controllerutils.UpdateStatus(ctx, p, c.kyvernoClient.PoliciesV1alpha1().NamespacedDeletingPolicies(p.GetNamespace()), func(p *v1alpha1.NamespacedDeletingPolicy) error {
			p.Status = v1alpha1.DeletingPolicyStatus{
				LastExecutionTime: metav1.NewTime(time),
			}
			return nil
		}, func(current, expect *v1alpha1.NamespacedDeletingPolicy) bool {
			return datautils.DeepEqual(current.Status, expect.Status)
		})
		if err != nil {
			return err
		}
		logging.Info("updated namespaced deleting policy status", "name", p.GetName(), "namespace", p.GetNamespace(), "status", p.Status)
	default:
		return fmt.Errorf("unsupported policy type: %T", policy)
	}
	return nil
}
