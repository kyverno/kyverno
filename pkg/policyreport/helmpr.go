package policyreport

import (
	"errors"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	corev1 "k8s.io/api/core/v1"
	"fmt"
	policyreportv1alpha12 "github.com/nirmata/kyverno/pkg/api/policyreport/v1alpha1"
	"github.com/nirmata/kyverno/pkg/constant"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
)

const helmWorkQueueName = "policy-report-helm"
const helmWorkQueueRetryLimit = 3

//helmPR ...
type helmPR struct {
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

func newHelmPR(log logr.Logger, dclient *client.Client,
	nsprLister policyreportlister.PolicyReportLister,
	policyreportInterface policyreportv1alpha1.PolicyV1alpha1Interface,
	policyStatus policystatus.Listener,
) *helmPR {
	nspr := helmPR{
		dclient:               dclient,
		nsprLister:            nsprLister,
		policyreportInterface: policyreportInterface,
		log:                   log,
		policyStatusListener:  policyStatus,
		dataStore:             newDataStore(),
		queue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), helmWorkQueueName),
	}
	return &nspr
}

func (hpr *helmPR) enqueue(info Info) {
	// add to data map
	keyHash := info.toKey()
	// add to
	// queue the key hash
	hpr.dataStore.add(keyHash, info)
	hpr.queue.Add(keyHash)
}

//Add queues a policy violation create request
func (hpr *helmPR) Add(infos ...Info) {
	for _, info := range infos {
		hpr.enqueue(info)
	}
}

// Run starts the workers
func (hpr *helmPR) Run(workers int, stopCh <-chan struct{}) {
	logger := hpr.log
	defer utilruntime.HandleCrash()
	logger.Info("start")
	defer logger.Info("shutting down")

	for i := 0; i < workers; i++ {
		go wait.Until(hpr.runWorker, constant.PolicyViolationControllerResync, stopCh)
	}
	<-stopCh
}

func (hpr *helmPR) runWorker() {
	for hpr.processNextWorkItem() {
	}
}

func (hpr *helmPR) handleErr(err error, key interface{}) {
	logger := hpr.log
	if err == nil {
		hpr.queue.Forget(key)
		return
	}

	// retires requests if there is error
	if hpr.queue.NumRequeues(key) < helmWorkQueueRetryLimit {
		logger.Error(err, "failed to sync policy violation", "key", key)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		hpr.queue.AddRateLimited(key)
		return
	}
	hpr.queue.Forget(key)
	// remove from data store
	if keyHash, ok := key.(string); ok {
		hpr.dataStore.delete(keyHash)
	}
	logger.Error(err, "dropping key out of the queue", "key", key)
}

func (hpr *helmPR) processNextWorkItem() bool {
	logger := hpr.log
	obj, shutdown := hpr.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer hpr.queue.Done(obj)
		var keyHash string
		var ok bool

		if keyHash, ok = obj.(string); !ok {
			hpr.queue.Forget(obj)
			logger.Info("incorrect type; expecting type 'string'", "obj", obj)
			return nil
		}

		// lookup data store
		info := hpr.dataStore.lookup(keyHash)
		if reflect.DeepEqual(info, Info{}) {
			// empty key
			hpr.queue.Forget(obj)
			logger.Info("empty key")
			return nil
		}

		err := hpr.syncHandler(info)
		hpr.handleErr(err, obj)
		return nil
	}(obj)

	if err != nil {
		logger.Error(err, "failed to process item")
		return true
	}

	return true
}

func (hpr *helmPR) syncHandler(info Info) error {
	logger := hpr.log
	failure := false
	builder := newPrBuilder()

	pv := builder.generate(info)

	resource, err := hpr.dclient.GetResource(info.Resource.GetAPIVersion(), info.Resource.GetKind(), info.Resource.GetName(), info.Resource.GetNamespace())
	if err != nil {
		logger.Error(err, "failed to get resource")
	}
	labels := resource.GetLabels()
	// Create Policy Violations
	logger.V(4).Info("creating policy violation", "key", info.toKey())
	if err := hpr.create(pv,labels["helm.sh/chart"]); err != nil {
		failure = true
		logger.Error(err, "failed to create policy violation")
	}

	if failure {
		// even if there is a single failure we requeue the request
		return errors.New("Failed to process some policy violations, re-queuing")
	}
	return nil
}

func (hpr *helmPR) create(pv kyverno.PolicyViolationTemplate,appName string) error {
	reportName := fmt.Sprintf("kyverno-policyreport-%s",appName)
	pr, err := hpr.policyreportInterface.PolicyReports(pv.Spec.Namespace).Get(reportName, v1.GetOptions{})
	if err != nil {
		if !k8serror.IsNotFound(err) {
			return err
		}
		pr = &policyreportv1alpha12.PolicyReport{
			Scope:  &corev1.ObjectReference{
				Kind : "Namespace",
				Namespace: pv.Spec.Namespace,
			},
			Summary: policyreportv1alpha12.PolicyReportSummary{

			},
			Results: []*policyreportv1alpha12.PolicyReportResult{},
		}
		labelMap := map[string]string{
			"policy-scope": "application",
			"helm.sh/chart" : appName,
		}
		pv.SetLabels(labelMap)
		pr.ObjectMeta.Name = reportName
		pr = CreatePolicyViolationToPolicyReport(&pv, pr)
		_, err = hpr.policyreportInterface.PolicyReports(pv.Spec.Namespace).Create(pr)
		if err != nil {
			return err
		}
		return nil
	}
	pr = CreatePolicyViolationToPolicyReport(&pv, pr)
	_, err = hpr.policyreportInterface.PolicyReports(pv.Spec.Namespace).Update(pr)
	if err != nil {
		return err
	}
	return nil
}
