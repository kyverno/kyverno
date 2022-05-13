package policycache

import (
	"os"
	"reflect"
	"sync/atomic"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/policy"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// Cache get method use for to get policy names and mostly use to test cache testcases
type Cache interface {
	// GetPolicies returns all policies that apply to a namespace, including cluster-wide policies
	// If the namespace is empty, only cluster-wide policies are returned
	GetPolicies(PolicyType, string, string) []kyverno.PolicyInterface
	// CheckPolicySync wait until the internal policy cache is fully loaded
	CheckPolicySync(<-chan struct{})
}

// controller is responsible for synchronizing Policy Cache,
// it embeds a policy informer to handle policy events.
// The cache is synced when a policy is add/update/delete.
// This cache is only used in the admission webhook to fast retrieve
// policies based on types (Mutate/ValidateEnforce/Generate/imageVerify).
type controller struct {
	store
	cpolLister kyvernolister.ClusterPolicyLister
	polLister  kyvernolister.PolicyLister
	pCounter   int64
}

// NewCache create a new Cache
func NewCache(pInformer kyvernoinformer.ClusterPolicyInformer, nspInformer kyvernoinformer.PolicyInformer) Cache {
	pc := controller{
		store:      newPolicyCache(),
		cpolLister: pInformer.Lister(),
		polLister:  nspInformer.Lister(),
		pCounter:   -1,
	}
	pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicy,
		UpdateFunc: pc.updatePolicy,
		DeleteFunc: pc.deletePolicy,
	})
	nspInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addNsPolicy,
		UpdateFunc: pc.updateNsPolicy,
		DeleteFunc: pc.deleteNsPolicy,
	})
	return &pc
}

func (c *controller) GetPolicies(pkey PolicyType, kind, nspace string) []kyverno.PolicyInterface {
	var names []string
	names = append(names, c.store.get(pkey, kind, "")...)
	names = append(names, c.store.get(pkey, "*", "")...)
	if nspace != "" {
		names = append(names, c.store.get(pkey, kind, nspace)...)
		names = append(names, c.store.get(pkey, "*", nspace)...)
	}
	var policies []kyverno.PolicyInterface
	for _, name := range names {
		ns, key, isNamespacedPolicy := policy.ParseNamespacedPolicy(name)
		if !isNamespacedPolicy {
			if p, err := c.cpolLister.Get(key); err == nil {
				policies = append(policies, p)
			}
		} else {
			if ns == nspace {
				if p, err := c.polLister.Policies(ns).Get(key); err == nil {
					policies = append(policies, p)
				}
			}
		}
	}
	return policies
}

func (c *controller) CheckPolicySync(stopCh <-chan struct{}) {
	logger.Info("starting")
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
		c.store.add(policy)
		atomic.AddInt64(&c.pCounter, ^int64(0))
	}
	if !c.hasPolicySynced() {
		logger.Error(nil, "Failed to sync policy with cache")
		os.Exit(1)
	}
}

func (c *controller) addPolicy(obj interface{}) {
	p := obj.(*kyverno.ClusterPolicy)
	c.store.add(p)
}

func (c *controller) updatePolicy(old, cur interface{}) {
	pOld := old.(*kyverno.ClusterPolicy)
	pNew := cur.(*kyverno.ClusterPolicy)
	if reflect.DeepEqual(pOld.Spec, pNew.Spec) {
		return
	}
	c.store.update(pOld, pNew)
}

func (c *controller) deletePolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyverno.ClusterPolicy)
	if ok {
		c.store.remove(p)
	} else {
		logger.Info("Failed to get deleted object, the deleted policy cannot be removed from the cache", "obj", obj)
	}
}

func (c *controller) addNsPolicy(obj interface{}) {
	p := obj.(*kyverno.Policy)
	c.store.add(p)
}

func (c *controller) updateNsPolicy(old, cur interface{}) {
	npOld := old.(*kyverno.Policy)
	npNew := cur.(*kyverno.Policy)
	if reflect.DeepEqual(npOld.Spec, npNew.Spec) {
		return
	}
	c.store.update(npOld, npNew)
}

func (c *controller) deleteNsPolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyverno.Policy)
	if ok {
		c.store.remove(p)
	} else {
		logger.Info("Failed to get deleted object, the deleted cluster policy cannot be removed from the cache", "obj", obj)
	}
}

func (c *controller) hasPolicySynced() bool {
	return atomic.LoadInt64(&c.pCounter) == 0
}
