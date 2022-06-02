package background

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/dclient"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
	workers    = 3
)

type controller struct {
	// clients
	client        dclient.Interface
	kyvernoClient kyvernoclient.Interface

	// listers
	cpolLister kyvernov1listers.ClusterPolicyLister
	polLister  kyvernov1listers.PolicyLister
	urLister   kyvernov1beta1listers.UpdateRequestNamespaceLister

	// queue
	queue workqueue.RateLimitingInterface
}

func NewController(
	client dclient.Interface,
	kyvernoClient kyvernoclient.Interface,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	urInformer kyvernov1beta1informers.UpdateRequestInformer,
) *controller {
	c := controller{
		client:        client,
		kyvernoClient: kyvernoClient,
		cpolLister:    cpolInformer.Lister(),
		polLister:     polInformer.Lister(),
		urLister:      urInformer.Lister().UpdateRequests(config.KyvernoNamespace()),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "background-controller"),
	}
	controllerutils.AddDefaultEventHandlers(logger, cpolInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, polInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	controllerutils.Run(logger, c.queue, workers, maxRetries, c.reconcile, stopCh)
}

func (c *controller) reconcile(key, namespace, name string) error {
	logger := logger.WithValues("key", key, "namespace", namespace, "name", name)
	logger.Info("reconciling ...")
	_, err := c.loadPolicy(namespace, name)
	if err != nil {
		if errors.IsNotFound(err) {
			return c.cleanupPolicy(logger, key)
		}
		return err
	}
	return nil
}

func (c *controller) loadPolicy(namespace, name string) (kyvernov1.PolicyInterface, error) {
	if namespace == "" {
		return c.cpolLister.Get(name)
	} else {
		return c.polLister.Policies(namespace).Get(name)
	}
}

func (c *controller) cleanupPolicy(logger logr.Logger, key string) error {
	// generatePolicyWithClone := pkgCommon.ProcessDeletePolicyForCloneGenerateRule(p, c.client, c.kyvernoClient, c.urLister, p.GetName(), logger)
	urs, err := c.urLister.GetUpdateRequestsForClusterPolicy(key)
	if err != nil {
		return err
	}
	for _, ur := range urs {
		logger = logger.WithValues("ur-namespace", ur.GetNamespace(), "ur-name", ur.GetName())
		if err := c.cleanupUpdateRequest(logger, ur); err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) cleanupUpdateRequest(logger logr.Logger, ur *kyvernov1beta1.UpdateRequest) error {
	if !c.ownerResourceExists(logger, ur.Spec.Resource) {
		if err := c.deleteGeneratedResources(logger, ur); err != nil {
			return err
		}
		// - trigger-resource is deleted
		// - generated-resources are deleted
		// - > Now delete the UpdateRequest CR
		return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), ur.Name, metav1.DeleteOptions{})
	} else {
		logger.Info("owner resource exists, skipping cleanup")
	}
	return nil
}

func (c *controller) ownerResourceExists(logger logr.Logger, resource kyvernov1.ResourceSpec) bool {
	if _, err := c.client.GetResource(resource.APIVersion, resource.Kind, resource.Namespace, resource.Name); err != nil {
		if errors.IsNotFound(err) {
			return false
		}
		logger.Error(err, "failed to get resource", "genKind", resource.Kind, "genNamespace", resource.Namespace, "genName", resource.Name)
	}
	return true
}

func (c *controller) deleteGeneratedResources(logger logr.Logger, ur *kyvernov1beta1.UpdateRequest) error {
	for _, genResource := range ur.Status.GeneratedResources {
		err := c.client.DeleteResource("", genResource.Kind, genResource.Namespace, genResource.Name, false)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		logger.Info("generated resource deleted", "genKind", genResource.Kind, "genNamespace", genResource.Namespace, "genName", genResource.Name)
	}
	return nil
}
