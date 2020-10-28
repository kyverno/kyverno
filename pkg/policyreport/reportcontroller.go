package policyreport

import (
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	policyreportinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policyreport/v1alpha1"
	policyreport "github.com/kyverno/kyverno/pkg/client/listers/policyreport/v1alpha1"
	"github.com/kyverno/kyverno/pkg/constant"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	labels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	prWorkQueueName     = "policy-report-controller"
	clusterpolicyreport = "clusterpolicyreport"
)

// ReportGenerator creates policy report
type ReportGenerator struct {
	dclient *dclient.Client

	// reportInterface reportrequest.PolicyV1alpha1Interface

	reportLister policyreport.PolicyReportLister
	reportSynced cache.InformerSynced

	clusterReportLister policyreport.ClusterPolicyReportLister
	clusterReportSynced cache.InformerSynced

	reportRequestLister policyreport.ReportRequestLister
	reportReqSynced     cache.InformerSynced

	clusterReportRequestLister policyreport.ClusterReportRequestLister
	clusterReportReqSynced     cache.InformerSynced

	nsLister       listerv1.NamespaceLister
	nsListerSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface

	log logr.Logger
}

// NewReportGenerator returns a new instance of policy report generator
func NewReportGenerator(
	dclient *dclient.Client,
	clusterReportInformer policyreportinformer.ClusterPolicyReportInformer,
	reportInformer policyreportinformer.PolicyReportInformer,
	reportReqInformer policyreportinformer.ReportRequestInformer,
	clusterReportReqInformer policyreportinformer.ClusterReportRequestInformer,
	namespace informers.NamespaceInformer,
	log logr.Logger) *ReportGenerator {

	gen := &ReportGenerator{
		dclient: dclient,
		queue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), prWorkQueueName),
		log:     log,
	}

	reportReqInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    gen.addReportRequest,
			UpdateFunc: gen.updateReportRequest,
		})

	clusterReportReqInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    gen.addClusterReportRequest,
			UpdateFunc: gen.updateClusterReportRequest,
		})

	gen.clusterReportLister = clusterReportInformer.Lister()
	gen.clusterReportSynced = clusterReportInformer.Informer().HasSynced
	gen.reportLister = reportInformer.Lister()
	gen.reportSynced = reportInformer.Informer().HasSynced
	gen.clusterReportRequestLister = clusterReportReqInformer.Lister()
	gen.clusterReportReqSynced = clusterReportReqInformer.Informer().HasSynced
	gen.reportRequestLister = reportReqInformer.Lister()
	gen.reportReqSynced = reportReqInformer.Informer().HasSynced
	gen.nsLister = namespace.Lister()
	gen.nsListerSynced = namespace.Informer().HasSynced

	return gen
}

func (g *ReportGenerator) addReportRequest(obj interface{}) {
	r := obj.(*report.ReportRequest)
	ns := r.GetNamespace()
	if ns == "" {
		ns = "default"
	}

	g.queue.Add(ns)
}

func (g *ReportGenerator) updateReportRequest(old interface{}, cur interface{}) {
	oldReq := old.(*report.ReportRequest)
	curReq := cur.(*report.ReportRequest)
	if reflect.DeepEqual(oldReq.Results, curReq.Results) {
		return
	}

	ns := curReq.GetNamespace()
	if ns == "" {
		ns = "default"
	}
	g.queue.Add(ns)
}

func (g *ReportGenerator) addClusterReportRequest(obj interface{}) {
	_ = obj.(*report.ClusterReportRequest)
	g.queue.Add("")
}

func (g *ReportGenerator) updateClusterReportRequest(old interface{}, cur interface{}) {
	oldReq := old.(*report.ClusterReportRequest)
	curReq := cur.(*report.ClusterReportRequest)

	if reflect.DeepEqual(oldReq.Results, curReq.Results) {
		return
	}

	g.queue.Add("")
}

// Run starts the workers
func (g *ReportGenerator) Run(workers int, stopCh <-chan struct{}) {
	logger := g.log
	defer utilruntime.HandleCrash()
	defer g.queue.ShutDown()

	logger.Info("start")
	defer logger.Info("shutting down")

	if !cache.WaitForCacheSync(stopCh, g.reportReqSynced, g.clusterReportReqSynced, g.reportSynced, g.clusterReportSynced, g.nsListerSynced) {
		logger.Info("failed to sync informer cache")
	}

	for i := 0; i < workers; i++ {
		go wait.Until(g.runWorker, constant.PolicyViolationControllerResync, stopCh)
	}

	<-stopCh
}

func (g *ReportGenerator) runWorker() {
	for g.processNextWorkItem() {
	}
}

func (g *ReportGenerator) processNextWorkItem() bool {
	key, shutdown := g.queue.Get()
	if shutdown {
		return false
	}

	defer g.queue.Done(key)
	ns, ok := key.(string)
	if !ok {
		g.queue.Forget(key)
		g.log.Info("incorrect type; expecting type 'string'", "obj", key)
		return true
	}

	err := g.syncHandler(ns)
	g.handleErr(err, key)

	return true
}

func (g *ReportGenerator) handleErr(err error, key interface{}) {
	logger := g.log
	if err == nil {
		g.queue.Forget(key)
		return
	}

	// retires requests if there is error
	if g.queue.NumRequeues(key) < workQueueRetryLimit {
		logger.Error(err, "failed to sync report request", "key", key)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		g.queue.AddRateLimited(key)
		return
	}
	g.queue.Forget(key)
	logger.Error(err, "dropping key out of the queue", "key", key)
}

// syncHandler reconciles clusterPolicyReport if namespace == ""
// otherwise it updates policyrReport
func (g *ReportGenerator) syncHandler(namespace string) error {
	log := g.log.WithName("sync")

	new, aggregatedRequests, err := g.aggregateReports(namespace)
	if err != nil {
		return fmt.Errorf("failed to aggregate reportRequest results %v", err)
	}

	var old interface{}
	if namespace != "" {
		old, err = g.reportLister.PolicyReports(namespace).Get(generatePolicyReportName((namespace)))
		if err != nil {
			if apierrors.IsNotFound(err) && new != nil {
				if _, err := g.dclient.CreateResource(new.GetAPIVersion(), new.GetKind(), new.GetNamespace(), new, false); err != nil {
					return fmt.Errorf("failed to create policyReport: %v", err)
				}

				log.V(2).Info("successfully created policyReport", "namespace", new.GetNamespace(), "name", new.GetName())
				g.cleanupReportRequets(aggregatedRequests)
				return nil
			}

			return fmt.Errorf("unable to get policyReport: %v", err)
		}
	} else {
		old, err = g.clusterReportLister.Get(generatePolicyReportName((namespace)))
		if err != nil {
			if apierrors.IsNotFound(err) && new != nil {
				if _, err := g.dclient.CreateResource(new.GetAPIVersion(), new.GetKind(), new.GetNamespace(), new, false); err != nil {
					return fmt.Errorf("failed to create ClusterPolicyReport: %v", err)
				}

				log.V(2).Info("successfully created ClusterPolicyReport")
				g.cleanupReportRequets(aggregatedRequests)
				return nil
			}

			return fmt.Errorf("unable to get ClusterPolicyReport: %v", err)
		}
	}

	if err := g.updateReport(old, new); err != nil {
		return err
	}

	g.cleanupReportRequets(aggregatedRequests)
	return nil
}

func (g *ReportGenerator) aggregateReports(namespace string) (
	report *unstructured.Unstructured, aggregatedRequests interface{}, err error) {

	if namespace == "" {
		requests, err := g.clusterReportLister.List(labels.Everything())
		if err != nil {
			return nil, nil, fmt.Errorf("unable to list ClusterReportRequests within: %v", err)
		}

		if report, aggregatedRequests, err = mergeRequests(nil, requests); err != nil {
			return nil, nil, fmt.Errorf("unable to merge ClusterReportRequests results: %v", err)
		}
	} else {
		ns, err := g.nsLister.Get(namespace)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to get namespace %s: %v", ns.GetName(), err)
		}

		requests, err := g.reportRequestLister.ReportRequests(ns.GetName()).List(labels.Everything())
		if err != nil {
			return nil, nil, fmt.Errorf("unable to list reportRequests within namespace %s: %v", ns, err)
		}

		if report, aggregatedRequests, err = mergeRequests(ns, requests); err != nil {
			return nil, nil, fmt.Errorf("unable to merge results: %v", err)
		}
	}

	return report, aggregatedRequests, nil
}

func mergeRequests(ns *v1.Namespace, requestsGeneral interface{}) (*unstructured.Unstructured, interface{}, error) {
	results := []*report.PolicyReportResult{}

	if requests, ok := requestsGeneral.([]*report.ClusterReportRequest); ok {
		aggregatedRequests := []*report.ClusterReportRequest{}
		for _, request := range requests {
			if request.GetDeletionTimestamp() != nil {
				continue
			}
			results = append(results, request.Results...)
			aggregatedRequests = append(aggregatedRequests, request)
		}

		report := &report.ClusterPolicyReport{
			Results: results,
			Summary: calculateSummary(results),
		}

		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(report)
		if err != nil {
			return nil, aggregatedRequests, err
		}

		req := &unstructured.Unstructured{Object: obj}
		setReport(req, nil)
		return req, aggregatedRequests, nil
	}

	if requests, ok := requestsGeneral.([]*report.ReportRequest); ok {
		aggregatedRequests := []*report.ReportRequest{}
		for _, request := range requests {
			if request.GetDeletionTimestamp() != nil {
				continue
			}
			results = append(results, request.Results...)
			aggregatedRequests = append(aggregatedRequests, request)
		}

		report := &report.PolicyReport{
			Results: results,
			Summary: calculateSummary(results),
		}

		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(report)
		if err != nil {
			return nil, aggregatedRequests, err
		}

		req := &unstructured.Unstructured{Object: obj}
		setReport(req, ns)
		return req, aggregatedRequests, nil
	}

	return nil, nil, nil
}

func setReport(report *unstructured.Unstructured, ns *v1.Namespace) {
	report.SetAPIVersion("policy.kubernetes.io/v1alpha1")

	if ns == nil {
		report.SetName(generatePolicyReportName(""))
		report.SetKind("ClusterPolicyReport")
		return
	}

	report.SetName(generatePolicyReportName(ns.GetName()))
	report.SetNamespace(ns.GetName())
	report.SetKind("PolicyReport")

	controllerFlag := true
	blockOwnerDeletionFlag := true

	report.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion:         "v1",
			Kind:               "Namespace",
			Name:               ns.GetName(),
			UID:                ns.GetUID(),
			Controller:         &controllerFlag,
			BlockOwnerDeletion: &blockOwnerDeletionFlag,
		},
	})
}

func (g *ReportGenerator) updateReport(old interface{}, new *unstructured.Unstructured) (err error) {
	if new == nil {
		g.log.V(4).Info("empty report to update")
		return nil
	}

	oldUnstructed := make(map[string]interface{})

	if oldTyped, ok := old.(*report.ClusterPolicyReport); ok {
		if oldTyped.GetDeletionTimestamp() != nil {
			return g.dclient.DeleteResource(oldTyped.APIVersion, "ClusterPolicyReport", oldTyped.Namespace, oldTyped.Name, false)
		}

		if oldUnstructed, err = runtime.DefaultUnstructuredConverter.ToUnstructured(oldTyped); err != nil {
			return fmt.Errorf("unable to convert clusterPolicyReport: %v", err)
		}
		new.SetUID(oldTyped.GetUID())
		new.SetResourceVersion(oldTyped.GetResourceVersion())
	} else if oldTyped, ok := old.(*report.PolicyReport); ok {
		if oldTyped.GetDeletionTimestamp() != nil {
			return g.dclient.DeleteResource(oldTyped.APIVersion, "PolicyReport", oldTyped.Namespace, oldTyped.Name, false)
		}

		if oldUnstructed, err = runtime.DefaultUnstructuredConverter.ToUnstructured(oldTyped); err != nil {
			return fmt.Errorf("unable to convert policyReport: %v", err)
		}

		new.SetUID(oldTyped.GetUID())
		new.SetResourceVersion(oldTyped.GetResourceVersion())
	}

	obj, err := updateResults(oldUnstructed, new.UnstructuredContent())
	if err != nil {
		return fmt.Errorf("failed to update results entry: %v", err)
	}
	new.Object = obj

	if !hasResultsChanged(oldUnstructed, new.UnstructuredContent()) {
		g.log.V(4).Info("unchanged policy report", "namespace", new.GetNamespace(), "name", new.GetName())
		return nil
	}

	if _, err = g.dclient.UpdateResource(new.GetAPIVersion(), new.GetKind(), new.GetNamespace(), new, false); err != nil {
		return fmt.Errorf("failed to update policy report: %v", err)
	}

	g.log.V(3).Info("successfully updated policy report", "kind", new.GetKind(), "namespace", new.GetNamespace(), "name", new.GetName())
	return
}

func (g *ReportGenerator) cleanupReportRequets(requestsGeneral interface{}) {
	defer g.log.V(2).Info("successfully cleaned up report requests ")
	if requests, ok := requestsGeneral.([]*report.ReportRequest); ok {
		for _, request := range requests {
			if err := g.dclient.DeleteResource(request.APIVersion, "ReportRequest", request.Namespace, request.Name, false); err != nil {
				g.log.Error(err, "failed to delete report request")
			}
		}
	}

	if requests, ok := requestsGeneral.([]*report.ClusterReportRequest); ok {
		for _, request := range requests {
			if err := g.dclient.DeleteResource(request.APIVersion, "ClusterReportRequest", request.Namespace, request.Name, false); err != nil {
				g.log.Error(err, "failed to delete clusterReportRequest")
			}
		}
	}
}
