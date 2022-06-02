package background

import (
	"strconv"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/dclient"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
	workers    = 3
)

type controller struct {
	// clients
	client dclient.Interface

	// listers
	cpolLister kyvernov1listers.ClusterPolicyLister
	polLister  kyvernov1listers.PolicyLister
	urLister   kyvernov1beta1listers.UpdateRequestNamespaceLister

	// queue
	queue workqueue.RateLimitingInterface
}

func NewController(
	client dclient.Interface,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	urInformer kyvernov1beta1informers.UpdateRequestInformer,
) *controller {
	c := controller{
		client:     client,
		cpolLister: cpolInformer.Lister(),
		polLister:  polInformer.Lister(),
		urLister:   urInformer.Lister().UpdateRequests(config.KyvernoNamespace()),
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "background-controller"),
	}
	controllerutils.AddDefaultEventHandlers(logger, cpolInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, polInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	controllerutils.Run(logger, c.queue, workers, maxRetries, c.reconcile, stopCh)
}

func (c *controller) reconcile(key, namespace, name string) error {
	logger.Info("reconciling ...", "key", key, "namespace", namespace, "name", name)
	_, err := c.loadPolicy(namespace, name)
	if err != nil {
		if errors.IsNotFound(err) {
			return c.cleanupPolicy(key)
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

func (c *controller) cleanupPolicy(key string) error {
	// generatePolicyWithClone := pkgCommon.ProcessDeletePolicyForCloneGenerateRule(p, c.client, c.kyvernoClient, c.urLister, p.GetName(), logger)
	urs, err := c.urLister.GetUpdateRequestsForClusterPolicy(key)
	if err != nil {
		return err
	}
	for _, ur := range urs {
		if err := c.cleanupUpdateRequest(ur); err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) cleanupUpdateRequest(ur *kyvernov1beta1.UpdateRequest) error {
	resource := ur.Spec.Resource
	if !c.ownerResourceExists(logger, resource.Kind, resource.Namespace, resource.Name) {
		deleteUR := false
		// check retry count in annotaion
		urAnnotations := ur.Annotations
		if val, ok := urAnnotations[kyvernov1beta1.URGenerateRetryCountAnnotation]; ok {
			retryCount, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return err
			}
			if retryCount >= 5 {
				deleteUR = true
			}
		}
		if deleteUR {
			// if err := deleteGeneratedResources(logger, c.client, ur); err != nil {
			// 	return err
			// }
			// // - trigger-resource is deleted
			// // - generated-resources are deleted
			// // - > Now delete the UpdateRequest CR
			// return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), ur.Name, metav1.DeleteOptions{})
		}
	}
	return nil
}

func (c *controller) ownerResourceExists(logger logr.Logger, kind, namespace, name string) bool {
	_, err := c.client.GetResource("", kind, namespace, name)
	// trigger resources has been deleted
	if errors.IsNotFound(err) {
		return false
	}
	if err != nil {
		logger.Error(err, "failed to get resource", "genKind", kind, "genNamespace", namespace, "genName", name)
	}
	// if there was an error while querying the resources we don't delete the generated resources
	// but expect the deletion in next reconciliation loop
	return true
}
