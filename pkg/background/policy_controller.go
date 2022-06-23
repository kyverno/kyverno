package background

import (
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"

	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

func (c *controller) addPolicy(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Error(err, "failed to compute policy key")
	} else {
		p := obj.(*kyvernov1.ClusterPolicy)
		logger.V(4).Info("adding policy", "key", key)
		urs, err := c.urLister.GetUpdateRequestsForClusterPolicy(key)
		if err != nil {
			logger.Error(err, "failed to list update requests for policy", "key", key)
			return
		}
		// re-evaluate the UR as the policy was added
		for _, ur := range urs {
			c.enqueueUpdateRequest(ur)
		}
		logger.V(4).Info("queuing policy for background processing", "key", key)
		c.enqueuePolicy(p)
	}
}

func (c *controller) updatePolicy(_, obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Error(err, "failed to compute policy key")
	} else {
		p := obj.(*kyvernov1.ClusterPolicy)
		logger.V(4).Info("updating policy", "key", key)
		urs, err := c.urLister.GetUpdateRequestsForClusterPolicy(key)
		if err != nil {
			logger.Error(err, "failed to list update requests for policy", "key", key)
			return
		}
		// re-evaluate the UR as the policy was updated
		for _, ur := range urs {
			c.enqueueUpdateRequest(ur)
		}
		logger.V(4).Info("queuing policy for background processing", "key", key)
		c.enqueuePolicy(p)
	}
}

func (c *controller) deletePolicy(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(kubeutils.GetObjectWithTombstone(obj))

	if err != nil {
		logger.Error(err, "failed to compute policy key")
	} else {
		p := obj.(*kyvernov1.ClusterPolicy)
		logger.V(4).Info("updating policy", "key", key)
		urs, err := c.urLister.GetUpdateRequestsForClusterPolicy(key)
		if err != nil {
			logger.Error(err, "failed to list update requests for policy", "key", key)
			return
		}
		// re-evaluate the UR as the policy was updated
		for _, ur := range urs {
			c.enqueueUpdateRequest(ur)
		}
		logger.V(4).Info("queuing policy for background processing", "key", key)
		c.enqueuePolicy(p)
	}
}

func (c *controller) syncPolicy(key string) error {
	policy, err := c.getPolicy(key)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	} else {
		err = c.updateURs(key, policy)
		if err != nil {
			logger.Error(err, "failed to updateUR on Policy update")
		}
		return nil
	}
}

func (c *controller) enqueuePolicy(policy kyvernov1.PolicyInterface) {
	key, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		logger.Error(err, "failed to enqueue policy")
		return
	}
	c.policyqueue.Add(key)
}

// Run begins watching and syncing.
func (c *controller) run(workers int, stopCh <-chan struct{}) {

	defer utilruntime.HandleCrash()
	defer c.policyqueue.ShutDown()

	logger.Info("starting")
	defer logger.Info("shutting down")

	for i := 0; i < workers; i++ {
		go wait.Until(c.policyworker, time.Second, stopCh)
	}
	<-stopCh
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *controller) policyworker() {
	for c.processNextPolicyWorkItem() {
	}
}

func (c *controller) processNextPolicyWorkItem() bool {
	key, quit := c.policyqueue.Get()
	if quit {
		return false
	}
	defer c.policyqueue.Done(key)
	err := c.syncPolicy(key.(string))
	c.handlePolicyErr(err, key)

	return true
}

func (c *controller) handlePolicyErr(err error, key interface{}) {
	if err == nil {
		c.policyqueue.Forget(key)
		return
	}

	if c.policyqueue.NumRequeues(key) < maxRetries {
		logger.Error(err, "failed to sync policy", "key", key)
		c.policyqueue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	logger.V(2).Info("dropping policy out of queue", "key", key)
	c.policyqueue.Forget(key)
}
