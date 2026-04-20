package deleting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/cel/policies/dpol/engine"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1beta1"
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
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
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
	enqueue controllerutils.EnqueueFuncT[v1beta1.DeletingPolicyLike]

	// config
	configuration config.Configuration
	cmResolver    engineapi.ConfigmapResolver
	eventGen      event.Interface
	metrics       pkgmetrics.DeletingMetrics
}

const (
	maxRetries      = 10
	Workers         = 3
	ControllerName  = "deleting-controller"
	minRequeueDelay = 1 * time.Second
)

func NewController(
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	polInformer kyvernov1beta1informers.DeletingPolicyInformer,
	ndpolInformer kyvernov1beta1informers.NamespacedDeletingPolicyInformer,
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
	keyFunc := controllerutils.MetaNamespaceKeyT[v1beta1.DeletingPolicyLike]
	baseEnqueueFunc := controllerutils.LogError(logger, controllerutils.Parse(keyFunc, controllerutils.Queue(queue)))
	enqueueFunc := func(logger logr.Logger, operation, kind string) controllerutils.EnqueueFuncT[v1beta1.DeletingPolicyLike] {
		logger = logger.WithValues("kind", kind, "operation", operation)
		return func(obj v1beta1.DeletingPolicyLike) error {
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
		// On update, enqueue only when generation (spec) changes; skip status-only updates
		func(oldObj, obj v1beta1.DeletingPolicyLike) {
			if oldObj.GetGeneration() != obj.GetGeneration() {
				_ = enqueueFunc(logger, "updated", "DeletingPolicy")(obj)
			}
		},
		controllerutils.DeleteFuncT(logger, enqueueFunc(logger, "deleted", "DeletingPolicy")),
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlersT(
		ndpolInformer.Informer(),
		controllerutils.AddFuncT(logger, enqueueFunc(logger, "added", "NamespacedDeletingPolicy")),
		// On update, enqueue only when generation (spec) changes; skip status-only updates
		func(oldObj, obj v1beta1.DeletingPolicyLike) {
			if oldObj.GetGeneration() != obj.GetGeneration() {
				_ = enqueueFunc(logger, "updated", "NamespacedDeletingPolicy")(obj)
			}
		},
		controllerutils.DeleteFuncT(logger, enqueueFunc(logger, "deleted", "NamespacedDeletingPolicy")),
	); err != nil {
		logger.Error(err, "failed to register namespaced event handlers")
	}
	return c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger.V(3), ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) deleting(ctx context.Context, logger logr.Logger, ePolicy engine.Policy) error {
	if c.client == nil {
		return nil
	}

	spec := ePolicy.Policy.GetDeletingPolicySpec()
	policy := ePolicy.Policy
	policyNamespace := policy.GetNamespace()

	debug := logger.V(4)
	var errs []error
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: spec.DeletionPropagationPolicy,
	}

	if spec.MatchConstraints == nil {
		return errors.New("matchConstraints is required")
	}

	selector, err := metav1.LabelSelectorAsSelector(spec.MatchConstraints.ObjectSelector)
	if err != nil {
		debug.Error(err, "failed to parse label selector")
		return err
	}

	restMapper, err := restmapper.GetRESTMapper(c.client)
	if err != nil {
		return err
	}

	gvrList := admissionpolicy.GetGVRs(spec.MatchConstraints, restMapper)

	for _, gvr := range gvrList {
		var client dynamic.ResourceInterface

		debug := debug.WithValues("gvr", gvr)
		debug.Info("processing...")
		if policyNamespace != "" && !isNamespaced(gvr, restMapper) {
			logger.WithValues("gvr", gvr).Error(errors.New("cluster-scoped kind cannot be used in namespaced policy"), "skipping cluster-scoped resource")
			continue
		}

		client = c.client.GetDynamicInterface().Resource(gvr)
		if policyNamespace != "" {
			client = client.(dynamic.NamespaceableResourceInterface).Namespace(policyNamespace)
		}

		list, err := client.List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			debug.Error(err, "failed to list resources")
			// record failure metric
			if c.metrics != nil {
				c.metrics.RecordDeletingFailure(ctx, gvr.Resource, "", policy, deleteOptions.PropagationPolicy)
			}
			// Check if this is a recoverable error (permission denied, resource not found, etc.)
			if dclient.IsRecoverableError(err) {
				logger.V(2).Info("skipping resource due to access restrictions", "resource", gvr.Resource, "error", err.Error())
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
			debug := debug.WithValues("name", name, "namespace", namespace)
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
				debug.Info("policy did not match match")
				errs = append(errs, err)
				continue
			}

			logger.WithValues("name", name, "namespace", namespace).Info("resource matched, it will be deleted...")
			if err := c.client.DeleteResource(ctx, resource.GetAPIVersion(), resource.GetKind(), namespace, name, false, deleteOptions); err != nil {
				if apierrors.IsNotFound(err) {
					debug.Info("resource not found")
					continue
				}
				if c.metrics != nil {
					c.metrics.RecordDeletingFailure(ctx, gvr.Resource, namespace, policy, deleteOptions.PropagationPolicy)
				}
				debug.Error(err, "failed to delete resource")
				errs = append(errs, err)
				e := event.NewDeletingPolicyEvent(ePolicy.Policy, resource, err)
				c.eventGen.Add(e)
			} else {
				if c.metrics != nil {
					c.metrics.RecordDeletedObject(ctx, gvr.Resource, namespace, policy, deleteOptions.PropagationPolicy)
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
	policy, err := c.provider.Get(ctx, namespace, name)
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
		if err := c.updateDeletingPolicyStatus(ctx, policy.Policy, time.Now()); err != nil {
			logger.Error(err, "failed to update the deleting policy status")
			return err
		}
		nextExecutionTime, err = policy.Policy.GetNextExecutionTime(time.Now())
		if err != nil {
			logger.Error(err, "failed to get the policy next execution time")
			return err
		}
	} else {
		nextExecutionTime = executionTime
	}
	// calculate the remaining time until deletion.
	// clamp to a sane minimum to avoid immediate hot-loops when nextExecutionTime is past/now
	delay := time.Until(*nextExecutionTime)
	if delay <= 0 {
		delay = minRequeueDelay
	}
	// add the item back to the queue after the delay
	c.queue.AddAfter(key, delay)
	return nil
}

func isNamespaced(gvr schema.GroupVersionResource, mapper apimeta.RESTMapper) bool {
	if mapper == nil {
		return false
	}
	kind, err := mapper.KindFor(gvr)
	if err != nil {
		return false
	}

	mapping, err := mapper.RESTMapping(kind.GroupKind(), kind.Version)
	if err != nil || mapping.Scope == nil {
		return false
	}

	return mapping.Scope.Name() == apimeta.RESTScopeNameNamespace
}

func (c *controller) updateDeletingPolicyStatus(ctx context.Context, policy v1beta1.DeletingPolicyLike, time time.Time) error {
	switch p := policy.(type) {
	case *v1beta1.DeletingPolicy:
		err := controllerutils.UpdateStatus(ctx, p, c.kyvernoClient.PoliciesV1beta1().DeletingPolicies(), func(p *v1beta1.DeletingPolicy) error {
			p.Status = v1beta1.DeletingPolicyStatus{
				LastExecutionTime: metav1.NewTime(time),
			}
			return nil
		}, func(current, expect *v1beta1.DeletingPolicy) bool {
			return datautils.DeepEqual(current.Status, expect.Status)
		})
		if err != nil {
			return err
		}
		logging.Info("updated deleting policy status", "name", p.GetName(), "namespace", p.GetNamespace(), "status", p.Status)
	case *v1beta1.NamespacedDeletingPolicy:
		err := controllerutils.UpdateStatus(ctx, p, c.kyvernoClient.PoliciesV1beta1().NamespacedDeletingPolicies(p.GetNamespace()), func(p *v1beta1.NamespacedDeletingPolicy) error {
			p.Status = v1beta1.DeletingPolicyStatus{
				LastExecutionTime: metav1.NewTime(time),
			}
			return nil
		}, func(current, expect *v1beta1.NamespacedDeletingPolicy) bool {
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
