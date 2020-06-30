package policycache

import (
	"reflect"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// Controller is responsible for synchronizing Policy Cache
type Controller struct {
	client        *client.Client
	kyvernoClient *kyvernoclient.Clientset
	syncHandler   func(pKey string) error
	enqueuePolicy func(policy *kyverno.ClusterPolicy)
	queue         workqueue.RateLimitingInterface
	pSynched      cache.InformerSynced
	pCache        Interface
	log           logr.Logger
}

// NewPolicyCacheController create a new PolicyController
func NewPolicyCacheController(
	pInformer kyvernoinformer.ClusterPolicyInformer,
	log logr.Logger) *Controller {

	pc := Controller{
		queue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "policy"),
		pCache: newPolicyCache(log),
		log:    log,
	}

	pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicy,
		UpdateFunc: pc.updatePolicy,
		DeleteFunc: pc.deletePolicy,
	})

	pc.pSynched = pInformer.Informer().HasSynced

	return &pc
}

func (c *Controller) addPolicy(obj interface{}) {
	p := obj.(*kyverno.ClusterPolicy)
	c.pCache.Add(p)
}

func (c *Controller) updatePolicy(old, cur interface{}) {
	pOld := old.(*kyverno.ClusterPolicy)
	pNew := cur.(*kyverno.ClusterPolicy)

	if reflect.DeepEqual(pOld.Spec, pNew.Spec) {
		return
	}

	c.pCache.Remove(pOld)
	c.pCache.Add(pNew)
}

func (c *Controller) deletePolicy(obj interface{}) {
	p := obj.(*kyverno.ClusterPolicy)
	c.pCache.Remove(p)
}

// Run begins watching and syncing.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	logger := c.log
	logger.Info("starting")
	defer logger.Info("shutting down")

	if !cache.WaitForCacheSync(stopCh, c.pSynched) {
		logger.Info("failed to sync informer cache")
		return
	}

	<-stopCh
}
