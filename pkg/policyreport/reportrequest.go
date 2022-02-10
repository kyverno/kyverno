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
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	requestinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha2"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	requestlister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine/response"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const workQueueName = "report-request-controller"
const workQueueRetryLimit = 10

// Generator creates report request
type Generator struct {
	dclient *dclient.Client

	reportChangeRequestLister requestlister.ReportChangeRequestLister

	clusterReportChangeRequestLister requestlister.ClusterReportChangeRequestLister

	// cpolLister can list/get policy from the shared informer's store
	cpolLister kyvernolister.ClusterPolicyLister

	// polLister can list/get namespace policy from the shared informer's store
	polLister kyvernolister.PolicyLister

	// returns true if the cluster report request store has been synced at least once
	reportReqSynced cache.InformerSynced

	// returns true if the namespaced report request store has been synced at at least once
	clusterReportReqSynced cache.InformerSynced

	// cpolListerSynced returns true if the cluster policy store has been synced at least once
	cpolListerSynced cache.InformerSynced

	// polListerSynced returns true if the namespace policy store has been synced at least once
	polListerSynced cache.InformerSynced

	queue     workqueue.RateLimitingInterface
	dataStore *dataStore

	requestCreator creator

	log logr.Logger
}

// NewReportChangeRequestGenerator returns a new instance of report request generator
func NewReportChangeRequestGenerator(client *policyreportclient.Clientset,
	dclient *dclient.Client,
	reportReqInformer requestinformer.ReportChangeRequestInformer,
	clusterReportReqInformer requestinformer.ClusterReportChangeRequestInformer,
	cpolInformer kyvernoinformer.ClusterPolicyInformer,
	polInformer kyvernoinformer.PolicyInformer,
	log logr.Logger) *Generator {
	gen := Generator{
		dclient:                          dclient,
		clusterReportChangeRequestLister: clusterReportReqInformer.Lister(),
		clusterReportReqSynced:           clusterReportReqInformer.Informer().HasSynced,
		reportChangeRequestLister:        reportReqInformer.Lister(),
		reportReqSynced:                  reportReqInformer.Informer().HasSynced,
		cpolLister:                       cpolInformer.Lister(),
		cpolListerSynced:                 cpolInformer.Informer().HasSynced,
		polLister:                        polInformer.Lister(),
		polListerSynced:                  polInformer.Informer().HasSynced,
		queue:                            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName),
		dataStore:                        newDataStore(),
		requestCreator:                   newChangeRequestCreator(dclient, 3*time.Second, log.WithName("requestCreator")),
		log:                              log,
	}

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

// GeneratorInterface provides API to create PVs
type GeneratorInterface interface {
	Add(infos ...Info)
}

func (gen *Generator) enqueue(info Info) {
	keyHash := info.ToKey()
	gen.dataStore.add(keyHash, info)
	gen.queue.Add(keyHash)
}

// Add queues a policy violation create request
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

	if !cache.WaitForCacheSync(stopCh, gen.reportReqSynced, gen.clusterReportReqSynced, gen.cpolListerSynced, gen.polListerSynced) {
		logger.Info("failed to sync informer cache")
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
			gen.queue.Forget(obj)
			logger.V(4).Info("empty key")
			return nil
		}

		err := gen.syncHandler(info)
		gen.handleErr(err, obj)
		return nil
	}(obj)

	if err != nil {
		logger.Error(err, "failed to process item")
	}

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
