package policyreport

import (
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	policyreportclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	policyreportv1alpha1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha1"
	policyreportinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/policyreport/v1alpha1"
	policyreportlister "github.com/nirmata/kyverno/pkg/client/listers/policyreport/v1alpha1"

	"github.com/nirmata/kyverno/pkg/constant"
	"github.com/nirmata/kyverno/pkg/policystatus"

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
	dclient          *dclient.Client

	policyreportInterface policyreportv1alpha1.PolicyV1alpha1Interface
	// get/list cluster policy report
	cprLister policyreportlister.ClusterPolicyReportLister
	// get/ist namespaced policy report
	nsprLister policyreportlister.PolicyReportLister
	// returns true if the cluster policy store has been synced at least once
	prSynced cache.InformerSynced
	// returns true if the namespaced cluster policy store has been synced at at least once
	log                  logr.Logger
	nsprSynced           cache.InformerSynced
	queue                workqueue.RateLimitingInterface
	dataStore            *dataStore
	policyStatusListener policystatus.Listener
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
	PolicyName string
	Resource   unstructured.Unstructured
	Rules      []kyverno.ViolatedRule
	FromSync   bool
}

func (i Info) toKey() string {
	keys := []string{
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
func NewPVGenerator(client *policyreportclient.Clientset,
	dclient *dclient.Client,
	prInformer policyreportinformer.ClusterPolicyReportInformer,
	nsprInformer policyreportinformer.PolicyReportInformer,
	policyStatus policystatus.Listener,
	log logr.Logger) *Generator {
	gen := Generator{
		policyreportInterface:     client.PolicyV1alpha1(),
		dclient:              dclient,
		cprLister:            prInformer.Lister(),
		prSynced:             prInformer.Informer().HasSynced,
		nsprLister:           nsprInformer.Lister(),
		nsprSynced:           nsprInformer.Informer().HasSynced,
		queue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName),
		dataStore:            newDataStore(),
		log:                  log,
		policyStatusListener: policyStatus,
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
	logger := gen.log
	defer utilruntime.HandleCrash()
	logger.Info("start")
	defer logger.Info("shutting down")

	if !cache.WaitForCacheSync(stopCh, gen.prSynced, gen.nsprSynced) {
		logger.Info("failed to sync informer cache")
	}

	for i := 0; i < workers; i++ {
		go wait.Until(gen.runWorker, constant.PolicyViolationControllerResync, stopCh)
	}
	<-stopCh
}

func (gen *Generator) runWorker() {
	for gen.processNextWorkItem() {
	}
}

func (gen *Generator) handleErr(err error, key interface{}) {
	logger := gen.log
	if err == nil {
		gen.queue.Forget(key)
		return
	}

	// retires requests if there is error
	if gen.queue.NumRequeues(key) < workQueueRetryLimit {
		logger.Error(err, "failed to sync policy violation", "key", key)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		gen.queue.AddRateLimited(key)
		return
	}
	gen.queue.Forget(key)
	// remove from data store
	if keyHash, ok := key.(string); ok {
		gen.dataStore.delete(keyHash)
	}
	logger.Error(err, "dropping key out of the queue", "key", key)
}

func (gen *Generator) processNextWorkItem() bool {
	logger := gen.log
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
			logger.Info("incorrect type; expecting type 'string'", "obj", obj)
			return nil
		}

		// lookup data store
		info := gen.dataStore.lookup(keyHash)
		if reflect.DeepEqual(info, Info{}) {
			// empty key
			gen.queue.Forget(obj)
			logger.Info("empty key")
			return nil
		}

		err := gen.syncHandler(info)
		gen.handleErr(err, obj)
		return nil
	}(obj)

	if err != nil {
		logger.Error(err, "failed to process item")
		return true
	}

	return true
}

func (gen *Generator) syncHandler(info Info) error {
	logger := gen.log
	resource, err := gen.dclient.GetResource(info.Resource.GetAPIVersion(),info.Resource.GetKind(),info.Resource.GetName(),info.Resource.GetNamespace())
	if  err != nil {
		logger.Error(err, "failed to get resource")
	}
	labels := resource.GetLabels();
	if _,okChart := labels["helm.sh/chart"]; !okChart {
		// cluster scope resource generate a helm package report
		handler := newHelmPR(gen.log.WithName("NamespacedPV"), gen.dclient, gen.nsprLister, gen.policyreportInterface, gen.policyStatusListener)
		handler.Add(info)
	}else if info.Resource.GetNamespace() == "" {
		// cluster scope resource generate a clusterpolicy violation
		handler := newClusterPR(gen.log.WithName("ClusterPV"), gen.dclient, gen.cprLister, gen.policyreportInterface, gen.policyStatusListener)
		handler.Add(info)
	} else {
		// namespaced resources generated a namespaced policy violation in the namespace of the resource
		handler := newNamespacedPR(gen.log.WithName("NamespacedPV"), gen.dclient, gen.nsprLister, gen.policyreportInterface, gen.policyStatusListener)
		handler.Add(info)
	}

	return nil
}

