package policyreport

import (
	"errors"
	"fmt"
	policyreportv1alpha12 "github.com/nirmata/kyverno/pkg/api/policyreport/v1alpha1"
	"github.com/nirmata/kyverno/pkg/constant"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"reflect"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	policyreportv1alpha1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha1"
	policyreportlister "github.com/nirmata/kyverno/pkg/client/listers/policyreport/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/policystatus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const clusterWorkQueueName = "policy-report-cluster"
const clusterWorkQueueRetryLimit = 3

//clusterPR ...
type clusterPR struct {
	// dynamic client
	dclient *client.Client
	// get/list cluster policy report
	cprLister policyreportlister.ClusterPolicyReportLister
	// policy violation interface
	policyreportInterface policyreportv1alpha1.PolicyV1alpha1Interface
	// logger
	log logr.Logger
	// update policy stats with violationCount
	policyStatusListener policystatus.Listener

	dataStore *dataStore

	queue workqueue.RateLimitingInterface
}

func newClusterPR(log logr.Logger, dclient *client.Client,
	cprLister policyreportlister.ClusterPolicyReportLister,
	policyreportInterface policyreportv1alpha1.PolicyV1alpha1Interface,
	policyStatus policystatus.Listener,
) *clusterPR {
	cpv := clusterPR{
		dclient:               dclient,
		cprLister:             cprLister,
		policyreportInterface: policyreportInterface,
		log:                   log,
		policyStatusListener:  policyStatus,

		dataStore: newDataStore(),
		queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), clusterWorkQueueName),
	}
	return &cpv
}

func (cpr *clusterPR) enqueue(info Info) {
	// add to data map
	keyHash := info.toKey()
	// add to
	// queue the key hash
	cpr.dataStore.add(keyHash, info)
	cpr.queue.Add(keyHash)
}

//Add queues a policy violation create request
func (cpr *clusterPR) Add(infos ...Info) {
	for _, info := range infos {
		cpr.enqueue(info)
	}
}

// Run starts the workers
func (cpr *clusterPR) Run(workers int, stopCh <-chan struct{}) {
	logger := cpr.log
	defer utilruntime.HandleCrash()
	logger.Info("start")
	defer logger.Info("shutting down")

	for i := 0; i < workers; i++ {
		go wait.Until(cpr.runWorker, constant.PolicyViolationControllerResync, stopCh)
	}
	<-stopCh
}

func (cpr *clusterPR) runWorker() {
	for cpr.processNextWorkItem() {
	}
}

func (cpr *clusterPR) handleErr(err error, key interface{}) {
	logger := cpr.log
	if err == nil {
		cpr.queue.Forget(key)
		return
	}

	// retires requests if there is error
	if cpr.queue.NumRequeues(key) < clusterWorkQueueRetryLimit {
		logger.Error(err, "failed to sync policy violation", "key", key)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		cpr.queue.AddRateLimited(key)
		return
	}
	cpr.queue.Forget(key)
	// remove from data store
	if keyHash, ok := key.(string); ok {
		cpr.dataStore.delete(keyHash)
	}
	logger.Error(err, "dropping key out of the queue", "key", key)
}

func (cpr *clusterPR) processNextWorkItem() bool {
	logger := cpr.log
	obj, shutdown := cpr.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer cpr.queue.Done(obj)
		var keyHash string
		var ok bool

		if keyHash, ok = obj.(string); !ok {
			cpr.queue.Forget(obj)
			logger.Info("incorrect type; expecting type 'string'", "obj", obj)
			return nil
		}

		// lookup data store
		info := cpr.dataStore.lookup(keyHash)
		if reflect.DeepEqual(info, Info{}) {
			// empty key
			cpr.queue.Forget(obj)
			logger.Info("empty key")
			return nil
		}

		err := cpr.syncHandler(info)
		cpr.handleErr(err, obj)
		return nil
	}(obj)

	if err != nil {
		logger.Error(err, "failed to process item")
		return true
	}

	return true
}

func (cpr *clusterPR) syncHandler(info Info) error {
	logger := cpr.log
	failure := false
	builder := newPrBuilder()

	pv := builder.generate(info)

	if info.FromSync {
		pv.Annotations = map[string]string{
			"fromSync": "true",
		}
	}

	// Create Policy Violations
	logger.V(4).Info("creating policy violation", "key", info.toKey())
	if err := cpr.create(pv, ""); err != nil {
		failure = true
		logger.Error(err, "failed to create policy violation")
	}

	if failure {
		// even if there is a single failure we requeue the request
		return errors.New("Failed to process some policy violations, re-queuing")
	}
	return nil
}

func (cpr *clusterPR) create(pv kyverno.PolicyViolationTemplate, appName string) error {
	reportName := fmt.Sprintf("kyverno-clusterpolicyreport-%s", pv.Spec.Policy)
	clusterpr, err := cpr.policyreportInterface.ClusterPolicyReports().Get(reportName, v1.GetOptions{})
	if err != nil {
		if !k8serror.IsNotFound(err) {
			return err
		}
		clusterpr = &policyreportv1alpha12.ClusterPolicyReport{
			Scope: &corev1.ObjectReference{
				Kind: "Cluster",
			},
			Summary: policyreportv1alpha12.PolicyReportSummary{},
			Results: []*policyreportv1alpha12.PolicyReportResult{},
		}
		labelMap := map[string]string{
			"policy-scope": "cluster",
		}
		clusterpr.SetLabels(labelMap)
		clusterpr.ObjectMeta.Name = reportName
		prObj := NewPolicyReport(nil, clusterpr, &pv, cpr.dclient)
		clusterpr := prObj.CreateClusterPolicyViolationsToClusterPolicyReport()

		_, err = cpr.policyreportInterface.ClusterPolicyReports().Create(clusterpr)
		if err != nil {
			return err
		}
		return nil
	}
	prObj := NewPolicyReport(nil, clusterpr, &pv, cpr.dclient)
	clusterpr = prObj.CreateClusterPolicyViolationsToClusterPolicyReport()

	_, err = cpr.policyreportInterface.ClusterPolicyReports().Update(clusterpr)
	if err != nil {
		return err
	}
	return nil
}
