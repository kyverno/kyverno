package policyviolation

import (
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernov1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"

	client "github.com/nirmata/kyverno/pkg/dclient"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const workQueueName = "policy-violation-controller"
const workQueueRetryLimit = 3

//Generator creates PV
type Generator struct {
	dclient     *dclient.Client
	pvInterface kyvernov1.KyvernoV1Interface
	// get/list cluster policy violation
	pvLister kyvernolister.ClusterPolicyViolationLister
	// get/ist namespaced policy violation
	nspvLister kyvernolister.PolicyViolationLister
	// returns true if the cluster policy store has been synced at least once
	pvSynced cache.InformerSynced
	// returns true if the namespaced cluster policy store has been synced at at least once
	nspvSynced cache.InformerSynced
	queue      workqueue.RateLimitingInterface
	dataStore  *dataStore
}

//NewDataStore returns an instance of data store
func newDataStore() *dataStore {
	ds := dataStore{
		data: make(map[string]Info),
	}
	return &ds
}

type dataStore struct {
	data map[string]Info
	mu   sync.RWMutex
}

func (ds *dataStore) add(keyHash string, info Info) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	// queue the key hash
	ds.data[keyHash] = info
}

func (ds *dataStore) lookup(keyHash string) Info {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.data[keyHash]
}

func (ds *dataStore) delete(keyHash string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	delete(ds.data, keyHash)
}

//Info is a request to create PV
type Info struct {
	Blocked    bool
	PolicyName string
	Resource   unstructured.Unstructured
	Rules      []kyverno.ViolatedRule
}

func (i Info) toKey() string {
	keys := []string{
		strconv.FormatBool(i.Blocked),
		i.PolicyName,
		i.Resource.GetKind(),
		i.Resource.GetNamespace(),
		i.Resource.GetName(),
		strconv.Itoa(len(i.Rules)),
	}
	return strings.Join(keys, "/")
}

// make the struct hashable

//GeneratorInterface provides API to create PVs
type GeneratorInterface interface {
	Add(infos ...Info)
}

// NewPVGenerator returns a new instance of policy violation generator
func NewPVGenerator(client *kyvernoclient.Clientset, dclient *client.Client,
	pvInformer kyvernoinformer.ClusterPolicyViolationInformer,
	nspvInformer kyvernoinformer.PolicyViolationInformer) *Generator {
	gen := Generator{
		pvInterface: client.KyvernoV1(),
		dclient:     dclient,
		pvLister:    pvInformer.Lister(),
		pvSynced:    pvInformer.Informer().HasSynced,
		nspvLister:  nspvInformer.Lister(),
		nspvSynced:  nspvInformer.Informer().HasSynced,
		queue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName),
		dataStore:   newDataStore(),
	}
	return &gen
}

func (gen *Generator) enqueue(info Info) {
	// add to data map
	keyHash := info.toKey()
	// add to
	// queue the key hash
	gen.dataStore.add(keyHash, info)
	gen.queue.Add(keyHash)
}

//Add queues a policy violation create request
func (gen *Generator) Add(infos ...Info) {
	for _, info := range infos {
		gen.enqueue(info)
	}
}

// Run starts the workers
func (gen *Generator) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	glog.Info("Start policy violation generator")
	defer glog.Info("Shutting down policy violation generator")

	if !cache.WaitForCacheSync(stopCh, gen.pvSynced, gen.nspvSynced) {
		glog.Error("policy violation generator: failed to sync informer cache")
	}

	for i := 0; i < workers; i++ {
		go wait.Until(gen.runWorker, time.Second, stopCh)
	}
	<-stopCh
}

func (gen *Generator) runWorker() {
	for gen.processNextWorkitem() {
	}
}

func (gen *Generator) handleErr(err error, key interface{}) {
	if err == nil {
		gen.queue.Forget(key)
		return
	}

	// retires requests if there is error
	if gen.queue.NumRequeues(key) < workQueueRetryLimit {
		glog.V(4).Infof("Error syncing policy violation %v: %v", key, err)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		gen.queue.AddRateLimited(key)
		return
	}
	gen.queue.Forget(key)
	glog.Error(err)
	// remove from data store
	if keyHash, ok := key.(string); ok {
		gen.dataStore.delete(keyHash)
	}

	glog.Warningf("Dropping the key out of the queue: %v", err)
}

func (gen *Generator) processNextWorkitem() bool {
	obj, shutdown := gen.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer gen.queue.Done(obj)
		var keyHash string
		var ok bool
		if keyHash, ok = obj.(string); !ok {
			gen.queue.Forget(obj)
			glog.Warningf("Expecting type string but got %v\n", obj)
			return nil
		}
		// lookup data store
		info := gen.dataStore.lookup(keyHash)
		if reflect.DeepEqual(info, Info{}) {
			// empty key
			gen.queue.Forget(obj)
			glog.Warningf("Got empty key %v\n", obj)
			return nil
		}
		err := gen.syncHandler(info)
		gen.handleErr(err, obj)
		return nil
	}(obj)
	if err != nil {
		glog.Error(err)
		return true
	}
	return true
}

func (gen *Generator) syncHandler(info Info) error {
	glog.V(4).Infof("recieved info:%v", info)

	// cluster scope resource generate a clusterpolicy violation
	// namespaced resources generated a namespaced policy violation in the namespace of the resource
	if info.Resource.GetNamespace() == "" {
		return gen.createCusterPV(info)
	}
	return gen.createNamespacedPV(info)

}
