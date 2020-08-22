package policyreport

import (
	"errors"
	"fmt"
	policyreportv1alpha12 "github.com/nirmata/kyverno/pkg/api/policyreport/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"reflect"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	policyreportv1alpha1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha1"
	policyreportlister "github.com/nirmata/kyverno/pkg/client/listers/policyreport/v1alpha1"
	"github.com/nirmata/kyverno/pkg/constant"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/policystatus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

const nsWorkQueueName = "policy-report-namespace"
const nsWorkQueueRetryLimit = 3

//namespacedPR ...
type namespacedPR struct {
	// dynamic client
	dclient *client.Client
	// get/list namespaced policy violation
	nsprLister policyreportlister.PolicyReportLister
	// policy violation interface
	policyreportInterface policyreportv1alpha1.PolicyV1alpha1Interface
	// logger
	log logr.Logger
	// update policy status with violationCount
	policyStatusListener policystatus.Listener

	dataStore *dataStore

	queue workqueue.RateLimitingInterface
}

func newNamespacedPR(log logr.Logger, dclient *client.Client,
	nsprLister policyreportlister.PolicyReportLister,
	policyreportInterface policyreportv1alpha1.PolicyV1alpha1Interface,
	policyStatus policystatus.Listener,
) *namespacedPR {
	nspr := namespacedPR{
		dclient:               dclient,
		nsprLister:            nsprLister,
		policyreportInterface: policyreportInterface,
		log:                   log,
		policyStatusListener:  policyStatus,
		dataStore:             newDataStore(),
		queue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), nsWorkQueueName),
	}
	return &nspr
}

func (nspr *namespacedPR) enqueue(info Info) {
	// add to data map
	keyHash := info.toKey()
	// add to
	// queue the key hash
	nspr.dataStore.add(keyHash, info)
	nspr.queue.Add(keyHash)
}

//Add queues a policy violation create request
func (nspr *namespacedPR) Add(infos ...Info) {
	for _, info := range infos {
		nspr.enqueue(info)
	}
}

// Run starts the workers
func (nspr *namespacedPR) Run(workers int, stopCh <-chan struct{}) {
	logger := nspr.log
	defer utilruntime.HandleCrash()
	logger.Info("start")
	defer logger.Info("shutting down")

	for i := 0; i < workers; i++ {
		go wait.Until(nspr.runWorker, constant.PolicyViolationControllerResync, stopCh)
	}
	<-stopCh
}

func (nspr *namespacedPR) runWorker() {
	for nspr.processNextWorkItem() {
	}
}

func (nspr *namespacedPR) handleErr(err error, key interface{}) {
	logger := nspr.log
	if err == nil {
		nspr.queue.Forget(key)
		return
	}

	// retires requests if there is error
	if nspr.queue.NumRequeues(key) < nsWorkQueueRetryLimit {
		logger.Error(err, "failed to sync policy violation", "key", key)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		nspr.queue.AddRateLimited(key)
		return
	}
	nspr.queue.Forget(key)
	// remove from data store
	if keyHash, ok := key.(string); ok {
		nspr.dataStore.delete(keyHash)
	}
	logger.Error(err, "dropping key out of the queue", "key", key)
}

func (nspr *namespacedPR) processNextWorkItem() bool {
	logger := nspr.log
	obj, shutdown := nspr.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer nspr.queue.Done(obj)
		var keyHash string
		var ok bool

		if keyHash, ok = obj.(string); !ok {
			nspr.queue.Forget(obj)
			logger.Info("incorrect type; expecting type 'string'", "obj", obj)
			return nil
		}

		// lookup data store
		info := nspr.dataStore.lookup(keyHash)
		if reflect.DeepEqual(info, Info{}) {
			// empty key
			nspr.queue.Forget(obj)
			logger.Info("empty key")
			return nil
		}

		err := nspr.syncHandler(info)
		nspr.handleErr(err, obj)
		return nil
	}(obj)

	if err != nil {
		logger.Error(err, "failed to process item")
		return true
	}

	return true
}

func (nspr *namespacedPR) syncHandler(info Info) error {
	logger := nspr.log
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
	if err := nspr.create(pv, ""); err != nil {
		failure = true
		logger.Error(err, "failed to create policy violation")
	}

	if failure {
		// even if there is a single failure we requeue the request
		return errors.New("Failed to process some policy violations, re-queuing")
	}
	return nil
}

func (nspr *namespacedPR) create(pv kyverno.PolicyViolationTemplate, appName string) error {
	reportName := fmt.Sprintf("kyverno-policyreport-%s-%s", appName,pv.Spec.Policy)
	pr, err := nspr.policyreportInterface.PolicyReports(pv.Spec.Namespace).Get(reportName, v1.GetOptions{})
	if err != nil {
		if !k8serror.IsNotFound(err) {
			return err
		}
		pr = &policyreportv1alpha12.PolicyReport{
			Scope: &corev1.ObjectReference{
				Kind:      "Namespace",
				Namespace: pv.Spec.Namespace,
			},
			Summary: policyreportv1alpha12.PolicyReportSummary{},
			Results: []*policyreportv1alpha12.PolicyReportResult{},
		}
		labelMap := map[string]string{
			"policy-scope": "namespace",
			"policy" : pv.Spec.Policy,
		}
		pr.SetLabels(labelMap)
		pr.ObjectMeta.Name = reportName
		prObj := NewPolicyReport(pr, nil, &pv,nspr.dclient)
		cpr := prObj.CreatePolicyViolationToPolicyReport()
		_, err = nspr.policyreportInterface.PolicyReports(pv.Spec.Namespace).Create(cpr)
		if err != nil {
			return err
		}
		return nil
	}
	prObj := NewPolicyReport(pr, nil, &pv,nspr.dclient)
	cpr := prObj.CreatePolicyViolationToPolicyReport()
	cpr, err = nspr.policyreportInterface.PolicyReports(pv.Spec.Namespace).Update(cpr)
	if err != nil {
		return err
	}
	return nil
}
