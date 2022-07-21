package policyreport

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	changerequest "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha2"
	policyreportv1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policyreport/v1alpha2"
	kyvernov1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	policyreportv1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/toggle"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/kyverno/pkg/version"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	prWorkQueueName     = "policy-report-controller"
	clusterpolicyreport = "clusterpolicyreport"

	LabelSelectorKey   = "managed-by"
	LabelSelectorValue = "kyverno"

	deletedPolicyKey = "deletedpolicy"

	resourceExhaustedErr = "ResourceExhausted"
)

var LabelSelector = &metav1.LabelSelector{
	MatchLabels: map[string]string{
		LabelSelectorKey: LabelSelectorValue,
	},
}

// ReportGenerator creates policy report
type ReportGenerator struct {
	pclient kyvernoclient.Interface
	dclient dclient.Interface

	clusterReportInformer    policyreportv1alpha2informers.ClusterPolicyReportInformer
	reportInformer           policyreportv1alpha2informers.PolicyReportInformer
	reportReqInformer        kyvernov1alpha2informers.ReportChangeRequestInformer
	clusterReportReqInformer kyvernov1alpha2informers.ClusterReportChangeRequestInformer

	reportLister                     policyreportv1alpha2listers.PolicyReportLister
	clusterReportLister              policyreportv1alpha2listers.ClusterPolicyReportLister
	reportChangeRequestLister        kyvernov1alpha2listers.ReportChangeRequestLister
	clusterReportChangeRequestLister kyvernov1alpha2listers.ClusterReportChangeRequestLister
	nsLister                         corev1listers.NamespaceLister

	informersSynced []cache.InformerSynced

	queue workqueue.RateLimitingInterface

	// ReconcileCh sends a signal to policy controller to force the reconciliation of policy report
	// if send true, the reports' results will be erased, this is used to recover from the invalid records
	ReconcileCh chan bool

	cleanupChangeRequest chan<- ReconcileInfo

	log logr.Logger
}

// NewReportGenerator returns a new instance of policy report generator
func NewReportGenerator(
	pclient kyvernoclient.Interface,
	dclient dclient.Interface,
	clusterReportInformer policyreportv1alpha2informers.ClusterPolicyReportInformer,
	reportInformer policyreportv1alpha2informers.PolicyReportInformer,
	reportReqInformer kyvernov1alpha2informers.ReportChangeRequestInformer,
	clusterReportReqInformer kyvernov1alpha2informers.ClusterReportChangeRequestInformer,
	namespace corev1informers.NamespaceInformer,
	cleanupChangeRequest chan<- ReconcileInfo,
	log logr.Logger,
) (*ReportGenerator, error) {
	gen := &ReportGenerator{
		pclient:                  pclient,
		dclient:                  dclient,
		clusterReportInformer:    clusterReportInformer,
		reportInformer:           reportInformer,
		reportReqInformer:        reportReqInformer,
		clusterReportReqInformer: clusterReportReqInformer,
		queue:                    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), prWorkQueueName),
		ReconcileCh:              make(chan bool, 10),
		cleanupChangeRequest:     cleanupChangeRequest,
		log:                      log,
	}

	gen.clusterReportLister = clusterReportInformer.Lister()
	gen.reportLister = reportInformer.Lister()
	gen.clusterReportChangeRequestLister = clusterReportReqInformer.Lister()
	gen.reportChangeRequestLister = reportReqInformer.Lister()
	gen.nsLister = namespace.Lister()

	gen.informersSynced = []cache.InformerSynced{clusterReportInformer.Informer().HasSynced, reportInformer.Informer().HasSynced, reportReqInformer.Informer().HasSynced, clusterReportInformer.Informer().HasSynced, namespace.Informer().HasSynced}

	return gen, nil
}

// the key of queue can be
// - <namespace name> for the resource
// - "" for cluster wide resource
// - "deletedpolicy/policyName/ruleName(optional)" for a deleted policy or rule
func (g *ReportGenerator) generateCacheKey(changeRequest interface{}) string {
	if request, ok := changeRequest.(*changerequest.ReportChangeRequest); ok {
		label := request.GetLabels()
		policy := label[deletedLabelPolicy]
		rule := label[deletedLabelRule]
		if rule != "" || policy != "" {
			return strings.Join([]string{deletedPolicyKey, policy, rule}, "/")
		}

		ns := label[ResourceLabelNamespace]
		if ns == "" {
			ns = "default"
		}
		if toggle.SplitPolicyReport() {
			policy = label[policyLabel]
			return strings.Join([]string{ns, policy}, "/")
		} else {
			return ns
		}
	} else if request, ok := changeRequest.(*changerequest.ClusterReportChangeRequest); ok {
		label := request.GetLabels()
		policy := label[deletedLabelPolicy]
		rule := label[deletedLabelRule]
		if rule != "" || policy != "" {
			return strings.Join([]string{deletedPolicyKey, policy, rule}, "/")
		}
		if toggle.SplitPolicyReport() {
			policy = label[policyLabel]
			return strings.Join([]string{"", policy}, "/")
		} else {
			return ""
		}
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

	key := g.generateCacheKey(obj)
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

	key := g.generateCacheKey(cur)
	g.queue.Add(key)
}

func (g *ReportGenerator) addClusterReportChangeRequest(obj interface{}) {
	if !managedRequest(obj) {
		g.cleanupReportRequests([]*changerequest.ClusterReportChangeRequest{obj.(*changerequest.ClusterReportChangeRequest)})
		return
	}

	key := g.generateCacheKey(obj)
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

	key := g.generateCacheKey(cur)
	g.queue.Add(key)
}

func (g *ReportGenerator) deletePolicyReport(obj interface{}) {
	report, ok := kubeutils.GetObjectWithTombstone(obj).(*policyreportv1alpha2.PolicyReport)
	if ok {
		g.log.V(2).Info("PolicyReport deleted", "name", report.GetName())
	} else {
		g.log.Info("Failed to get deleted object", "obj", obj)
	}
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

	if !cache.WaitForNamedCacheSync("PolicyReportGenerator", stopCh, g.informersSynced...) {
		return
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
// otherwise it updates policyReport. the key is of type "namespace/policyname"
func (g *ReportGenerator) syncHandler(key string) (aggregatedRequests interface{}, err error) {
	g.log.V(4).Info("syncing policy report", "key", key)

	if policy, rule, ok := isDeletedPolicyKey(key); ok {
		g.log.V(4).Info("sync policy report on policy deletion")
		return g.removePolicyEntryFromReport(policy, rule)
	}
	var namespace, policyName string
	if toggle.SplitPolicyReport() {
		namespace = strings.Split(key, "/")[0]
		policyName = strings.Split(key, "/")[1]
	} else {
		namespace = key
	}
	new, aggregatedRequests, err := g.aggregateReports(namespace, policyName)
	if err != nil {
		return aggregatedRequests, fmt.Errorf("failed to aggregate reportChangeRequest results %v", err)
	}

	if toggle.SplitPolicyReport() {
		deleteResources := getDeletedResources(aggregatedRequests)
		if len(deleteResources) != 0 {
			for _, dr := range deleteResources {
				if err := g.updateReportsForDeletedResource(dr.name, new, aggregatedRequests); err != nil {
					return aggregatedRequests, err
				}
			}
		}
	}

	var report *policyreportv1alpha2.PolicyReport
	report, err = g.reportLister.PolicyReports(namespace).Get(GeneratePolicyReportName(namespace, policyName))
	if err == nil {
		if val, ok := report.GetLabels()[inactiveLabelKey]; ok && val == inactiveLabelVal {
			g.log.Info("got resourceExhausted error, please opt-in via \"splitPolicyReport\" to generate report per policy")
			return aggregatedRequests, nil
		}
	}

	// Delete changes request does not have the policyName label set
	var old interface{}
	if old, err = g.createReportIfNotPresent(namespace, policyName, new, aggregatedRequests); err != nil {
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
func (g *ReportGenerator) createReportIfNotPresent(namespace, policyName string, new *unstructured.Unstructured, aggregatedRequests interface{}) (report interface{}, err error) {
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

		report, err = g.reportLister.PolicyReports(namespace).Get(GeneratePolicyReportName(namespace, policyName))
		if err != nil {
			if apierrors.IsNotFound(err) && new != nil {
				polr, err := convertToPolr(new)
				if err != nil {
					return nil, fmt.Errorf("failed to convert to policyReport: %v", err)
				}

				if _, err := g.pclient.Wgpolicyk8sV1alpha2().PolicyReports(new.GetNamespace()).Create(context.TODO(), polr, metav1.CreateOptions{}); err != nil {
					return nil, fmt.Errorf("failed to create policyReport: %v", err)
				}

				log.V(2).Info("successfully created policyReport", "namespace", new.GetNamespace(), "name", new.GetName())
				g.cleanupReportRequests(aggregatedRequests)
				return nil, nil
			}

			return nil, fmt.Errorf("unable to get policyReport: %v", err)
		}
	} else {
		report, err = g.clusterReportLister.Get(GeneratePolicyReportName(namespace, policyName))
		if err != nil {
			if apierrors.IsNotFound(err) {
				if new != nil {
					cpolr, err := convertToCpolr(new)
					if err != nil {
						return nil, fmt.Errorf("failed to convert to ClusterPolicyReport: %v", err)
					}

					if _, err := g.pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Create(context.TODO(), cpolr, metav1.CreateOptions{}); err != nil {
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
		newRes := []policyreportv1alpha2.PolicyReportResult{}
		for _, result := range cpolr.Results {
			if ruleName != "" && result.Rule == ruleName && result.Policy == policyName {
				continue
			} else if ruleName == "" && result.Policy == policyName {
				if toggle.SplitPolicyReport() {
					if err := g.pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Delete(context.TODO(), cpolr.GetName(), metav1.DeleteOptions{}); err != nil {
						if apierrors.IsNotFound(err) {
							return nil
						} else {
							return fmt.Errorf("failed to delete clusterPolicyReport %s %v", policyName, err)
						}
					}
				} else {
					continue
				}
			}
			newRes = append(newRes, result)
		}
		cpolr.Results = newRes
		cpolr.Summary = calculateSummary(newRes)
		gv := policyreportv1alpha2.SchemeGroupVersion
		cpolr.SetGroupVersionKind(schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: "ClusterPolicyReport"})
		if _, err := g.pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Update(context.TODO(), cpolr, metav1.UpdateOptions{}); err != nil {
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

	selector, err := metav1.LabelSelectorAsSelector(LabelSelector)
	if err != nil {
		g.log.Error(err, "failed to build labelSelector")
	}

	policyReports := []*policyreportv1alpha2.PolicyReport{}
	for _, ns := range namespaces.Items {
		reports, err := g.reportLister.PolicyReports(ns.GetName()).List(selector)
		if err != nil {
			return fmt.Errorf("unable to list policyReport for namespace %s %v", ns.GetName(), err)
		}
		policyReports = append(policyReports, reports...)
	}

	for _, r := range policyReports {
		newRes := []policyreportv1alpha2.PolicyReportResult{}
		for _, result := range r.Results {
			if ruleName != "" && result.Rule == ruleName && result.Policy == policyName {
				continue
			} else if ruleName == "" && result.Policy == policyName {
				if toggle.SplitPolicyReport() {
					if err := g.pclient.Wgpolicyk8sV1alpha2().PolicyReports(r.GetNamespace()).Delete(context.TODO(), r.GetName(), metav1.DeleteOptions{}); err != nil {
						if apierrors.IsNotFound(err) {
							return nil
						} else {
							return fmt.Errorf("failed to delete PolicyReport %s %v", r.GetName(), err)
						}
					}
				} else {
					continue
				}
			}
			newRes = append(newRes, result)
		}

		r.Results = newRes
		r.Summary = calculateSummary(newRes)
		gv := policyreportv1alpha2.SchemeGroupVersion
		gvk := schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: "PolicyReport"}
		r.SetGroupVersionKind(gvk)

		if _, err := g.pclient.Wgpolicyk8sV1alpha2().PolicyReports(r.GetNamespace()).Update(context.TODO(), r, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update PolicyReport %s %v", r.GetName(), err)
		}
	}
	return nil
}

// aggregateReports aggregates cluster / report change requests to a policy report
func (g *ReportGenerator) aggregateReports(namespace, policyName string) (
	report *unstructured.Unstructured,
	aggregatedRequests interface{},
	err error,
) {
	kyvernoNamespace, err := g.nsLister.Get(config.KyvernoNamespace)
	if err != nil {
		g.log.Error(err, "failed to get Kyverno namespace, policy reports will not be garbage collected upon termination")
	}

	selector := labels.NewSelector()
	if namespace == "" {
		if toggle.SplitPolicyReport() {
			selector = labels.SelectorFromSet(labels.Set(map[string]string{appVersion: version.BuildVersion, policyLabel: TrimmedName(policyName)}))
		} else {
			selector = labels.SelectorFromSet(labels.Set(map[string]string{appVersion: version.BuildVersion}))
		}
		requests, err := g.clusterReportChangeRequestLister.List(selector)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to list ClusterReportChangeRequests within: %v", err)
		}

		if report, aggregatedRequests, err = g.mergeRequests(nil, kyvernoNamespace, policyName, requests); err != nil {
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

		if toggle.SplitPolicyReport() {
			selector = labels.SelectorFromSet(labels.Set(map[string]string{appVersion: version.BuildVersion, ResourceLabelNamespace: namespace, policyLabel: TrimmedName(policyName)}))
		} else {
			selector = labels.SelectorFromSet(labels.Set(map[string]string{appVersion: version.BuildVersion, ResourceLabelNamespace: namespace}))
		}
		requests, err := g.reportChangeRequestLister.ReportChangeRequests(config.KyvernoNamespace).List(selector)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to list reportChangeRequests within namespace %s: %v", ns, err)
		}

		if report, aggregatedRequests, err = g.mergeRequests(ns, kyvernoNamespace, policyName, requests); err != nil {
			return nil, nil, fmt.Errorf("unable to merge results: %v", err)
		}
	}

	return report, aggregatedRequests, nil
}

func (g *ReportGenerator) mergeRequests(ns, kyvernoNs *v1.Namespace, policyName string, requestsGeneral interface{}) (*unstructured.Unstructured, interface{}, error) {
	results := []policyreportv1alpha2.PolicyReportResult{}
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

		report := &policyreportv1alpha2.ClusterPolicyReport{
			Results: results,
			Summary: calculateSummary(results),
		}

		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(report)
		if err != nil {
			return nil, aggregatedRequests, err
		}

		req := &unstructured.Unstructured{Object: obj}

		g.setReport(req, ns, kyvernoNs, policyName)

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

		report := &policyreportv1alpha2.PolicyReport{
			Results: results,
			Summary: calculateSummary(results),
		}

		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(report)
		if err != nil {
			return nil, aggregatedRequests, err
		}

		req := &unstructured.Unstructured{Object: obj}

		g.setReport(req, ns, kyvernoNs, policyName)

		return req, aggregatedRequests, nil
	}

	return nil, nil, nil
}

func (g *ReportGenerator) setReport(reportUnstructured *unstructured.Unstructured, ns, kyvernoNs *v1.Namespace, policyname string) {
	reportUnstructured.SetAPIVersion(policyreportv1alpha2.SchemeGroupVersion.String())
	reportUnstructured.SetLabels(LabelSelector.MatchLabels)

	if kyvernoNs != nil {
		controllerFlag := true

		reportUnstructured.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion: "v1",
				Kind:       "Namespace",
				Name:       kyvernoNs.GetName(),
				UID:        kyvernoNs.GetUID(),
				Controller: &controllerFlag,
			},
		})
	}

	if ns == nil {
		reportUnstructured.SetName(GeneratePolicyReportName("", policyname))
		reportUnstructured.SetKind("ClusterPolicyReport")
		return
	}

	reportUnstructured.SetName(GeneratePolicyReportName(ns.GetName(), policyname))
	reportUnstructured.SetNamespace(ns.GetName())
	reportUnstructured.SetKind("PolicyReport")
}

func (g *ReportGenerator) updateReport(old interface{}, new *unstructured.Unstructured, aggregatedRequests interface{}) (err error) {
	if new == nil {
		g.log.V(4).Info("empty report to update")
		return nil
	}
	g.log.V(4).Info("reconcile policy report")

	oldUnstructured := make(map[string]interface{})

	if oldTyped, ok := old.(*policyreportv1alpha2.ClusterPolicyReport); ok {
		if oldTyped.GetDeletionTimestamp() != nil {
			return g.pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Delete(context.TODO(), oldTyped.Name, metav1.DeleteOptions{})
		}

		if oldUnstructured, err = runtime.DefaultUnstructuredConverter.ToUnstructured(oldTyped); err != nil {
			return fmt.Errorf("unable to convert clusterPolicyReport: %v", err)
		}
		new.SetUID(oldTyped.GetUID())
		new.SetResourceVersion(oldTyped.GetResourceVersion())
	} else if oldTyped, ok := old.(*policyreportv1alpha2.PolicyReport); ok {
		if oldTyped.GetDeletionTimestamp() != nil {
			return g.pclient.Wgpolicyk8sV1alpha2().PolicyReports(oldTyped.Namespace).Delete(context.TODO(), oldTyped.Name, metav1.DeleteOptions{})
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

	if new.GetKind() == "PolicyReport" {
		polr, err := convertToPolr(new)
		if err != nil {
			return fmt.Errorf("error converting to PolicyReport: %v", err)
		}
		if _, err := g.pclient.Wgpolicyk8sV1alpha2().PolicyReports(new.GetNamespace()).Update(context.TODO(), polr, metav1.UpdateOptions{}); err != nil {
			if strings.Contains(err.Error(), resourceExhaustedErr) {
				g.log.V(4).Info("got ResourceExhausted error, cleanning up change requests and erasing report results")

				annotations := polr.GetAnnotations()
				if annotations == nil {
					annotations = make(map[string]string)
				}
				annotations[inactiveLabelKey] = "Unable to update policy report due to resourceExhausted error, please enable the flag \"splitPolicyReport\" to generate a report per policy"
				polr.SetAnnotations(annotations)

				labels := polr.GetLabels()
				labels[inactiveLabelKey] = inactiveLabelVal
				polr.SetLabels(labels)

				polr.Results = []policyreportv1alpha2.PolicyReportResult{}
				polr.Summary = policyreportv1alpha2.PolicyReportSummary{}
				if _, err := g.pclient.Wgpolicyk8sV1alpha2().PolicyReports(new.GetNamespace()).Update(context.TODO(), polr, metav1.UpdateOptions{}); err != nil {
					return fmt.Errorf("failed to erase policy report results: %v", err)
				}
				ns := new.GetNamespace()
				g.cleanupChangeRequest <- ReconcileInfo{Namespace: &ns, MapperInactive: true}
				return nil
			}
			return fmt.Errorf("failed to update PolicyReport: %v", err)
		}
	}

	if new.GetKind() == "ClusterPolicyReport" {
		cpolr, err := convertToCpolr(new)
		if err != nil {
			return fmt.Errorf("error converting to ClusterPolicyReport: %v", err)
		}
		if _, err := g.pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Update(context.TODO(), cpolr, metav1.UpdateOptions{}); err != nil {
			if strings.Contains(err.Error(), resourceExhaustedErr) {
				g.log.V(4).Info("got ResourceExhausted error, cleanning up change requests and erasing report results")

				annotations := cpolr.GetAnnotations()
				if annotations == nil {
					annotations = make(map[string]string)
				}
				annotations[inactiveLabelKey] = "Unable to update cluster policy report due to resourceExhausted error, please enable the flag \"splitPolicyReport\" to generate report per policy"
				cpolr.SetAnnotations(annotations)

				labels := cpolr.GetLabels()
				labels[inactiveLabelKey] = inactiveLabelVal
				cpolr.SetLabels(labels)

				cpolr.Results = []policyreportv1alpha2.PolicyReportResult{}
				cpolr.Summary = policyreportv1alpha2.PolicyReportSummary{}
				if _, err := g.pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Update(context.TODO(), cpolr, metav1.UpdateOptions{}); err != nil {
					return fmt.Errorf("failed to erase cluster policy report results: %v", err)
				}
				ns := ""
				g.cleanupChangeRequest <- ReconcileInfo{Namespace: &ns, MapperInactive: true}
				return nil
			}
			return fmt.Errorf("failed to update ClusterPolicyReport: %v", err)
		}

	}

	g.log.V(3).Info("successfully updated policy report", "kind", new.GetKind(), "namespace", new.GetNamespace(), "name", new.GetName())
	return
}

func (g *ReportGenerator) updateReportsForDeletedResource(resName string, new *unstructured.Unstructured, aggregatedRequests interface{}) (err error) {
	if _, ok := aggregatedRequests.([]*changerequest.ClusterReportChangeRequest); ok {
		cpolrs, err := g.clusterReportLister.List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list clusterPolicyReport %v", err)
		}
		for _, cpolr := range cpolrs {
			newRes := []policyreportv1alpha2.PolicyReportResult{}
			for _, result := range cpolr.Results {
				if len(result.Resources) != 0 {
					for _, res := range result.Resources {
						if res.Name != resName {
							newRes = append(newRes, result)
						}
					}
				}
			}
			cpolr.Results = newRes
			cpolr.Summary = calculateSummary(newRes)
			gv := policyreportv1alpha2.SchemeGroupVersion
			cpolr.SetGroupVersionKind(schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: "ClusterPolicyReport"})
			if _, err := g.pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Update(context.TODO(), cpolr, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("failed to update clusterPolicyReport %s %v", cpolr.Name, err)
			}
		}
	} else {
		polrs, err := g.reportLister.List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list clusterPolicyReport %v", err)
		}
		for _, polr := range polrs {
			newRes1 := []policyreportv1alpha2.PolicyReportResult{}
			for _, result := range polr.Results {
				if len(result.Resources) != 0 {
					for _, res := range result.Resources {
						if res.Name != resName {
							newRes1 = append(newRes1, result)
						}
					}
				}
			}
			polr.Results = newRes1
			polr.Summary = calculateSummary(newRes1)
			gv := policyreportv1alpha2.SchemeGroupVersion

			polr.SetGroupVersionKind(schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: "PolicyReport"})

			if _, err := g.pclient.Wgpolicyk8sV1alpha2().PolicyReports(new.GetNamespace()).Update(context.TODO(), polr, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("failed to update clusterPolicyReport %s %v", polr.Name, err)
			}
		}
	}

	return
}

func (g *ReportGenerator) cleanupReportRequests(requestsGeneral interface{}) {
	defer g.log.V(5).Info("successfully cleaned up report requests")
	if requests, ok := requestsGeneral.([]*changerequest.ReportChangeRequest); ok {
		for _, request := range requests {
			if err := g.pclient.KyvernoV1alpha2().ReportChangeRequests(config.KyvernoNamespace).Delete(context.TODO(), request.Name, metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					g.log.Error(err, "failed to delete report request")
				}
			}
		}
	}

	if requests, ok := requestsGeneral.([]*changerequest.ClusterReportChangeRequest); ok {
		for _, request := range requests {
			if err := g.pclient.KyvernoV1alpha2().ClusterReportChangeRequests().Delete(context.TODO(), request.Name, metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					g.log.Error(err, "failed to delete clusterReportChangeRequest")
				}
			}
		}
	}
}
