package policycache

import (
	"os"
	"reflect"
	"sync/atomic"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1informer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1lister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// Cache get method use for to get policy names and mostly use to test cache testcases
type Cache interface {
	// GetPolicies returns all policies that apply to a namespace, including cluster-wide policies
	// If the namespace is empty, only cluster-wide policies are returned
	GetPolicies(PolicyType, string, string) []kyvernov1.PolicyInterface
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
	cpolLister kyvernov1lister.ClusterPolicyLister
	polLister  kyvernov1lister.PolicyLister
	pCounter   int64
}

// NewCache create a new Cache
func NewCache(pInformer kyvernov1informer.ClusterPolicyInformer, nspInformer kyvernov1informer.PolicyInformer) Cache {
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

func (c *controller) GetPolicies(pkey PolicyType, kind, nspace string) []kyvernov1.PolicyInterface {
	var result []kyvernov1.PolicyInterface
	result = append(result, c.store.get(pkey, kind, "")...)
	result = append(result, c.store.get(pkey, "*", "")...)
	if nspace != "" {
		result = append(result, c.store.get(pkey, kind, nspace)...)
		result = append(result, c.store.get(pkey, "*", nspace)...)
	}
	return result
}

func (c *controller) CheckPolicySync(stopCh <-chan struct{}) {
	logger.Info("starting")
	policies := []kyvernov1.PolicyInterface{}
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
		c.store.set(policy)
		atomic.AddInt64(&c.pCounter, ^int64(0))
	}
	if !c.hasPolicySynced() {
		logger.Error(nil, "Failed to sync policy with cache")
		os.Exit(1)
	}
}

func (c *controller) addPolicy(obj interface{}) {
	p := obj.(*kyvernov1.ClusterPolicy)
	c.store.set(p)
}

func (c *controller) updatePolicy(old, cur interface{}) {
	pOld := old.(*kyvernov1.ClusterPolicy)
	pNew := cur.(*kyvernov1.ClusterPolicy)
	if reflect.DeepEqual(pOld.Spec, pNew.Spec) {
		return
	}
	c.store.set(pNew)
}

func (c *controller) deletePolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
	if ok {
		c.store.unset(p)
	} else {
		logger.Info("Failed to get deleted object, the deleted cluster policy cannot be removed from the cache", "obj", obj)
	}
}

func (c *controller) addNsPolicy(obj interface{}) {
	p := obj.(*kyvernov1.Policy)
	c.store.set(p)
}

func (c *controller) updateNsPolicy(old, cur interface{}) {
	npOld := old.(*kyvernov1.Policy)
	npNew := cur.(*kyvernov1.Policy)
	if reflect.DeepEqual(npOld.Spec, npNew.Spec) {
		return
	}
	c.store.set(npNew)
}

func (c *controller) deleteNsPolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.Policy)
	if ok {
		c.store.unset(p)
	} else {
		logger.Info("Failed to get deleted object, the deleted policy cannot be removed from the cache", "obj", obj)
	}
}

func (c *controller) hasPolicySynced() bool {
	return atomic.LoadInt64(&c.pCounter) == 0
}
