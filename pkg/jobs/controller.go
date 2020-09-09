package jobs

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/nirmata/kyverno/pkg/config"
	v1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-logr/logr"

	"github.com/nirmata/kyverno/pkg/constant"

	dclient "github.com/nirmata/kyverno/pkg/dclient"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

const workQueueName = "policy-violation-controller"
const workQueueRetryLimit = 3

//Job creates policy report
type Job struct {
	dclient   *dclient.Client
	log       logr.Logger
	queue     workqueue.RateLimitingInterface
	dataStore *dataStore
	mux       sync.Mutex
}

// Job Info Define Job Type
type JobInfo struct {
	JobType string
}

func (i JobInfo) toKey() string {
	return fmt.Sprintf("kyverno-%v", rand.Int63n(1000))
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

// NewJobsJob returns a new instance of policy violation generator
func NewJobsJob(dclient *dclient.Client,
	log logr.Logger) *Job {
	gen := Job{
		dclient:   dclient,
		queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName),
		dataStore: newDataStore(),
		log:       log,
	}
	return &gen
}

func (j *Job) enqueue(info JobInfo) {
	// add to data map
	keyHash := info.toKey()
	// add to
	// queue the key hash

	j.dataStore.add(keyHash, info)
	j.queue.Add(keyHash)
}

//Add queues a policy violation create request
func (j *Job) Add(infos ...JobInfo) {
	for _, info := range infos {
		j.enqueue(info)
	}
}

// Run starts the workers
func (j *Job) Run(workers int, stopCh <-chan struct{}) {
	logger := j.log
	defer utilruntime.HandleCrash()
	logger.Info("start")
	defer logger.Info("shutting down")

	for i := 0; i < workers; i++ {
		go wait.Until(j.runWorker, constant.PolicyViolationControllerResync, stopCh)
	}

	go func() {
		ctx := context.Background()
		ticker := time.NewTicker(100 * time.Second)
		for {
			select {
			case <-ticker.C:
				var wg sync.WaitGroup
				wg.Add(3)
				go j.syncNamespace(&wg, "Helm", "SYNC")
				go j.syncNamespace(&wg, "Namespace", "SYNC")
				go j.syncNamespace(&wg, "Cluster", "SYNC")
				wg.Wait()
			case <-ctx.Done():
				break
				// Create Jobs
			}
		}
	}()

	<-stopCh
}

func (j *Job) runWorker() {
	for j.processNextWorkItem() {
	}
}

func (j *Job) handleErr(err error, key interface{}) {
	logger := j.log
	if err == nil {
		j.queue.Forget(key)
		return
	}

	// retires requests if there is error
	if j.queue.NumRequeues(key) < workQueueRetryLimit {
		logger.Error(err, "failed to sync policy violation", "key", key)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		j.queue.AddRateLimited(key)
		return
	}
	j.queue.Forget(key)
	// remove from data store
	if keyHash, ok := key.(string); ok {
		j.dataStore.delete(keyHash)
	}
	logger.Error(err, "dropping key out of the queue", "key", key)
}

func (j *Job) processNextWorkItem() bool {
	logger := j.log
	obj, shutdown := j.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer j.queue.Done(obj)
		var keyHash string
		var ok bool

		if keyHash, ok = obj.(string); !ok {
			j.queue.Forget(obj)
			logger.Info("incorrect type; expecting type 'string'", "obj", obj)
			return nil
		}

		// lookup data store
		info := j.dataStore.lookup(keyHash)
		if reflect.DeepEqual(info, JobInfo{}) {
			// empty key
			j.queue.Forget(obj)
			logger.Info("empty key")
			return nil
		}

		err := j.syncHandler(info)
		j.handleErr(err, obj)
		return nil
	}(obj)

	if err != nil {
		logger.Error(err, "failed to process item")
		return true
	}

	return true
}

func (j *Job) syncHandler(info JobInfo) error {
	defer func() {
		j.mux.Unlock()
	}()
	j.mux.Lock()
	var wg sync.WaitGroup
	wg.Add(3)
	go j.syncNamespace(&wg, "Helm", "CONFIGMAP")
	go j.syncNamespace(&wg, "Namespace", "CONFIGMAP")
	go j.syncNamespace(&wg, "Cluster", "CONFIGMAP")
	wg.Wait()
	return nil
}

func (j *Job) syncNamespace(wg *sync.WaitGroup, jobType, scope string) {
	defer func() {
		wg.Done()
	}()
	var args []string
	var mode string
	if scope == "SYNC" {
		mode = "cli"
	} else {
		mode = "configmap"
	}

	var job *v1.Job
	switch jobType {
	case "Helm":
		args = []string{
			"report",
			"helm",
			fmt.Sprintf("--mode=%s", mode),
		}
		break
	case "Namespace":
		args = []string{
			"report",
			"namespace",
			fmt.Sprintf("--mode=%s", mode),
		}
		break
	case "Cluster":
		args = []string{
			"report",
			"cluster",
			fmt.Sprintf("--mode=%s", mode),
		}
		break
	}
	job = CreateJob(args, jobType, scope)
	_, err := j.dclient.CreateResource("", "Job", config.KubePolicyNamespace, job, false)
	if err != nil {
		return
	}
	deadline := time.Now().Add(80 * time.Second)
	for {
		resource, err := j.dclient.GetResource("", "Job", config.KubePolicyNamespace, job.GetName())
		if err != nil {
			continue
		}
		job := v1.Job{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.UnstructuredContent(), &job); err != nil {
			continue
		}
		if time.Now().After(deadline) {
			break
		}
	}
	err = j.dclient.DeleteResource("", "Job", config.KubePolicyNamespace, job.GetName(), false)
	if err != nil {
		return
	}

	return
}

// CreateJob will create Job template for background scan
func CreateJob(args []string, jobType, scope string) *v1.Job {
	job := &v1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.KubePolicyNamespace,
		},
		Spec: v1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:            strings.ToLower(fmt.Sprintf("%s-%s", jobType, scope)),
							Image:           config.KyvernoCliImage,
							ImagePullPolicy: "Always",
							Args:            args,
						},
					},
					ServiceAccountName: "kyverno-service-account",
					RestartPolicy:      "OnFailure",
				},
			},
		},
	}
	job.SetGenerateName("kyverno-policyreport-")
	return job
}
