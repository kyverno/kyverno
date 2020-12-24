package policyreport

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	changerequest "github.com/kyverno/kyverno/pkg/api/kyverno/v1alpha1"
	policyreportclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	requestinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	requestlister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha1"
	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/policystatus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	log       logr.Logger
}

// NewReportChangeRequestGenerator returns a new instance of report request generator
func NewReportChangeRequestGenerator(client *policyreportclient.Clientset,
	dclient *dclient.Client,
	reportReqInformer requestinformer.ReportChangeRequestInformer,
	clusterReportReqInformer requestinformer.ClusterReportChangeRequestInformer,
	cpolInformer kyvernoinformer.ClusterPolicyInformer,
	polInformer kyvernoinformer.PolicyInformer,
	policyStatus policystatus.Listener,
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
		logger.V(3).Info("retrying report request", "key", key, "error", err)
		gen.queue.AddRateLimited(key)
		return
	}

	logger.Error(err, "failed to process report request", "key", key)
	gen.queue.Forget(key)
	if keyHash, ok := key.(string); ok {
		gen.dataStore.delete(keyHash)
	}
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
	gen.log.V(3).Info("generating report change request")

	builder := NewBuilder(gen.cpolLister, gen.polLister)
	rcrUnstructured, err := builder.build(info)
	if err != nil {
		return fmt.Errorf("unable to build reportChangeRequest: %v", err)
	}

	if rcrUnstructured == nil {
		return nil
	}

	return gen.sync(rcrUnstructured, info)
}

func (gen *Generator) sync(reportReq *unstructured.Unstructured, info Info) error {
	logger := gen.log.WithName("sync report change request")
	reportReq.SetCreationTimestamp(v1.Now())
	if reportReq.GetKind() == "ClusterReportChangeRequest" {
		return gen.syncClusterReportChangeRequest(reportReq, logger)
	}

	return gen.syncReportChangeRequest(reportReq, logger)
}

func (gen *Generator) syncClusterReportChangeRequest(reportReq *unstructured.Unstructured, logger logr.Logger) error {
	old, err := gen.clusterReportChangeRequestLister.Get(reportReq.GetName())
	if err != nil {
		if apierrors.IsNotFound(err) {
			if _, err = gen.dclient.CreateResource(reportReq.GetAPIVersion(), reportReq.GetKind(), "", reportReq, false); err != nil {
				return fmt.Errorf("failed to create clusterReportChangeRequest: %v", err)
			}

			logger.V(3).Info("successfully created clusterReportChangeRequest", "name", reportReq.GetName())
			return nil
		}
		return fmt.Errorf("unable to get %s: %v", reportReq.GetKind(), err)
	}

	return updateReportChangeRequest(gen.dclient, old, reportReq, logger)
}

func (gen *Generator) syncReportChangeRequest(reportReq *unstructured.Unstructured, logger logr.Logger) error {
	old, err := gen.reportChangeRequestLister.ReportChangeRequests(config.KyvernoNamespace).Get(reportReq.GetName())
	if err != nil {
		if apierrors.IsNotFound(err) {
			if _, err = gen.dclient.CreateResource(reportReq.GetAPIVersion(), reportReq.GetKind(), config.KyvernoNamespace, reportReq, false); err != nil {
				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("failed to create ReportChangeRequest: %v", err)
				}
			}

			logger.V(3).Info("successfully created reportChangeRequest", "name", reportReq.GetName())
			return nil
		}
		return fmt.Errorf("unable to get existing reportChangeRequest %v", err)
	}

	return updateReportChangeRequest(gen.dclient, old, reportReq, logger)
}

func updateReportChangeRequest(dClient *client.Client, old interface{}, new *unstructured.Unstructured, log logr.Logger) (err error) {
	oldUnstructured := make(map[string]interface{})
	if oldTyped, ok := old.(*changerequest.ReportChangeRequest); ok {
		if oldUnstructured, err = runtime.DefaultUnstructuredConverter.ToUnstructured(oldTyped); err != nil {
			return fmt.Errorf("unable to convert reportChangeRequest: %v", err)
		}
		new.SetResourceVersion(oldTyped.GetResourceVersion())
		new.SetUID(oldTyped.GetUID())
	} else {
		oldTyped := old.(*changerequest.ClusterReportChangeRequest)
		if oldUnstructured, err = runtime.DefaultUnstructuredConverter.ToUnstructured(oldTyped); err != nil {
			return fmt.Errorf("unable to convert clusterReportChangeRequest: %v", err)
		}
		new.SetUID(oldTyped.GetUID())
		new.SetResourceVersion(oldTyped.GetResourceVersion())
	}

	if !hasResultsChanged(oldUnstructured, new.UnstructuredContent()) {
		log.V(4).Info("unchanged report request", "name", new.GetName())
		return nil
	}

	if _, err = dClient.UpdateResource(new.GetAPIVersion(), new.GetKind(), config.KyvernoNamespace, new, false); err != nil {
		return fmt.Errorf("failed to update report request: %v", err)
	}

	log.V(4).Info("successfully updated report request", "kind", new.GetKind(), "name", new.GetName())
	return
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
