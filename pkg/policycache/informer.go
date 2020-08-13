package policycache

import (
	"reflect"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	"k8s.io/client-go/tools/cache"
)

// Controller is responsible for synchronizing Policy Cache,
// it embeds a policy informer to handle policy events.
// The cache is synced when a policy is add/update/delete.
// This cache is only used in the admission webhook to fast retrieve
// policies based on types (Mutate/ValidateEnforce/Generate).
type Controller struct {
	pSynched cache.InformerSynced
	Cache    Interface
	log      logr.Logger
}

// NewPolicyCacheController create a new PolicyController
func NewPolicyCacheController(
	pInformer kyvernoinformer.ClusterPolicyInformer,
	nspInformer kyvernoinformer.NamespacePolicyInformer,
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

	nspInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicy,
		UpdateFunc: pc.updatePolicy,
		DeleteFunc: pc.deletePolicy,
	})

	pc.pSynched = pInformer.Informer().HasSynced

	return &pc
}

// convertNamespacedPolicyToClusterPolicy - convert NamespacePolicy to ClusterPolicy
func convertNamespacedPolicyToClusterPolicy(nsPolicies *kyverno.NamespacePolicy) *kyverno.ClusterPolicy {
	cpol := kyverno.ClusterPolicy(*nsPolicies)
	return &cpol
}

func (c *Controller) addPolicy(obj interface{}) {
	p, ok := obj.(*kyverno.ClusterPolicy)
	if ok {
		c.Cache.Add(p)
	} else {
		p := obj.(*kyverno.NamespacePolicy)
		c.Cache.Add(convertNamespacedPolicyToClusterPolicy(p))
	}
}

func (c *Controller) updatePolicy(old, cur interface{}) {
	pOld, ok := old.(*kyverno.ClusterPolicy)
	pNew, ok := cur.(*kyverno.ClusterPolicy)

	if reflect.DeepEqual(pOld.Spec, pNew.Spec) {
		return
	}
	if ok {
		c.Cache.Remove(pOld)
		c.Cache.Add(pNew)
	} else {
		pOld := old.(*kyverno.NamespacePolicy)
		pNew := cur.(*kyverno.NamespacePolicy)
		if reflect.DeepEqual(pOld.Spec, pNew.Spec) {
			return
		}
		c.Cache.Remove(convertNamespacedPolicyToClusterPolicy(pOld))
		c.Cache.Add(convertNamespacedPolicyToClusterPolicy(pNew))
	}
}

func (c *Controller) deletePolicy(obj interface{}) {
	p, ok := obj.(*kyverno.ClusterPolicy)
	if ok {
		c.Cache.Remove(p)
	} else {
		p := obj.(*kyverno.NamespacePolicy)
		c.Cache.Remove(convertNamespacedPolicyToClusterPolicy(p))
	}
}

// Run waits until policy informer to be synced
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
