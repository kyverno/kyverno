package cleanup

import (
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	// typed client for kyverno CRDs
	kyvernoClient *kyvernoclient.Clientset
	// handler for GR CR
	syncHandler func(grKey string) error
	// handler to enqueue GR
	enqueueGR func(gr *kyverno.GenerateRequest)

	// control is used to delete the GR
	control ControlInterface
	// gr that need to be synced
	queue workqueue.RateLimitingInterface
	// pLister can list/get cluster policy from the shared informer's store
	pLister kyvernolister.ClusterPolicyLister
	// grLister can list/get generate request from the shared informer's store
	grLister kyvernolister.GenerateRequestNamespaceLister
	// pSynced returns true if the cluster policy has been synced at least once
	pSynced cache.InformerSynced
	// grSynced returns true if the generate request store has been synced at least once
	grSynced cache.InformerSynced
}

func NewController(
	kyvernoclient *kyvernoclient.Clientset,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	grInformer kyvernoinformer.GenerateRequestInformer,
) *Controller {
	c := Controller{
		kyvernoClient: kyvernoclient,
		//TODO: do the math for worst case back off and make sure cleanup runs after that
		// as we dont want a deleted GR to be re-queue
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(1, 30), "generate-request-cleanup"),
	}
	c.control = Control{client: kyvernoclient}

	pInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		DeleteFunc: c.deletePolicy, // we only cleanup if the policy is delete
	}, 30)

	grInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addGR,
		UpdateFunc: c.updateGR,
		DeleteFunc: c.deleteGR,
	}, 30)

	return &c
}

func (c *Controller) deletePolicy(obj interface{}) {

}

func (c *Controller) addGR(obj interface{}) {

}

func (c *Controller) updateGR(old, curr interface{}) {

}

func (c *Controller) deleteGR(obj interface{}) {

}
