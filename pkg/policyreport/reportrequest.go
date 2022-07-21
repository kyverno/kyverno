package policyreport

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	policyreportclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha2"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	requestlister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine/response"
	cmap "github.com/orcaman/concurrent-map"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const workQueueName = "report-request-controller"
const workQueueRetryLimit = 10

// Generator creates report request
type Generator struct {
	dclient dclient.Interface

	reportChangeRequestLister requestlister.ReportChangeRequestLister

	clusterReportChangeRequestLister requestlister.ClusterReportChangeRequestLister

	// changeRequestMapper stores the change requests' count per namespace
	changeRequestMapper concurrentMap

	// cpolLister can list/get policy from the shared informer's store
	cpolLister kyvernolister.ClusterPolicyLister

	// polLister can list/get namespace policy from the shared informer's store
	polLister kyvernolister.PolicyLister

	informersSynced []cache.InformerSynced

	queue     workqueue.RateLimitingInterface
	dataStore *dataStore

	requestCreator creator

	// changeRequestLimit defines the max count for change requests (per namespace for RCR / cluster-wide for CRCR)
	changeRequestLimit int

	// CleanupChangeRequest signals the policy report controller to cleanup change requests
	CleanupChangeRequest chan ReconcileInfo

	log logr.Logger
}

// NewReportChangeRequestGenerator returns a new instance of report request generator
func NewReportChangeRequestGenerator(client policyreportclient.Interface,
	dclient dclient.Interface,
	reportReqInformer kyvernov1alpha2informers.ReportChangeRequestInformer,
	clusterReportReqInformer kyvernov1alpha2informers.ClusterReportChangeRequestInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	changeRequestLimit int,
	log logr.Logger,
) *Generator {
	gen := Generator{
		dclient:                          dclient,
		clusterReportChangeRequestLister: clusterReportReqInformer.Lister(),
		reportChangeRequestLister:        reportReqInformer.Lister(),
		changeRequestMapper:              newChangeRequestMapper(),
		cpolLister:                       cpolInformer.Lister(),
		polLister:                        polInformer.Lister(),
		queue:                            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName),
		dataStore:                        newDataStore(),
		changeRequestLimit:               changeRequestLimit,
		CleanupChangeRequest:             make(chan ReconcileInfo, 10),
		requestCreator:                   newChangeRequestCreator(client, 3*time.Second, log.WithName("requestCreator")),
		log:                              log,
	}

	gen.informersSynced = []cache.InformerSynced{clusterReportReqInformer.Informer().HasSynced, reportReqInformer.Informer().HasSynced, cpolInformer.Informer().HasSynced, polInformer.Informer().HasSynced}

	return &gen
}

type ReconcileInfo struct {
	Namespace      *string
	MapperInactive bool
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

// Info stores the policy application results for all matched resources
// Namespace is set to empty "" if resource is cluster wide resource
type Info struct {
	PolicyName string
	Namespace  string
	Results    []EngineResponseResult
}

type EngineResponseResult struct {
	Resource response.ResourceSpec
	Rules    []kyverno.ViolatedRule
}

func (i Info) ToKey() string {
	keys := []string{
		i.PolicyName,
		i.Namespace,
		strconv.Itoa(len(i.Results)),
	}

	for _, result := range i.Results {
		keys = append(keys, result.Resource.GetKey())
	}
	return strings.Join(keys, "/")
}

func (i Info) GetRuleLength() int {
	l := 0
	for _, res := range i.Results {
		l += len(res.Rules)
	}
	return l
}

func parseKeyHash(keyHash string) (policyName, ns string) {
	keys := strings.Split(keyHash, "/")
	return keys[0], keys[1]
}

// GeneratorInterface provides API to create PVs
type GeneratorInterface interface {
	Add(infos ...Info)
	MapperReset(string)
	MapperInactive(string)
	MapperInvalidate()
}

func (gen *Generator) enqueue(info Info) {
	keyHash := info.ToKey()
	gen.dataStore.add(keyHash, info)
	gen.queue.Add(keyHash)
}

// Add queues a policy violation create request
func (gen *Generator) Add(infos ...Info) {
	for _, info := range infos {
		count, ok := gen.changeRequestMapper.ConcurrentMap.Get(info.Namespace)
		if ok && count == -1 {
			gen.log.V(6).Info("inactive policy report, skip creating report change request", "namespace", info.Namespace)
			continue
		}

		gen.changeRequestMapper.increase(info.Namespace)
		gen.enqueue(info)
	}
}

// MapperReset resets the change request mapper for the given namespace
func (gen Generator) MapperReset(ns string) {
	gen.changeRequestMapper.ConcurrentMap.Set(ns, 0)
}

// MapperInactive sets the change request mapper for the given namespace to -1
// which indicates the report is inactive
func (gen Generator) MapperInactive(ns string) {
	gen.changeRequestMapper.ConcurrentMap.Set(ns, -1)
}

// MapperInvalidate reset map entries
func (gen Generator) MapperInvalidate() {
	for ns := range gen.changeRequestMapper.ConcurrentMap.Items() {
		gen.changeRequestMapper.ConcurrentMap.Remove(ns)
	}
}

// Run starts the workers
func (gen *Generator) Run(workers int, stopCh <-chan struct{}) {
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

func (gen *Generator) runWorker() {
	for gen.processNextWorkItem() {
	}
}

func (gen *Generator) handleErr(err error, key interface{}) {
	logger := gen.log
	keyHash, ok := key.(string)
	if !ok {
		keyHash = ""
	}

	if err == nil {
		gen.queue.Forget(key)
		gen.dataStore.delete(keyHash)
		gen.changeRequestMapper.decrease(keyHash)
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
	gen.changeRequestMapper.decrease(keyHash)
}

func (gen *Generator) processNextWorkItem() bool {
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

	count, ok := gen.changeRequestMapper.Get(info.Namespace)
	if ok {
		if count.(int) > gen.changeRequestLimit {
			logger.Info("throttling report change requests", "namespace", info.Namespace, "threshold", gen.changeRequestLimit, "count", count.(int))
			gen.CleanupChangeRequest <- ReconcileInfo{Namespace: &(info.Namespace), MapperInactive: false}
			gen.queue.Forget(obj)
			gen.dataStore.delete(keyHash)
			return true
		}
	}

	err := gen.syncHandler(info)
	gen.handleErr(err, obj)

	return true
}

func (gen *Generator) syncHandler(info Info) error {
	builder := NewBuilder(gen.cpolLister, gen.polLister)
	reportReq, err := builder.build(info)
	if err != nil {
		return fmt.Errorf("unable to build reportChangeRequest: %v", err)
	}

	if reportReq == nil {
		return nil
	}

	gen.requestCreator.add(reportReq)
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

func newChangeRequestMapper() concurrentMap {
	return concurrentMap{cmap.New()}
}
