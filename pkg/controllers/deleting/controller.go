package deleting

import (
	"context"
	"errors"
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
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/toggle"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"github.com/kyverno/kyverno/pkg/utils/restmapper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
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
	metrics       deletingMetrics
}

type deletingMetrics struct {
	deletedObjectsTotal   metric.Int64Counter
	deletingFailuresTotal metric.Int64Counter
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
		metrics:       newDeletignMetrics(logger),
		provider:      provider,
		engine:        engine,
	}
	if _, err := controllerutils.AddEventHandlersT(
		polInformer.Informer(),
		controllerutils.AddFuncT(logger, enqueueFunc(logger, "added", "DeletigPolicy")),
		controllerutils.UpdateFuncT(logger, enqueueFunc(logger, "updated", "DeletigPolicy")),
		controllerutils.DeleteFuncT(logger, enqueueFunc(logger, "deleted", "DeletigPolicy")),
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	return c
}

func newDeletignMetrics(logger logr.Logger) deletingMetrics {
	meter := otel.GetMeterProvider().Meter(metrics.MeterName)
	deletedObjectsTotal, err := meter.Int64Counter(
		"kyverno_deleting_controller_deletedobjects",
		metric.WithDescription("can be used to track number of deleted objects."),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, cleanup_controller_deletedobjects_total")
	}
	cleanupFailuresTotal, err := meter.Int64Counter(
		"kyverno_deleting_controller_errors",
		metric.WithDescription("can be used to track number of cleanup failures."),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, cleanup_controller_errors_total")
	}
	return deletingMetrics{
		deletedObjectsTotal:   deletedObjectsTotal,
		deletingFailuresTotal: cleanupFailuresTotal,
	}
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger.V(3), ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) deleting(ctx context.Context, logger logr.Logger, ePolicy engine.Policy) error {
	spec := ePolicy.Policy.Spec
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

	kinds, err := admissionpolicy.GetKinds(spec.MatchConstraints, restMapper)
	if err != nil {
		return err
	}

	for _, kind := range kinds {
		commonLabels := []attribute.KeyValue{
			attribute.String("policy_type", policy.Kind),
			attribute.String("policy_namespace", policy.GetNamespace()),
			attribute.String("policy_name", policy.GetName()),
			attribute.String("resource_kind", kind),
		}
		debug := debug.WithValues("kind", kind)
		debug.Info("processing...")
		list, err := c.client.ListResource(ctx, "", kind, "", policy.Spec.MatchConstraints.ObjectSelector)
		if err != nil {
			debug.Error(err, "failed to list resources")
			errs = append(errs, err)
			if c.metrics.deletingFailuresTotal != nil {
				c.metrics.deletingFailuresTotal.Add(ctx, 1, metric.WithAttributes(commonLabels...))
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

			var labels []attribute.KeyValue
			labels = append(labels, commonLabels...)
			labels = append(labels, attribute.String("resource_namespace", namespace))
			if deleteOptions.PropagationPolicy != nil {
				labels = append(labels, attribute.String("deletion_policy", string(*deleteOptions.PropagationPolicy)))
			}
			logger.WithValues("name", name, "namespace", namespace).Info("resource matched, it will be deleted...")
			if err := c.client.DeleteResource(ctx, resource.GetAPIVersion(), resource.GetKind(), namespace, name, false, deleteOptions); err != nil {
				if c.metrics.deletingFailuresTotal != nil {
					c.metrics.deletingFailuresTotal.Add(ctx, 1, metric.WithAttributes(labels...))
				}
				debug.Error(err, "failed to delete resource")
				errs = append(errs, err)
				e := event.NewDeletingPolicyEvent(ePolicy.Policy, resource, err)
				c.eventGen.Add(e)
			} else {
				if c.metrics.deletedObjectsTotal != nil {
					c.metrics.deletedObjectsTotal.Add(ctx, 1, metric.WithAttributes(labels...))
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

func (c *controller) updateDeletingPolicyStatus(ctx context.Context, policy v1alpha1.DeletingPolicy, time time.Time) error {
	err := controllerutils.UpdateStatus(ctx, &policy, c.kyvernoClient.PoliciesV1alpha1().DeletingPolicies(), func(p *v1alpha1.DeletingPolicy) error {
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
	logging.Info("updated deleting policy status", "name", policy.GetName(), "namespace", policy.GetNamespace(), "status", policy.Status)

	return nil
}
