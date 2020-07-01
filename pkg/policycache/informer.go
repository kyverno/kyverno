package policycache

import (
	"reflect"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	"k8s.io/client-go/tools/cache"
)

// Controller is responsible for synchronizing Policy Cache
type Controller struct {
	pSynched cache.InformerSynced
	Cache    Interface
	log      logr.Logger
}

// NewPolicyCacheController create a new PolicyController
func NewPolicyCacheController(
	pInformer kyvernoinformer.ClusterPolicyInformer,
	log logr.Logger) *Controller {

	pc := Controller{
		Cache: newPolicyCache(log),
		log:   log,
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
	c.Cache.Add(p)
}

func (c *Controller) updatePolicy(old, cur interface{}) {
	pOld := old.(*kyverno.ClusterPolicy)
	pNew := cur.(*kyverno.ClusterPolicy)

	if reflect.DeepEqual(pOld.Spec, pNew.Spec) {
		return
	}

	c.Cache.Remove(pOld)
	c.Cache.Add(pNew)
}

func (c *Controller) deletePolicy(obj interface{}) {
	p := obj.(*kyverno.ClusterPolicy)
	c.Cache.Remove(p)
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
