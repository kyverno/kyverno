package policyviolation

import (
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"

	policyreportinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/policyreport/v1alpha1"
	"github.com/nirmata/kyverno/pkg/jobs"
	"github.com/nirmata/kyverno/pkg/policyreport"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernov1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
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
	kyvernoInterface kyvernov1.KyvernoV1Interface
	// get/list cluster policy violation
	cpvLister kyvernolister.ClusterPolicyViolationLister
	// get/ist namespaced policy violation
	nspvLister kyvernolister.PolicyViolationLister
	// returns true if the cluster policy store has been synced at least once
	pvSynced cache.InformerSynced
	// returns true if the namespaced cluster policy store has been synced at at least once
	log                  logr.Logger
	nspvSynced           cache.InformerSynced
	queue                workqueue.RateLimitingInterface
	dataStore            *dataStore
	policyStatusListener policystatus.Listener
	prgen                *policyreport.Generator
	job                  *jobs.Job
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
func NewPVGenerator(client *kyvernoclient.Clientset,
	dclient *dclient.Client,
	pvInformer kyvernoinformer.ClusterPolicyViolationInformer,
	nspvInformer kyvernoinformer.PolicyViolationInformer,
	prInformer policyreportinformer.ClusterPolicyReportInformer,
	nsprInformer policyreportinformer.PolicyReportInformer,
	policyStatus policystatus.Listener,
	job *jobs.Job,
	log logr.Logger,
	stopChna <-chan struct{}) *Generator {
	gen := Generator{
		kyvernoInterface:     client.KyvernoV1(),
		dclient:              dclient,
		cpvLister:            pvInformer.Lister(),
		pvSynced:             pvInformer.Informer().HasSynced,
		nspvLister:           nspvInformer.Lister(),
		nspvSynced:           nspvInformer.Informer().HasSynced,
		queue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName),
		dataStore:            newDataStore(),
		log:                  log,
		job:                  job,
		policyStatusListener: policyStatus,
	}
	if os.Getenv("POLICY-TYPE") == "POLICYREPORT" {
		gen.prgen = policyreport.NewPRGenerator(client,
			dclient,
			prInformer,
			nsprInformer,
			policyStatus,
			job,
			log,
			stopChna,
		)
		go gen.prgen.Run(3, stopChna)

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

	if !cache.WaitForCacheSync(stopCh, gen.pvSynced, gen.nspvSynced) {
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
	if os.Getenv("POLICY-TYPE") == "POLICYREPORT" {
		gen.prgen.Add(policyreport.Info(info))
		return nil
	}
	var handler pvGenerator
	builder := newPvBuilder()
	if info.Resource.GetNamespace() == "" {
		// cluster scope resource generate a clusterpolicy violation
		handler = newClusterPV(gen.log.WithName("ClusterPV"), gen.dclient, gen.cpvLister, gen.kyvernoInterface, gen.policyStatusListener)
	} else {
		// namespaced resources generated a namespaced policy violation in the namespace of the resource
		handler = newNamespacedPV(gen.log.WithName("NamespacedPV"), gen.dclient, gen.nspvLister, gen.kyvernoInterface, gen.policyStatusListener)
	}

	failure := false
	pv := builder.generate(info)

	if info.FromSync {
		pv.Annotations = map[string]string{
			"fromSync": "true",
		}
	}

	// Create Policy Violations
	logger.V(4).Info("creating policy violation", "key", info.toKey())
	if err := handler.create(pv); err != nil {
		failure = true
		logger.Error(err, "failed to create policy violation")
	}

	if failure {
		// even if there is a single failure we requeue the request
		return errors.New("Failed to process some policy violations, re-queuing")
	}
	return nil
}

// Provides an interface to generate policy violations
// implementations for namespaced and cluster PV
type pvGenerator interface {
	create(policyViolation kyverno.PolicyViolationTemplate) error
}
