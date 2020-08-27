package jobs

import (
	"fmt"
	"github.com/docker/docker/daemon/logger"
	"github.com/nirmata/kyverno/pkg/config"
	batchv1 "k8s.io/client-go/pkg/apis/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/api/core/v1"
	"sync"

	"github.com/go-logr/logr"
	policyreportclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"

	"github.com/nirmata/kyverno/pkg/constant"

	dclient "github.com/nirmata/kyverno/pkg/dclient"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

const workQueueName = "policy-violation-controller"
const workQueueRetryLimit = 3

//Generator creates PV
type Generator struct {
	dclient *dclient.Client
	log                  logr.Logger
	queue                workqueue.RateLimitingInterface
	dataStore            *dataStore
	mux sync.Mutex
}

type JobInfo struct {
  JobType string
  Policy string
}

func (i JobInfo) toKey() string {
	if i.Policy != "" {
		return fmt.Sprintf("%s-%s",i.JobType,i.Policy)
	}
	return fmt.Sprintf("%s",i.JobType)
}

//NewDataStore returns an instance of data store
func newDataStore() *dataStore {
	ds := dataStore{
		data: make(map[string]JobInfo),
	}
	return &ds
}

type dataStore struct {
	data map[string]JobInfo
	mu   sync.RWMutex
}

func (ds *dataStore) add(keyHash string, info JobInfo) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	// queue the key hash
	ds.data[keyHash] = info
}

func (ds *dataStore) lookup(keyHash string) JobInfo {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.data[keyHash]
}

func (ds *dataStore) delete(keyHash string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	delete(ds.data, keyHash)
}

// make the struct hashable

//JobsInterface provides API to create PVs
type JobsInterface interface {
	Add(infos ...JobInfo)
}


// NewJobsGenerator returns a new instance of policy violation generator
func NewJobsGenerator(client *policyreportclient.Clientset,
	dclient *dclient.Client,
	log logr.Logger) *Generator {
	gen := Generator{
		dclient:               dclient,
		queue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName),
		dataStore:             newDataStore(),
		log:                   log,
	}
	return &gen
}

func (gen *Generator) enqueue(info JobInfo) {
	// add to data map
	keyHash := info.toKey()
	// add to
	// queue the key hash

	gen.dataStore.add(keyHash, info)
	gen.queue.Add(keyHash)
}

//Add queues a policy violation create request
func (gen *Generator) Add(infos ...JobInfo) {
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
		if reflect.DeepEqual(info, JobInfo{}) {
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


func (gen *Generator) syncHandler(info JobInfo) error {
	defer func(){
		gen.mux.Unlock()
	}()
	gen.mux.Lock()
	if len(info.Policy) > 0 {
		var wg sync.WaitGroup
		wg.Add(3)
		go gen.syncNamespace(&wg,"HELM","POLICY",info.Policy)
		go gen.syncNamespace(&wg,"NAMESPACE","POLICY",info.Policy)
		go gen.syncNamespace(&wg,"CLUSTER","POLICY",info.Policy)
		wg.Wait()
		return nil
	}
	var wg sync.WaitGroup
	wg.Add(3)
	go gen.syncNamespace(&wg,"HELM","SYNC",info.Policy)
	go gen.syncNamespace(&wg,"NAMESPACE","SYNC",info.Policy)
	go gen.syncNamespace(&wg,"CLUSTER","SYNC",info.Policy)
	wg.Wait()
	return nil
	return nil
}

func(gen *Generator) syncNamespace(wg *sync.WaitGroup,jobType,scope,policy string){
	defer func(){
		wg.Done()
	}()
	var args []string{}
	if len(policy) > 0 {
		 args = []string{
			"report",
			"--policy",
		}
	}else{
		args = []string{
			"report",
		}
	}

	var job *batchv1.Job
	switch jobType {
	case "HELM":
		job = CreateJob(append(args,"helm"),jobType,scope)
		break;
	case "NAMESPACE" :
		job = CreateJob(append(args,"namespace"),jobType,scope)
		break;
	case "CLUSTER":
		job = CreateJob(append(args,"cluster"),jobType,scope)
		break
	}
	_, err := gen.dclient.UpdateStatusResource("","Job",config.KubePolicyNamespace,job,false)
	if err != nil {
		return
	}
	return
}

func CreateJob(args []string,jobType,scope string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s",jobType,scope),
			Namespace: config.KubePolicyNamespace,
		},
		Spec: batchv1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  fmt.Sprintf("%s-%s",jobType,scope),
							Image: "nirmata/kyverno-cli:latest",
							Args: args,
						},
					},
				},
			},
		},
	}
}

