package background

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"

	"k8s.io/apimachinery/pkg/api/errors"
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
