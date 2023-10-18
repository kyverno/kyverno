package cleanup

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov2beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2beta1"
	kyvernov2beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"github.com/kyverno/kyverno/pkg/utils/match"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
)

type controller struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface

	// listers
	cpolLister kyvernov2beta1listers.ClusterCleanupPolicyLister
	polLister  kyvernov2beta1listers.CleanupPolicyLister
	nsLister   corev1listers.NamespaceLister

	// queue
	queue   workqueue.RateLimitingInterface
	enqueue controllerutils.EnqueueFuncT[kyvernov2alpha1.CleanupPolicyInterface]

	// config
	configuration config.Configuration
	cmResolver    engineapi.ConfigmapResolver
	eventGen      event.Interface
	jp            jmespath.Interface
	metrics       cleanupMetrics
}

type cleanupMetrics struct {
	deletedObjectsTotal  metric.Int64Counter
	cleanupFailuresTotal metric.Int64Counter
}

const (
	maxRetries     = 10
	Workers        = 3
	ControllerName = "cleanup-controller"
)

func NewController(
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	cpolInformer kyvernov2beta1informers.ClusterCleanupPolicyInformer,
	polInformer kyvernov2beta1informers.CleanupPolicyInformer,
	nsLister corev1listers.NamespaceLister,
	configuration config.Configuration,
	cmResolver engineapi.ConfigmapResolver,
	jp jmespath.Interface,
	eventGen event.Interface,
) controllers.Controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName)
	keyFunc := controllerutils.MetaNamespaceKeyT[kyvernov2alpha1.CleanupPolicyInterface]
	baseEnqueueFunc := controllerutils.LogError(logger, controllerutils.Parse(keyFunc, controllerutils.Queue(queue)))
	enqueueFunc := func(logger logr.Logger, operation, kind string) controllerutils.EnqueueFuncT[kyvernov2alpha1.CleanupPolicyInterface] {
		logger = logger.WithValues("kind", kind, "operation", operation)
		return func(obj kyvernov2alpha1.CleanupPolicyInterface) error {
			logger = logger.WithValues("name", obj.GetName())
			if obj.GetNamespace() != "" {
				logger = logger.WithValues("namespace", obj.GetNamespace())
			}
			logger.Info(operation)
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
		cpolLister:    cpolInformer.Lister(),
		polLister:     polInformer.Lister(),
		nsLister:      nsLister,
		queue:         queue,
		enqueue:       baseEnqueueFunc,
		configuration: configuration,
		cmResolver:    cmResolver,
		eventGen:      eventGen,
		metrics:       newCleanupMetrics(logger),
		jp:            jp,
	}
	if _, err := controllerutils.AddEventHandlersT(
		cpolInformer.Informer(),
		controllerutils.AddFuncT(logger, enqueueFunc(logger, "added", "ClusterCleanupPolicy")),
		controllerutils.UpdateFuncT(logger, enqueueFunc(logger, "updated", "ClusterCleanupPolicy")),
		controllerutils.DeleteFuncT(logger, enqueueFunc(logger, "deleted", "ClusterCleanupPolicy")),
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlersT(
		polInformer.Informer(),
		controllerutils.AddFuncT(logger, enqueueFunc(logger, "added", "CleanupPolicy")),
		controllerutils.UpdateFuncT(logger, enqueueFunc(logger, "updated", "CleanupPolicy")),
		controllerutils.DeleteFuncT(logger, enqueueFunc(logger, "deleted", "CleanupPolicy")),
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	return c
}

func newCleanupMetrics(logger logr.Logger) cleanupMetrics {
	meter := otel.GetMeterProvider().Meter(metrics.MeterName)
	deletedObjectsTotal, err := meter.Int64Counter(
		"kyverno_cleanup_controller_deletedobjects",
		metric.WithDescription("can be used to track number of deleted objects."),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, cleanup_controller_deletedobjects_total")
	}
	cleanupFailuresTotal, err := meter.Int64Counter(
		"kyverno_cleanup_controller_errors",
		metric.WithDescription("can be used to track number of cleanup failures."),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, cleanup_controller_errors_total")
	}
	return cleanupMetrics{
		deletedObjectsTotal:  deletedObjectsTotal,
		cleanupFailuresTotal: cleanupFailuresTotal,
	}
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger.V(3), ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) getPolicy(namespace, name string) (kyvernov2alpha1.CleanupPolicyInterface, error) {
	if namespace == "" {
		cpolicy, err := c.cpolLister.Get(name)
		if err != nil {
			return nil, err
		}
		return cpolicy, nil
	} else {
		policy, err := c.polLister.CleanupPolicies(namespace).Get(name)
		if err != nil {
			return nil, err
		}
		return policy, nil
	}
}

func (c *controller) cleanup(ctx context.Context, logger logr.Logger, policy kyvernov2alpha1.CleanupPolicyInterface) error {
	spec := policy.GetSpec()
	kinds := sets.New(spec.MatchResources.GetKinds()...)
	debug := logger.V(4)
	var errs []error

	enginectx := enginecontext.NewContext(c.jp)
	ctxFactory := factories.DefaultContextLoaderFactory(c.cmResolver)

	loader := ctxFactory(nil, kyvernov1.Rule{})
	if err := loader.Load(
		ctx,
		c.jp,
		c.client,
		nil,
		spec.Context,
		enginectx,
	); err != nil {
		return err
	}

	for kind := range kinds {
		commonLabels := []attribute.KeyValue{
			attribute.String("policy_type", policy.GetKind()),
			attribute.String("policy_namespace", policy.GetNamespace()),
			attribute.String("policy_name", policy.GetName()),
			attribute.String("resource_kind", kind),
		}
		debug := debug.WithValues("kind", kind)
		debug.Info("processing...")
		list, err := c.client.ListResource(ctx, "", kind, policy.GetNamespace(), nil)
		if err != nil {
			debug.Error(err, "failed to list resources")
			errs = append(errs, err)
			if c.metrics.cleanupFailuresTotal != nil {
				c.metrics.cleanupFailuresTotal.Add(ctx, 1, metric.WithAttributes(commonLabels...))
			}
		} else {
			for i := range list.Items {
				resource := list.Items[i]
				namespace := resource.GetNamespace()
				name := resource.GetName()
				debug := debug.WithValues("name", name, "namespace", namespace)
				if !controllerutils.IsManagedByKyverno(&resource) {
					var nsLabels map[string]string
					if namespace != "" {
						ns, err := c.nsLister.Get(namespace)
						if err != nil {
							debug.Error(err, "failed to get namespace labels")
							errs = append(errs, err)
						}
						nsLabels = ns.GetLabels()
					}
					// match namespaces
					if err := match.CheckNamespace(policy.GetNamespace(), resource); err != nil {
						debug.Info("resource namespace didn't match policy namespace", "result", err)
					}
					// match resource with match/exclude clause
					matched := match.CheckMatchesResources(
						resource,
						spec.MatchResources,
						nsLabels,
						// TODO(eddycharly): we don't have user info here, we should check that
						// we don't have user conditions in the policy rule
						kyvernov1beta1.RequestInfo{},
						resource.GroupVersionKind(),
						"",
					)
					if matched != nil {
						debug.Info("resource/match didn't match", "result", matched)
						continue
					}
					if spec.ExcludeResources != nil {
						excluded := match.CheckMatchesResources(
							resource,
							*spec.ExcludeResources,
							nsLabels,
							// TODO(eddycharly): we don't have user info here, we should check that
							// we don't have user conditions in the policy rule
							kyvernov1beta1.RequestInfo{},
							resource.GroupVersionKind(),
							"",
						)
						if excluded == nil {
							debug.Info("resource/exclude matched")
							continue
						} else {
							debug.Info("resource/exclude didn't match", "result", excluded)
						}
					}
					// check conditions
					if spec.Conditions != nil {
						enginectx.Reset()
						if err := enginectx.SetTargetResource(resource.Object); err != nil {
							debug.Error(err, "failed to add resource in context")
							errs = append(errs, err)
							continue
						}
						if err := enginectx.AddNamespace(resource.GetNamespace()); err != nil {
							debug.Error(err, "failed to add namespace in context")
							errs = append(errs, err)
							continue
						}
						if err := enginectx.AddImageInfos(&resource, c.configuration); err != nil {
							debug.Error(err, "failed to add image infos in context")
							errs = append(errs, err)
							continue
						}
						passed, err := checkAnyAllConditions(logger, enginectx, *spec.Conditions)
						if err != nil {
							debug.Error(err, "failed to check condition")
							errs = append(errs, err)
							continue
						}
						if !passed {
							debug.Info("conditions did not pass")
							continue
						}
					}
					var labels []attribute.KeyValue
					labels = append(labels, commonLabels...)
					labels = append(labels, attribute.String("resource_namespace", namespace))
					logger.WithValues("name", name, "namespace", namespace).Info("resource matched, it will be deleted...")
					if err := c.client.DeleteResource(ctx, resource.GetAPIVersion(), resource.GetKind(), namespace, name, false); err != nil {
						if c.metrics.cleanupFailuresTotal != nil {
							c.metrics.cleanupFailuresTotal.Add(ctx, 1, metric.WithAttributes(labels...))
						}
						debug.Error(err, "failed to delete resource")
						errs = append(errs, err)
						e := event.NewCleanupPolicyEvent(policy, resource, err)
						c.eventGen.Add(e)
					} else {
						if c.metrics.deletedObjectsTotal != nil {
							c.metrics.deletedObjectsTotal.Add(ctx, 1, metric.WithAttributes(labels...))
						}
						debug.Info("deleted")
						e := event.NewCleanupPolicyEvent(policy, resource, nil)
						c.eventGen.Add(e)
					}
				}
			}
		}
	}
	return multierr.Combine(errs...)
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	policy, err := c.getPolicy(namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		logger.Error(err, "unable to get the policy from policy informer")
		return err
	}

	var nextExecutionTime *time.Time
	executionTime, err := policy.GetExecutionTime()
	if err != nil {
		logger.Error(err, "failed to get the policy execution time")
		return err
	}
	// In case it is the time to do the cleanup process
	if time.Now().After(*executionTime) {
		err := c.cleanup(ctx, logger, policy)
		if err != nil {
			return err
		}
		if err := c.updateCleanupPolicyStatus(ctx, policy, namespace, *executionTime); err != nil {
			logger.Error(err, "failed to update the cleanup policy status")
			return err
		}
		nextExecutionTime, err = policy.GetNextExecutionTime(*executionTime)
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

func (c *controller) updateCleanupPolicyStatus(ctx context.Context, policy kyvernov2alpha1.CleanupPolicyInterface, namespace string, time time.Time) error {
	switch obj := policy.(type) {
	case *kyvernov2beta1.ClusterCleanupPolicy:
		latest := obj.DeepCopy()
		latest.Status.LastExecutionTime = metav1.NewTime(time)

		new, err := c.kyvernoClient.KyvernoV2beta1().ClusterCleanupPolicies().UpdateStatus(ctx, latest, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		logging.V(3).Info("updated cluster cleanup policy status", "name", policy.GetName(), "status", new.Status)
	case *kyvernov2beta1.CleanupPolicy:
		latest := obj.DeepCopy()
		latest.Status.LastExecutionTime = metav1.NewTime(time)

		new, err := c.kyvernoClient.KyvernoV2beta1().CleanupPolicies(namespace).UpdateStatus(ctx, latest, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		logging.V(3).Info("updated cleanup policy status", "name", policy.GetName(), "namespace", policy.GetNamespace(), "status", new.Status)
	}
	return nil
}
