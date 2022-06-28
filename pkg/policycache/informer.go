package policycache

import (
	"os"
	"reflect"
	"sync/atomic"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// Controller is responsible for synchronizing Policy Cache,
// it embeds a policy informer to handle policy events.
// The cache is synced when a policy is add/update/delete.
// This cache is only used in the admission webhook to fast retrieve
// policies based on types (Mutate/ValidateEnforce/Generate/imageVerify).
type Controller struct {
	Cache      Interface
	cpolLister kyvernolister.ClusterPolicyLister
	polLister  kyvernolister.PolicyLister
	pCounter   int64
}

// NewPolicyCacheController create a new PolicyController
func NewPolicyCacheController(pInformer kyvernoinformer.ClusterPolicyInformer, nspInformer kyvernoinformer.PolicyInformer) *Controller {
	pc := Controller{
		Cache: newPolicyCache(pInformer.Lister(), nspInformer.Lister(), pInformer.Informer().HasSynced, nspInformer.Informer().HasSynced),
	}

	// ClusterPolicy Informer
	pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicy,
		UpdateFunc: pc.updatePolicy,
		DeleteFunc: pc.deletePolicy,
	})

	// Policy Informer
	nspInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addNsPolicy,
		UpdateFunc: pc.updateNsPolicy,
		DeleteFunc: pc.deleteNsPolicy,
	})

	pc.cpolLister = pInformer.Lister()
	pc.polLister = nspInformer.Lister()
	pc.pCounter = -1

	return &pc
}

func (c *Controller) addPolicy(obj interface{}) {
	p := obj.(*kyverno.ClusterPolicy)
	c.Cache.add(p)
}

func (c *Controller) updatePolicy(old, cur interface{}) {
	pOld := old.(*kyverno.ClusterPolicy)
	pNew := cur.(*kyverno.ClusterPolicy)
	if reflect.DeepEqual(pOld.Spec, pNew.Spec) {
		return
	}
	c.Cache.update(pOld, pNew)
}

func (c *Controller) deletePolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyverno.ClusterPolicy)
	if ok {
		c.Cache.remove(p)
	} else {
		logger.Info("Failed to get deleted object, the deleted policy cannot be removed from the cache", "obj", obj)
	}
}

// addNsPolicy - Add Policy to cache
func (c *Controller) addNsPolicy(obj interface{}) {
	p := obj.(*kyverno.Policy)
	c.Cache.add(p)
}

// updateNsPolicy - Update Policy of cache
func (c *Controller) updateNsPolicy(old, cur interface{}) {
	npOld := old.(*kyverno.Policy)
	npNew := cur.(*kyverno.Policy)
	if reflect.DeepEqual(npOld.Spec, npNew.Spec) {
		return
	}
	c.Cache.update(npOld, npNew)
}

// deleteNsPolicy - Delete Policy from cache
func (c *Controller) deleteNsPolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyverno.Policy)
	if ok {
		c.Cache.remove(p)
	} else {
		logger.Info("Failed to get deleted object, the deleted cluster policy cannot be removed from the cache", "obj", obj)
	}
}

// CheckPolicySync wait until the internal policy cache is fully loaded
func (c *Controller) CheckPolicySync(stopCh <-chan struct{}) {
	logger.Info("starting")

	if !cache.WaitForNamedCacheSync("config-controller", stopCh, c.Cache.informerHasSynced()...) {
		return
	}

	policies := []kyverno.PolicyInterface{}
	polList, err := c.polLister.Policies(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		logger.Error(err, "failed to list Policy")
		os.Exit(1)
	}
	for _, p := range polList {
		policies = append(policies, p)
	}
	cpolList, err := c.cpolLister.List(labels.Everything())
	if err != nil {
		logger.Error(err, "failed to list Cluster Policy")
		os.Exit(1)
	}
	for _, p := range cpolList {
		policies = append(policies, p)
	}

	atomic.StoreInt64(&c.pCounter, int64(len(policies)))
	for _, policy := range policies {
		c.Cache.add(policy)
		atomic.AddInt64(&c.pCounter, ^int64(0))
	}

	if !c.hasPolicySynced() {
		logger.Error(nil, "Failed to sync policy with cache")
		os.Exit(1)
	}
}

// hasPolicySynced check for policy counter zero
func (c *Controller) hasPolicySynced() bool {
	return atomic.LoadInt64(&c.pCounter) == 0
}
