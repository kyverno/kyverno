package cleanup

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	kyvernov2alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2alpha1"
	kyvernov2alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	batchv1informers "k8s.io/client-go/informers/batch/v1"
	"k8s.io/client-go/kubernetes"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// CleanupServicePath is the path for triggering cleanup
	CleanupServicePath = "/cleanup"
)

type controller struct {
	// clients
	client kubernetes.Interface

	// listers
	cpolLister kyvernov2alpha1listers.ClusterCleanupPolicyLister
	polLister  kyvernov2alpha1listers.CleanupPolicyLister
	cjLister   batchv1listers.CronJobLister

	// queue
	queue   workqueue.RateLimitingInterface
	enqueue controllerutils.EnqueueFuncT[kyvernov2alpha1.CleanupPolicyInterface]

	// config
	cleanupService string
}

const (
	maxRetries     = 10
	Workers        = 3
	ControllerName = "cleanup-controller"
)

func NewController(
	client kubernetes.Interface,
	cpolInformer kyvernov2alpha1informers.ClusterCleanupPolicyInformer,
	polInformer kyvernov2alpha1informers.CleanupPolicyInformer,
	cjInformer batchv1informers.CronJobInformer,
	cleanupService string,
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
		client:         client,
		cpolLister:     cpolInformer.Lister(),
		polLister:      polInformer.Lister(),
		cjLister:       cjInformer.Lister(),
		queue:          queue,
		cleanupService: cleanupService,
		enqueue:        baseEnqueueFunc,
	}
	controllerutils.AddEventHandlersT(
		cpolInformer.Informer(),
		controllerutils.AddFuncT(logger, enqueueFunc(logger, "added", "ClusterCleanupPolicy")),
		controllerutils.UpdateFuncT(logger, enqueueFunc(logger, "updated", "ClusterCleanupPolicy")),
		controllerutils.DeleteFuncT(logger, enqueueFunc(logger, "deleted", "ClusterCleanupPolicy")),
	)
	controllerutils.AddEventHandlersT(
		polInformer.Informer(),
		controllerutils.AddFuncT(logger, enqueueFunc(logger, "added", "CleanupPolicy")),
		controllerutils.UpdateFuncT(logger, enqueueFunc(logger, "updated", "CleanupPolicy")),
		controllerutils.DeleteFuncT(logger, enqueueFunc(logger, "deleted", "CleanupPolicy")),
	)
	controllerutils.AddEventHandlersT(
		cjInformer.Informer(),
		func(n *batchv1.CronJob) { c.enqueueCronJob(n) },
		func(o *batchv1.CronJob, n *batchv1.CronJob) { c.enqueueCronJob(o) },
		func(n *batchv1.CronJob) { c.enqueueCronJob(n) },
	)
	return c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger.V(3), ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) enqueueCronJob(n *batchv1.CronJob) {
	if len(n.OwnerReferences) == 1 {
		if n.OwnerReferences[0].Kind == "ClusterCleanupPolicy" {
			cpol := &kyvernov2alpha1.ClusterCleanupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: n.OwnerReferences[0].Name,
				},
			}
			err := c.enqueue(cpol)
			if err != nil {
				logger.Error(err, "failed to enqueue ClusterCleanupPolicy object", cpol)
			}
		} else if n.OwnerReferences[0].Kind == "CleanupPolicy" {
			pol := &kyvernov2alpha1.CleanupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      n.OwnerReferences[0].Name,
					Namespace: n.Namespace,
				},
			}
			err := c.enqueue(pol)
			if err != nil {
				logger.Error(err, "failed to enqueue CleanupPolicy object", pol)
			}
		}
	}
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

func (c *controller) getCronjob(namespace, name string) (*batchv1.CronJob, error) {
	cj, err := c.cjLister.CronJobs(namespace).Get(name)
	if err != nil {
		return nil, err
	}
	return cj, nil
}

func (c *controller) buildCronJob(cronJob *batchv1.CronJob, pol kyvernov2alpha1.CleanupPolicyInterface) error {
	// TODO: find a better way to do that, it looks like resources returned by WATCH don't have the GVK
	apiVersion := "kyverno.io/v2alpha1"
	kind := "CleanupPolicy"
	if pol.GetNamespace() == "" {
		kind = "ClusterCleanupPolicy"
	}
	policyName, err := cache.MetaNamespaceKeyFunc(pol)
	if err != nil {
		return err
	}
	// set owner reference
	cronJob.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: apiVersion,
			Kind:       kind,
			Name:       pol.GetName(),
			UID:        pol.GetUID(),
		},
	}
	var successfulJobsHistoryLimit int32 = 0
	var failedJobsHistoryLimit int32 = 1
	var boolFalse bool = false
	var boolTrue bool = true
	var int1000 int64 = 1000
	// set spec
	cronJob.Spec = batchv1.CronJobSpec{
		Schedule:                   pol.GetSpec().Schedule,
		SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
		FailedJobsHistoryLimit:     &failedJobsHistoryLimit,
		ConcurrencyPolicy:          batchv1.ForbidConcurrent,
		JobTemplate: batchv1.JobTemplateSpec{
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyOnFailure,
						Containers: []corev1.Container{
							{
								Name:  "cleanup",
								Image: "curlimages/curl:7.86.0",
								Args: []string{
									"-k",
									// TODO: ca
									// "--cacert",
									// "/tmp/ca.crt",
									fmt.Sprintf("%s%s?policy=%s", c.cleanupService, CleanupServicePath, policyName),
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: &boolFalse,
									RunAsNonRoot:             &boolTrue,
									RunAsUser:                &int1000,
									SeccompProfile: &corev1.SeccompProfile{
										Type: corev1.SeccompProfileTypeRuntimeDefault,
									},
									Capabilities: &corev1.Capabilities{
										Drop: []corev1.Capability{"ALL"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	// set labels
	controllerutils.SetManagedByKyvernoLabel(cronJob)
	controllerutils.SetManagedByKyvernoLabel(&cronJob.Spec.JobTemplate)
	controllerutils.SetManagedByKyvernoLabel(&cronJob.Spec.JobTemplate.Spec.Template)
	return nil
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
	cronjobNs := namespace
	if namespace == "" {
		cronjobNs = config.KyvernoNamespace()
	}
	observed, err := c.getCronjob(cronjobNs, string(policy.GetUID()))
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		observed = &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      string(policy.GetUID()),
				Namespace: cronjobNs,
			},
		}
	}
	if observed.ResourceVersion == "" {
		err := c.buildCronJob(observed, policy)
		if err != nil {
			return err
		}
		_, err = c.client.BatchV1().CronJobs(cronjobNs).Create(ctx, observed, metav1.CreateOptions{})
		return err
	} else {
		_, err = controllerutils.Update(ctx, observed, c.client.BatchV1().CronJobs(cronjobNs), func(observed *batchv1.CronJob) error {
			return c.buildCronJob(observed, policy)
		})
		return err
	}
}
