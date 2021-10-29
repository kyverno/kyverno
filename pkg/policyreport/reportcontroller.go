package policyreport

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	changerequest "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	report "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	requestinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha2"
	policyreportinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policyreport/v1alpha2"
	requestlister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	policyreport "github.com/kyverno/kyverno/pkg/client/listers/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/version"
)

const (
	prWorkQueueName     = "policy-report-controller"
	clusterpolicyreport = "clusterpolicyreport"
)

// ReportGenerator creates policy report
type ReportGenerator struct {
	pclient *kyvernoclient.Clientset
	dclient *dclient.Client

	clusterReportInformer    policyreportinformer.ClusterPolicyReportInformer
	reportInformer           policyreportinformer.PolicyReportInformer
	reportReqInformer        requestinformer.ReportChangeRequestInformer
	clusterReportReqInformer requestinformer.ClusterReportChangeRequestInformer

	reportLister policyreport.PolicyReportLister
	reportSynced cache.InformerSynced

	clusterReportLister policyreport.ClusterPolicyReportLister
	clusterReportSynced cache.InformerSynced

	reportChangeRequestLister requestlister.ReportChangeRequestLister
	reportReqSynced           cache.InformerSynced

	clusterReportChangeRequestLister requestlister.ClusterReportChangeRequestLister
	clusterReportReqSynced           cache.InformerSynced

	nsLister       listerv1.NamespaceLister
	nsListerSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface

	// ReconcileCh sends a signal to policy controller to force the reconciliation of policy report
	// if send true, the reports' results will be erased, this is used to recover from the invalid records
	ReconcileCh chan bool

	log logr.Logger
}

// NewReportGenerator returns a new instance of policy report generator
func NewReportGenerator(
	kubeClient kubernetes.Interface,
	pclient *kyvernoclient.Clientset,
	dclient *dclient.Client,
	clusterReportInformer policyreportinformer.ClusterPolicyReportInformer,
	reportInformer policyreportinformer.PolicyReportInformer,
	reportReqInformer requestinformer.ReportChangeRequestInformer,
	clusterReportReqInformer requestinformer.ClusterReportChangeRequestInformer,
	namespace informers.NamespaceInformer,
	log logr.Logger) (*ReportGenerator, error) {

	gen := &ReportGenerator{
		pclient:                  pclient,
		dclient:                  dclient,
		clusterReportInformer:    clusterReportInformer,
		reportInformer:           reportInformer,
		reportReqInformer:        reportReqInformer,
		clusterReportReqInformer: clusterReportReqInformer,
		queue:                    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), prWorkQueueName),
		ReconcileCh:              make(chan bool, 10),
		log:                      log,
	}

	gen.clusterReportLister = clusterReportInformer.Lister()
	gen.clusterReportSynced = clusterReportInformer.Informer().HasSynced
	gen.reportLister = reportInformer.Lister()
	gen.reportSynced = reportInformer.Informer().HasSynced
	gen.clusterReportChangeRequestLister = clusterReportReqInformer.Lister()
	gen.clusterReportReqSynced = clusterReportReqInformer.Informer().HasSynced
	gen.reportChangeRequestLister = reportReqInformer.Lister()
	gen.reportReqSynced = reportReqInformer.Informer().HasSynced
	gen.nsLister = namespace.Lister()
	gen.nsListerSynced = namespace.Informer().HasSynced

	return gen, nil
}

const deletedPolicyKey string = "deletedpolicy"

// the key of queue can be
// - <namespace name> for the resource
// - "" for cluster wide resource
// - "deletedpolicy/policyName/ruleName(optional)" for a deleted policy or rule
func generateCacheKey(changeRequest interface{}) string {
	if request, ok := changeRequest.(*changerequest.ReportChangeRequest); ok {
		label := request.GetLabels()
		policy := label[deletedLabelPolicy]
		rule := label[deletedLabelRule]
		if rule != "" || policy != "" {
			return strings.Join([]string{deletedPolicyKey, policy, rule}, "/")
		}

		ns := label[resourceLabelNamespace]
		if ns == "" {
			ns = "default"
		}
		return ns
	} else if request, ok := changeRequest.(*changerequest.ClusterReportChangeRequest); ok {
		label := request.GetLabels()
		policy := label[deletedLabelPolicy]
		rule := label[deletedLabelRule]
		if rule != "" || policy != "" {
			return strings.Join([]string{deletedPolicyKey, policy, rule}, "/")
		}
		return ""
	}

	return ""
}

// managedRequest returns true if the request is managed by
// the current version of Kyverno instance
func managedRequest(changeRequest interface{}) bool {
	labels := make(map[string]string)

	if request, ok := changeRequest.(*changerequest.ReportChangeRequest); ok {
		labels = request.GetLabels()
	} else if request, ok := changeRequest.(*changerequest.ClusterReportChangeRequest); ok {
		labels = request.GetLabels()
	}

	if v, ok := labels[appVersion]; !ok || v != version.BuildVersion {
		return false
	}

	return true
}

func (g *ReportGenerator) addReportChangeRequest(obj interface{}) {
	if !managedRequest(obj) {
		g.cleanupReportRequests([]*changerequest.ReportChangeRequest{obj.(*changerequest.ReportChangeRequest)})
		return
	}

	key := generateCacheKey(obj)
	g.queue.Add(key)
}

func (g *ReportGenerator) updateReportChangeRequest(old interface{}, cur interface{}) {
	oldReq := old.(*changerequest.ReportChangeRequest)
	curReq := cur.(*changerequest.ReportChangeRequest)
	if reflect.DeepEqual(oldReq.Results, curReq.Results) {
		return
	}

	if !managedRequest(curReq) {
		g.cleanupReportRequests([]*changerequest.ReportChangeRequest{curReq})
		return
	}

	key := generateCacheKey(cur)
	g.queue.Add(key)
}

func (g *ReportGenerator) addClusterReportChangeRequest(obj interface{}) {
	if !managedRequest(obj) {
		g.cleanupReportRequests([]*changerequest.ClusterReportChangeRequest{obj.(*changerequest.ClusterReportChangeRequest)})
		return
	}

	key := generateCacheKey(obj)
	g.queue.Add(key)
}

func (g *ReportGenerator) updateClusterReportChangeRequest(old interface{}, cur interface{}) {
	oldReq := old.(*changerequest.ClusterReportChangeRequest)
	curReq := cur.(*changerequest.ClusterReportChangeRequest)

	if reflect.DeepEqual(oldReq.Results, curReq.Results) {
		return
	}

	if !managedRequest(curReq) {
		return
	}

	g.queue.Add("")
}

func (g *ReportGenerator) deletePolicyReport(obj interface{}) {
	report := obj.(*report.PolicyReport)
	g.log.V(2).Info("PolicyReport deleted", "name", report.GetName())
	g.ReconcileCh <- false
}

func (g *ReportGenerator) deleteClusterPolicyReport(obj interface{}) {
	g.log.V(2).Info("ClusterPolicyReport deleted")
	g.ReconcileCh <- false
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

	g.reportReqInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    g.addReportChangeRequest,
			UpdateFunc: g.updateReportChangeRequest,
		})

	g.clusterReportReqInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    g.addClusterReportChangeRequest,
			UpdateFunc: g.updateClusterReportChangeRequest,
		})

	g.reportInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			DeleteFunc: g.deletePolicyReport,
		})

	g.clusterReportInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			DeleteFunc: g.deleteClusterPolicyReport,
		})

	for i := 0; i < workers; i++ {
		go wait.Until(g.runWorker, time.Second, stopCh)
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
	keyStr, ok := key.(string)
	if !ok {
		g.queue.Forget(key)
		g.log.Info("incorrect type; expecting type 'string'", "obj", key)
		return true
	}

	aggregatedRequests, err := g.syncHandler(keyStr)
	g.handleErr(err, key, aggregatedRequests)

	return true
}

func (g *ReportGenerator) handleErr(err error, key interface{}, aggregatedRequests interface{}) {
	logger := g.log
	if err == nil {
		g.queue.Forget(key)
		return
	}

	// retires requests if there is error
	if g.queue.NumRequeues(key) < workQueueRetryLimit {
		logger.V(3).Info("retrying policy report", "key", key, "error", err.Error())
		g.queue.AddRateLimited(key)
		return
	}

	logger.Error(err, "failed to process policy report", "key", key)
	g.queue.Forget(key)

	if aggregatedRequests != nil {
		g.cleanupReportRequests(aggregatedRequests)
	}
}

// syncHandler reconciles clusterPolicyReport if namespace == ""
// otherwise it updates policyReport
func (g *ReportGenerator) syncHandler(key string) (aggregatedRequests interface{}, err error) {
	g.log.V(4).Info("syncing policy report", "key", key)

	if policy, rule, ok := isDeletedPolicyKey(key); ok {
		g.log.V(4).Info("sync policy report on policy deletion")
		return g.removePolicyEntryFromReport(policy, rule)
	}

	namespace := key
	new, aggregatedRequests, err := g.aggregateReports(namespace)
	if err != nil {
		return aggregatedRequests, fmt.Errorf("failed to aggregate reportChangeRequest results %v", err)
	}

	var old interface{}
	if old, err = g.createReportIfNotPresent(namespace, new, aggregatedRequests); err != nil {
		return aggregatedRequests, err
	}

	if old == nil {
		g.log.V(4).Info("no existing policy report is found, clean up related report change requests")
		g.cleanupReportRequests(aggregatedRequests)
		return nil, nil
	}

	if err := g.updateReport(old, new, aggregatedRequests); err != nil {
		return aggregatedRequests, err
	}

	g.cleanupReportRequests(aggregatedRequests)
	return nil, nil
}

// createReportIfNotPresent creates cluster / policyReport if not present
// return the existing report if exist
func (g *ReportGenerator) createReportIfNotPresent(namespace string, new *unstructured.Unstructured, aggregatedRequests interface{}) (report interface{}, err error) {
	log := g.log.WithName("createReportIfNotPresent")
	obj, hasDuplicate, err := updateResults(new.UnstructuredContent(), new.UnstructuredContent(), nil)
	if hasDuplicate && err != nil {
		g.log.Error(err, "failed to remove duplicate results", "policy report", new.GetName())
	} else {
		new.Object = obj
	}

	if namespace != "" {
		ns, err := g.nsLister.Get(namespace)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to fetch namespace: %v", err)
		}

		if ns.GetDeletionTimestamp() != nil {
			return nil, nil
		}

		report, err = g.reportLister.PolicyReports(namespace).Get(generatePolicyReportName(namespace))
		if err != nil {
			if apierrors.IsNotFound(err) && new != nil {
				if _, err := g.dclient.CreateResource(new.GetAPIVersion(), new.GetKind(), new.GetNamespace(), new, false); err != nil {
					return nil, fmt.Errorf("failed to create policyReport: %v", err)
				}

				log.V(2).Info("successfully created policyReport", "namespace", new.GetNamespace(), "name", new.GetName())
				g.cleanupReportRequests(aggregatedRequests)
				return nil, nil
			}

			return nil, fmt.Errorf("unable to get policyReport: %v", err)
		}
	} else {
		report, err = g.clusterReportLister.Get(generatePolicyReportName(namespace))
		if err != nil {
			if apierrors.IsNotFound(err) {
				if new != nil {
					if _, err := g.dclient.CreateResource(new.GetAPIVersion(), new.GetKind(), new.GetNamespace(), new, false); err != nil {
						return nil, fmt.Errorf("failed to create ClusterPolicyReport: %v", err)
					}

					log.V(2).Info("successfully created ClusterPolicyReport")
					g.cleanupReportRequests(aggregatedRequests)
					return nil, nil
				}
				return nil, nil
			}
			return nil, fmt.Errorf("unable to get ClusterPolicyReport: %v", err)
		}
	}
	return report, nil
}

func (g *ReportGenerator) removePolicyEntryFromReport(policyName, ruleName string) (aggregatedRequests interface{}, err error) {
	if err := g.removeFromClusterPolicyReport(policyName, ruleName); err != nil {
		return nil, err
	}

	if err := g.removeFromPolicyReport(policyName, ruleName); err != nil {
		return nil, err
	}

	labelset := labels.Set(map[string]string{deletedLabelPolicy: policyName})
	if ruleName != "" {
		labelset = labels.Set(map[string]string{
			deletedLabelPolicy: policyName,
			deletedLabelRule:   ruleName,
		})
	}
	aggregatedRequests, err = g.reportChangeRequestLister.ReportChangeRequests(config.KyvernoNamespace).List(labels.SelectorFromSet(labelset))
	if err != nil {
		return aggregatedRequests, err
	}

	g.cleanupReportRequests(aggregatedRequests)
	return nil, nil
}

func (g *ReportGenerator) removeFromClusterPolicyReport(policyName, ruleName string) error {
	cpolrs, err := g.clusterReportLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list clusterPolicyReport %v", err)
	}

	for _, cpolr := range cpolrs {
		newRes := []*report.PolicyReportResult{}
		for _, result := range cpolr.Results {
			if ruleName != "" && result.Rule == ruleName && result.Policy == policyName {
				continue
			} else if ruleName == "" && result.Policy == policyName {
				continue
			}
			newRes = append(newRes, result)
		}
		cpolr.Results = newRes
		cpolr.Summary = calculateSummary(newRes)
		gv := report.SchemeGroupVersion
		cpolr.SetGroupVersionKind(schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: "ClusterPolicyReport"})
		if _, err := g.dclient.UpdateResource("", "ClusterPolicyReport", "", cpolr, false); err != nil {
			return fmt.Errorf("failed to update clusterPolicyReport %s %v", cpolr.Name, err)
		}
	}
	return nil
}

func (g *ReportGenerator) removeFromPolicyReport(policyName, ruleName string) error {
	namespaces, err := g.dclient.ListResource("", "Namespace", "", nil)
	if err != nil {
		return fmt.Errorf("unable to list namespace %v", err)
	}

	policyReports := []*report.PolicyReport{}
	for _, ns := range namespaces.Items {
		reports, err := g.reportLister.PolicyReports(ns.GetName()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("unable to list policyReport for namespace %s %v", ns.GetName(), err)
		}
		policyReports = append(policyReports, reports...)
	}

	for _, r := range policyReports {
		newRes := []*report.PolicyReportResult{}
		for _, result := range r.Results {
			if ruleName != "" && result.Rule == ruleName && result.Policy == policyName {
				continue
			} else if ruleName == "" && result.Policy == policyName {
				continue
			}
			newRes = append(newRes, result)
		}

		r.Results = newRes
		r.Summary = calculateSummary(newRes)
		gv := report.SchemeGroupVersion
		gvk := schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: "PolicyReport"}
		r.SetGroupVersionKind(gvk)
		if _, err := g.dclient.UpdateResource("", "PolicyReport", r.GetNamespace(), r, false); err != nil {
			return fmt.Errorf("failed to update PolicyReport %s %v", r.GetName(), err)
		}
	}
	return nil
}

// aggregateReports aggregates cluster / report change requests to a policy report
func (g *ReportGenerator) aggregateReports(namespace string) (
	report *unstructured.Unstructured, aggregatedRequests interface{}, err error) {

	if namespace == "" {
		selector := labels.SelectorFromSet(labels.Set(map[string]string{appVersion: version.BuildVersion}))
		requests, err := g.clusterReportChangeRequestLister.List(selector)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to list ClusterReportChangeRequests within: %v", err)
		}

		if report, aggregatedRequests, err = mergeRequests(nil, requests); err != nil {
			return nil, nil, fmt.Errorf("unable to merge ClusterReportChangeRequests results: %v", err)
		}
	} else {
		ns, err := g.nsLister.Get(namespace)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, nil, fmt.Errorf("unable to get namespace %s: %v", namespace, err)
			}
			// Namespace is deleted, create a fake ns to clean up RCRs
			ns = new(v1.Namespace)
			ns.SetName(namespace)
			now := metav1.Now()
			ns.SetDeletionTimestamp(&now)
		}

		selector := labels.SelectorFromSet(labels.Set(map[string]string{appVersion: version.BuildVersion, resourceLabelNamespace: namespace}))
		requests, err := g.reportChangeRequestLister.ReportChangeRequests(config.KyvernoNamespace).List(selector)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to list reportChangeRequests within namespace %s: %v", ns, err)
		}

		if report, aggregatedRequests, err = mergeRequests(ns, requests); err != nil {
			return nil, nil, fmt.Errorf("unable to merge results: %v", err)
		}
	}

	return report, aggregatedRequests, nil
}

func mergeRequests(ns *v1.Namespace, requestsGeneral interface{}) (*unstructured.Unstructured, interface{}, error) {
	results := []*report.PolicyReportResult{}

	if requests, ok := requestsGeneral.([]*changerequest.ClusterReportChangeRequest); ok {
		aggregatedRequests := []*changerequest.ClusterReportChangeRequest{}
		for _, request := range requests {
			if request.GetDeletionTimestamp() != nil {
				continue
			}
			if len(request.Results) != 0 {
				results = append(results, request.Results...)
			}
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

	if requests, ok := requestsGeneral.([]*changerequest.ReportChangeRequest); ok {
		aggregatedRequests := []*changerequest.ReportChangeRequest{}
		for _, request := range requests {
			if request.GetDeletionTimestamp() != nil {
				continue
			}
			if len(request.Results) != 0 {
				results = append(results, request.Results...)
			}
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

func setReport(reportUnstructured *unstructured.Unstructured, ns *v1.Namespace) {
	reportUnstructured.SetAPIVersion(report.SchemeGroupVersion.String())

	if ns == nil {
		reportUnstructured.SetName(generatePolicyReportName(""))
		reportUnstructured.SetKind("ClusterPolicyReport")
		return
	}

	reportUnstructured.SetName(generatePolicyReportName(ns.GetName()))
	reportUnstructured.SetNamespace(ns.GetName())
	reportUnstructured.SetKind("PolicyReport")

	controllerFlag := true
	blockOwnerDeletionFlag := true

	reportUnstructured.SetOwnerReferences([]metav1.OwnerReference{
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

func (g *ReportGenerator) updateReport(old interface{}, new *unstructured.Unstructured, aggregatedRequests interface{}) (err error) {
	if new == nil {
		g.log.V(4).Info("empty report to update")
		return nil
	}
	g.log.V(4).Info("reconcile policy report")

	oldUnstructured := make(map[string]interface{})

	if oldTyped, ok := old.(*report.ClusterPolicyReport); ok {
		if oldTyped.GetDeletionTimestamp() != nil {
			return g.dclient.DeleteResource(oldTyped.APIVersion, "ClusterPolicyReport", oldTyped.Namespace, oldTyped.Name, false)
		}

		if oldUnstructured, err = runtime.DefaultUnstructuredConverter.ToUnstructured(oldTyped); err != nil {
			return fmt.Errorf("unable to convert clusterPolicyReport: %v", err)
		}
		new.SetUID(oldTyped.GetUID())
		new.SetResourceVersion(oldTyped.GetResourceVersion())
	} else if oldTyped, ok := old.(*report.PolicyReport); ok {
		if oldTyped.GetDeletionTimestamp() != nil {
			return g.dclient.DeleteResource(oldTyped.APIVersion, "PolicyReport", oldTyped.Namespace, oldTyped.Name, false)
		}

		if oldUnstructured, err = runtime.DefaultUnstructuredConverter.ToUnstructured(oldTyped); err != nil {
			return fmt.Errorf("unable to convert policyReport: %v", err)
		}

		new.SetUID(oldTyped.GetUID())
		new.SetResourceVersion(oldTyped.GetResourceVersion())
	}

	g.log.V(4).Info("update results entries")
	obj, _, err := updateResults(oldUnstructured, new.UnstructuredContent(), aggregatedRequests)
	if err != nil {
		return fmt.Errorf("failed to update results entry: %v", err)
	}
	new.Object = obj

	if !hasResultsChanged(oldUnstructured, new.UnstructuredContent()) {
		g.log.V(4).Info("unchanged policy report", "kind", new.GetKind(), "namespace", new.GetNamespace(), "name", new.GetName())
		return nil
	}

	if _, err = g.dclient.UpdateResource(new.GetAPIVersion(), new.GetKind(), new.GetNamespace(), new, false); err != nil {
		return fmt.Errorf("failed to update policy report: %v", err)
	}

	g.log.V(3).Info("successfully updated policy report", "kind", new.GetKind(), "namespace", new.GetNamespace(), "name", new.GetName())
	return
}

func (g *ReportGenerator) cleanupReportRequests(requestsGeneral interface{}) {
	defer g.log.V(5).Info("successfully cleaned up report requests")
	if requests, ok := requestsGeneral.([]*changerequest.ReportChangeRequest); ok {
		for _, request := range requests {
			if err := g.dclient.DeleteResource(request.APIVersion, "ReportChangeRequest", config.KyvernoNamespace, request.Name, false); err != nil {
				if !apierrors.IsNotFound(err) {
					g.log.Error(err, "failed to delete report request")
				}
			}
		}
	}

	if requests, ok := requestsGeneral.([]*changerequest.ClusterReportChangeRequest); ok {
		for _, request := range requests {
			if err := g.dclient.DeleteResource(request.APIVersion, "ClusterReportChangeRequest", "", request.Name, false); err != nil {
				if !apierrors.IsNotFound(err) {
					g.log.Error(err, "failed to delete clusterReportChangeRequest")
				}
			}
		}
	}
}
