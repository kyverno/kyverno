package jobs

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/constant"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	v1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

const workQueueName = "policy-violation-controller"
const workQueueRetryLimit = 3

//Job creates policy report
type Job struct {
	dclient       *dclient.Client
	log           logr.Logger
	queue         workqueue.RateLimitingInterface
	dataStore     *dataStore
	configHandler config.Interface
	mux           sync.Mutex
}

// JobInfo defines Job Type
type JobInfo struct {
	JobType string
	JobData string
}

func (i JobInfo) toKey() string {
	return fmt.Sprintf("kyverno-%v", i.JobType)
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

//JobsInterface provides API to create PVs
type JobsInterface interface {
	Add(infos ...JobInfo)
}

// NewJobsJob returns a new instance of jobs generator
func NewJobsJob(dclient *dclient.Client,
	configHandler config.Interface,
	log logr.Logger) *Job {
	gen := Job{
		dclient:       dclient,
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName),
		dataStore:     newDataStore(),
		configHandler: configHandler,
		log:           log,
	}
	return &gen
}

func (j *Job) enqueue(info JobInfo) {
	keyHash := info.toKey()

	j.dataStore.add(keyHash, info)
	j.queue.Add(keyHash)
	j.log.V(4).Info("job added to the queue", "keyhash", keyHash)
}

//Add queues a job creation request
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
		logger.Error(err, "failed to sync queued jobs", "key", key)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		j.queue.AddRateLimited(key)
		return
	}

	j.queue.Forget(key)
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
	defer j.queue.Done(obj)

	var keyHash string
	var ok bool
	if keyHash, ok = obj.(string); !ok {
		j.queue.Forget(obj)
		logger.Info("incorrect type; expecting type 'string'", "obj", obj)
		return true
	}

	// lookup data store
	info := j.dataStore.lookup(keyHash)
	err := j.syncHandler(info)
	j.handleErr(err, keyHash)

	return true
}

func (j *Job) syncHandler(info JobInfo) error {
	defer func() {
		j.mux.Unlock()
	}()
	j.mux.Lock()

	var err error
	var wg sync.WaitGroup
	if info.JobType == constant.BackgroundPolicySync {
		wg.Add(1)
		go func() {
			err = j.syncKyverno(&wg, constant.All, constant.BackgroundPolicySync, info.JobData)
		}()
	}

	if info.JobType == constant.ConfigmapMode {
		// shuting?
		if info.JobData == "" {
			return nil
		}

		scopes := strings.Split(info.JobData, ",")
		if len(scopes) == 1 {
			wg.Add(1)
			go func() {
				err = j.syncKyverno(&wg, constant.All, constant.ConfigmapMode, "")
			}()
		} else {
			wg.Add(len(scopes))
			for _, scope := range scopes {
				go func(scope string) {
					err = j.syncKyverno(&wg, scope, constant.ConfigmapMode, "")
				}(scope)
			}
		}
	}

	wg.Wait()
	return err
}

func (j *Job) syncKyverno(wg *sync.WaitGroup, scope, jobType, data string) error {
	defer wg.Done()

	go j.cleanupCompletedJobs()

	mode := "cli"
	args := []string{
		"report",
		"all",
		fmt.Sprintf("--mode=%s", "configmap"),
	}

	if jobType == constant.BackgroundPolicySync || jobType == constant.BackgroundSync {
		switch scope {
		case constant.Namespace:
			args = []string{
				"report",
				"namespace",
				fmt.Sprintf("--mode=%s", mode),
			}
		case constant.Cluster:
			args = []string{
				"report",
				"cluster",
				fmt.Sprintf("--mode=%s", mode),
			}
		case constant.All:
			args = []string{
				"report",
				"all",
				fmt.Sprintf("--mode=%s", mode),
			}
		}
	}

	if jobType == constant.BackgroundPolicySync && data != "" {
		args = append(args, fmt.Sprintf("-p=%s", data))
	}

	exbackoff := &backoff.ExponentialBackOff{
		InitialInterval:     backoff.DefaultInitialInterval,
		RandomizationFactor: backoff.DefaultRandomizationFactor,
		Multiplier:          backoff.DefaultMultiplier,
		MaxInterval:         time.Second,
		MaxElapsedTime:      5 * time.Minute,
		Clock:               backoff.SystemClock,
	}

	exbackoff.Reset()
	err := backoff.Retry(func() error {
		resourceList, err := j.dclient.ListResource("", "Job", config.KubePolicyNamespace, &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"scope": scope,
				"type":  jobType,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to list jobs: %v", err)
		}

		if len(resourceList.Items) != 0 {
			return fmt.Errorf("found %d Jobs", len(resourceList.Items))
		}
		return nil
	}, exbackoff)

	if err != nil {
		return err
	}

	return j.CreateJob(args, jobType, scope)
}

// CreateJob will create Job template for background scan
func (j *Job) CreateJob(args []string, jobType, scope string) error {
	ttl := new(int32)
	*ttl = 60

	job := &v1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.KubePolicyNamespace,
			Labels: map[string]string{
				"scope": scope,
				"type":  jobType,
			},
		},
		Spec: v1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:            strings.ToLower(fmt.Sprintf("%s-%s", jobType, scope)),
							Image:           config.KyvernoCliImage,
							ImagePullPolicy: apiv1.PullAlways,
							Args:            args,
						},
					},
					ServiceAccountName: "kyverno-service-account",
					RestartPolicy:      "OnFailure",
				},
			},
		},
	}

	job.Spec.TTLSecondsAfterFinished = ttl
	job.SetGenerateName("kyverno-policyreport-")
	if _, err := j.dclient.CreateResource("", "Job", config.KubePolicyNamespace, job, false); err != nil {
		return fmt.Errorf("failed to create job: %v", err)
	}

	return nil
}

func (j *Job) cleanupCompletedJobs() {
	logger := j.log.WithName("cleanup jobs")

	exbackoff := &backoff.ExponentialBackOff{
		InitialInterval:     backoff.DefaultInitialInterval,
		RandomizationFactor: backoff.DefaultRandomizationFactor,
		Multiplier:          backoff.DefaultMultiplier,
		MaxInterval:         time.Second,
		MaxElapsedTime:      2 * time.Minute,
		Clock:               backoff.SystemClock,
	}

	exbackoff.Reset()
	err := backoff.Retry(func() error {
		resourceList, err := j.dclient.ListResource("", "Job", config.KubePolicyNamespace, nil)
		if err != nil {
			return fmt.Errorf("failed to list jobs : %v", err)
		}

		if err != nil {
			return fmt.Errorf("failed to list pods : %v", err)
		}

		for _, job := range resourceList.Items {
			succeeded, ok, _ := unstructured.NestedInt64(job.Object, "status", "succeeded")
			if ok && succeeded > 0 {
				if errnew := j.dclient.DeleteResource("", "Job", job.GetNamespace(), job.GetName(), false); errnew != nil {
					err = errnew
					continue
				}

				podList, errNew := j.dclient.ListResource("", "Pod", config.KubePolicyNamespace, &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"job-name": job.GetName(),
					},
				})

				if errNew != nil {
					err = errNew
					continue
				}

				for _, pod := range podList.Items {
					if errpod := j.dclient.DeleteResource("", "Pod", pod.GetNamespace(), pod.GetName(), false); errpod != nil {
						logger.Error(errpod, "failed to delete pod", "name", pod.GetName())
						err = errpod
					}
				}
			}
		}
		return err
	}, exbackoff)

	if err != nil {
		logger.Error(err, "failed to clean up completed jobs")
	}
}
