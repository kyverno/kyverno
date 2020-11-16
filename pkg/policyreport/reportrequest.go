package policyreport

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	changerequest "github.com/kyverno/kyverno/pkg/api/kyverno/v1alpha1"
	policyreportclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	requestinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	requestlister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/constant"
	client "github.com/kyverno/kyverno/pkg/dclient"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
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
const workQueueRetryLimit = 3

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

	// update policy status with violationCount
	policyStatusListener policystatus.Listener

	log logr.Logger
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
		policyStatusListener:             policyStatus,
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

// GeneratorInterface provides API to create PVs
type GeneratorInterface interface {
	Add(infos ...Info)
}

func (gen *Generator) enqueue(info Info) {
	keyHash := info.toKey()
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
		go wait.Until(gen.runWorker, constant.PolicyReportControllerResync, stopCh)
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
		logger.Error(err, "failed to sync report request", "key", key)
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
			gen.queue.Forget(obj)
			logger.V(3).Info("empty key")
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
	reportChangeRequestUnstructured, err := builder.build(info)
	if err != nil {
		return fmt.Errorf("unable to build reportChangeRequest: %v", err)
	}

	if reportChangeRequestUnstructured == nil {
		return nil
	}

	return gen.sync(reportChangeRequestUnstructured, info)
}

func (gen *Generator) sync(reportReq *unstructured.Unstructured, info Info) error {
	defer func() {
		if val := reportReq.GetAnnotations()["fromSync"]; val == "true" {
			gen.policyStatusListener.Send(violationCount{
				policyName:    info.PolicyName,
				violatedRules: info.Rules,
			})
		}
	}()

	logger := gen.log.WithName("sync")
	reportReq.SetCreationTimestamp(v1.Now())
	if reportReq.GetNamespace() == "" {
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

	old, err := gen.reportChangeRequestLister.ReportChangeRequests(config.KubePolicyNamespace).Get(reportReq.GetName())
	if err != nil {
		if apierrors.IsNotFound(err) {
			if _, err = gen.dclient.CreateResource(reportReq.GetAPIVersion(), reportReq.GetKind(), config.KubePolicyNamespace, reportReq, false); err != nil {
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
	oldUnstructed := make(map[string]interface{})
	if oldTyped, ok := old.(*changerequest.ReportChangeRequest); ok {
		if oldUnstructed, err = runtime.DefaultUnstructuredConverter.ToUnstructured(oldTyped); err != nil {
			return fmt.Errorf("unable to convert reportChangeRequest: %v", err)
		}
		new.SetResourceVersion(oldTyped.GetResourceVersion())
		new.SetUID(oldTyped.GetUID())
	} else {
		oldTyped := old.(*changerequest.ClusterReportChangeRequest)
		if oldUnstructed, err = runtime.DefaultUnstructuredConverter.ToUnstructured(oldTyped); err != nil {
			return fmt.Errorf("unable to convert clusterReportChangeRequest: %v", err)
		}
		new.SetUID(oldTyped.GetUID())
		new.SetResourceVersion(oldTyped.GetResourceVersion())
	}

	if !hasResultsChanged(oldUnstructed, new.UnstructuredContent()) {
		log.V(4).Info("unchanged report request", "name", new.GetName())
		return nil
	}
	// TODO(shuting): set annotation / label
	if _, err = dClient.UpdateResource(new.GetAPIVersion(), new.GetKind(), config.KubePolicyNamespace, new, false); err != nil {
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
