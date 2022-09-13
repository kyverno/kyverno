package policyreport

import (
	"reflect"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha2"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	workQueueName       = "report-request-controller"
	workQueueRetryLimit = 10
)

// generator creates report request
type generator struct {
	reportChangeRequestLister kyvernov1alpha2listers.ReportChangeRequestLister

	clusterReportChangeRequestLister kyvernov1alpha2listers.ClusterReportChangeRequestLister

	// cpolLister can list/get policy from the shared informer's store
	cpolLister kyvernov1listers.ClusterPolicyLister

	// polLister can list/get namespace policy from the shared informer's store
	polLister kyvernov1listers.PolicyLister

	informersSynced []cache.InformerSynced

	queue     workqueue.RateLimitingInterface
	dataStore *dataStore

	requestCreator creator

	// changeRequestLimit defines the max count for change requests (per namespace for RCR / cluster-wide for CRCR)
	changeRequestLimit int

	// cleanupChangeRequest signals the policy report controller to cleanup change requests
	// takes namespace in input
	cleanupChangeRequest chan string

	log logr.Logger
}

// NewReportChangeRequestGenerator returns a new instance of report request generator
func NewReportChangeRequestGenerator(client versioned.Interface,
	reportReqInformer kyvernov1alpha2informers.ReportChangeRequestInformer,
	clusterReportReqInformer kyvernov1alpha2informers.ClusterReportChangeRequestInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	changeRequestLimit int,
	log logr.Logger,
) Generator {
	gen := generator{
		clusterReportChangeRequestLister: clusterReportReqInformer.Lister(),
		reportChangeRequestLister:        reportReqInformer.Lister(),
		cpolLister:                       cpolInformer.Lister(),
		polLister:                        polInformer.Lister(),
		queue:                            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName),
		dataStore:                        newDataStore(),
		changeRequestLimit:               changeRequestLimit,
		cleanupChangeRequest:             make(chan string, 10),
		requestCreator:                   newChangeRequestCreator(client, 3*time.Second, log.WithName("requestCreator")),
		log:                              log,
	}

	gen.informersSynced = []cache.InformerSynced{clusterReportReqInformer.Informer().HasSynced, reportReqInformer.Informer().HasSynced, cpolInformer.Informer().HasSynced, polInformer.Informer().HasSynced}
	return &gen
}

// NewDataStore returns an instance of data store
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

// Generator provides API to create PVs
type Generator interface {
	Cleanup() chan string
	Run(int, <-chan struct{})
	Add(...Info)
}

func (gen *generator) enqueue(info Info) {
	keyHash := info.ToKey()
	gen.dataStore.add(keyHash, info)
	gen.queue.Add(keyHash)
}

// Add queues a policy violation create request
func (gen *generator) Add(infos ...Info) {
	for _, info := range infos {
		gen.enqueue(info)
	}
}

func (gen *generator) Cleanup() chan string {
	return gen.cleanupChangeRequest
}

// Run starts the workers
func (gen *generator) Run(workers int, stopCh <-chan struct{}) {
	logger := gen.log
	defer utilruntime.HandleCrash()
	logger.Info("start")
	defer logger.Info("shutting down")

	if !cache.WaitForNamedCacheSync("requestCreator", stopCh, gen.informersSynced...) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(gen.runWorker, time.Second, stopCh)
	}

	go gen.requestCreator.run(stopCh)

	<-stopCh
}

func (gen *generator) runWorker() {
	for gen.processNextWorkItem() {
	}
}

func (gen *generator) handleErr(err error, key interface{}) {
	logger := gen.log
	keyHash, ok := key.(string)
	if !ok {
		keyHash = ""
	}

	if err == nil {
		gen.queue.Forget(key)
		gen.dataStore.delete(keyHash)
		return
	}

	// retires requests if there is error
	if gen.queue.NumRequeues(key) < workQueueRetryLimit {
		logger.V(3).Info("retrying report request", "key", key, "error", err)
		gen.queue.AddRateLimited(key)
		return
	}

	logger.Error(err, "failed to process report request", "key", key)
	gen.queue.Forget(key)
	gen.dataStore.delete(keyHash)
}

func (gen *generator) getReportChangeRequestCount(ns string) int {
	if ns == "" {
		items, _ := gen.clusterReportChangeRequestLister.List(labels.Everything())
		return len(items)
	} else {
		selector := labels.SelectorFromSet(labels.Set(map[string]string{ResourceLabelNamespace: ns}))
		items, _ := gen.reportChangeRequestLister.List(selector)
		return len(items)
	}
}

func (gen *generator) processNextWorkItem() bool {
	logger := gen.log
	obj, shutdown := gen.queue.Get()
	if shutdown {
		return false
	}

	defer gen.queue.Done(obj)
	var keyHash string
	var ok bool

	if keyHash, ok = obj.(string); !ok {
		logger.Info("incorrect type; expecting type 'string'", "obj", obj)
		gen.handleErr(nil, obj)
		return true
	}

	// lookup data store
	info := gen.dataStore.lookup(keyHash)
	if reflect.DeepEqual(info, Info{}) {
		logger.V(4).Info("empty key")
		gen.handleErr(nil, obj)
		return true
	}

	count := gen.getReportChangeRequestCount(info.Namespace)
	if count > gen.changeRequestLimit {
		logger.Info("throttling report change requests", "namespace", info.Namespace, "threshold", gen.changeRequestLimit, "count", count)
		gen.cleanupChangeRequest <- info.Namespace
		gen.queue.Forget(obj)
		gen.dataStore.delete(keyHash)
		return true
	}

	err := gen.syncHandler(info)
	gen.handleErr(err, obj)

	return true
}

func (gen *generator) syncHandler(info Info) error {
	builder := NewBuilder(gen.cpolLister, gen.polLister)
	crcr, rcr, err := builder.build(info)
	if err != nil {
		return errors.Wrapf(err, "unable to build reportChangeRequest: %v", err)
	}

	if crcr == nil && rcr == nil {
		return nil
	}

	gen.requestCreator.add(crcr, rcr)
	return nil
}

func hasResultsChanged(old, new map[string]interface{}) bool {
	var oldRes, newRes []interface{}
	if val, ok := old["results"]; ok {
		oldRes = val.([]interface{})
	}

	if val, ok := new["results"]; ok {
		newRes = val.([]interface{})
	}

	if len(oldRes) != len(newRes) {
		return true
	}

	return !reflect.DeepEqual(oldRes, newRes)
}
