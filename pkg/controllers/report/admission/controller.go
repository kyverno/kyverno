package admission

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/controllers/report/utils"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 10
	ControllerName = "admission-report-controller"
	maxRetries     = 10
	deletionGrace  = time.Minute * 2
)

type controller struct {
	// clients
	client  versioned.Interface
	dclient dclient.Interface

	// listers
	admrLister  cache.GenericLister
	cadmrLister cache.GenericLister

	// queue
	queue workqueue.RateLimitingInterface
}

func NewController(
	client versioned.Interface,
	dclient dclient.Interface,
	metadataFactory metadatainformers.SharedInformerFactory,
) controllers.Controller {
	admrInformer := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("admissionreports"))
	cadmrInformer := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("clusteradmissionreports"))
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName)
	c := controller{
		client:      client,
		dclient:     dclient,
		admrLister:  admrInformer.Lister(),
		cadmrLister: cadmrInformer.Lister(),
		queue:       queue,
	}
	if _, err := controllerutils.AddEventHandlersT(
		admrInformer.Informer(),
		func(obj metav1.Object) { queue.Add(cache.ExplicitKey(reportutils.GetResourceUid(obj))) },
		func(old, obj metav1.Object) { queue.Add(cache.ExplicitKey(reportutils.GetResourceUid(old))) },
		func(obj metav1.Object) { queue.Add(cache.ExplicitKey(reportutils.GetResourceUid(obj))) },
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlersT(
		cadmrInformer.Informer(),
		func(obj metav1.Object) { queue.Add(cache.ExplicitKey(reportutils.GetResourceUid(obj))) },
		func(old, obj metav1.Object) { queue.Add(cache.ExplicitKey(reportutils.GetResourceUid(old))) },
		func(obj metav1.Object) { queue.Add(cache.ExplicitKey(reportutils.GetResourceUid(obj))) },
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) getReports(uid types.UID) ([]metav1.Object, error) {
	selector, err := reportutils.SelectorResourceUidEquals(uid)
	if err != nil {
		return nil, err
	}
	var results []metav1.Object
	admrs, err := c.admrLister.List(selector)
	if err != nil {
		return nil, err
	}
	for _, admr := range admrs {
		results = append(results, admr.(metav1.Object))
	}
	cadmrs, err := c.cadmrLister.List(selector)
	if err != nil {
		return nil, err
	}
	for _, cadmr := range cadmrs {
		results = append(results, cadmr.(metav1.Object))
	}
	return results, nil
}

func (c *controller) fetchReport(ctx context.Context, namespace, name string) (kyvernov1alpha2.ReportInterface, error) {
	if namespace == "" {
		return c.client.KyvernoV1alpha2().ClusterAdmissionReports().Get(ctx, name, metav1.GetOptions{})
	} else {
		return c.client.KyvernoV1alpha2().AdmissionReports(namespace).Get(ctx, name, metav1.GetOptions{})
	}
}

func (c *controller) fetchReports(ctx context.Context, uid types.UID) ([]kyvernov1alpha2.ReportInterface, error) {
	var results []kyvernov1alpha2.ReportInterface
	ns := sets.New[string]()
	if reports, err := c.getReports(uid); err != nil {
		return nil, err
	} else {
		// TODO: threshold here ?
		if len(reports) < 5 {
			for _, report := range reports {
				if result, err := c.fetchReport(ctx, report.GetNamespace(), report.GetName()); err != nil {
					return nil, err
				} else {
					results = append(results, result)
				}
			}
			return results, nil
		}
		for _, report := range reports {
			ns.Insert(report.GetNamespace())
		}
	}
	if selector, err := reportutils.SelectorResourceUidEquals(uid); err != nil {
		return nil, err
	} else {
		for n := range ns {
			if n == "" {
				cadmrs, err := c.client.KyvernoV1alpha2().ClusterAdmissionReports().List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
				if err != nil {
					return nil, err
				}
				for i := range cadmrs.Items {
					results = append(results, &cadmrs.Items[i])
				}
			} else {
				admrs, err := c.client.KyvernoV1alpha2().AdmissionReports(n).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
				if err != nil {
					return nil, err
				}
				for i := range admrs.Items {
					results = append(results, &admrs.Items[i])
				}
			}
		}
		return results, nil
	}
}

func (c *controller) deleteReport(ctx context.Context, namespace, name string) error {
	if namespace == "" {
		return c.client.KyvernoV1alpha2().ClusterAdmissionReports().Delete(ctx, name, metav1.DeleteOptions{})
	} else {
		return c.client.KyvernoV1alpha2().AdmissionReports(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}
}

func mergeReports(resource corev1.ObjectReference, accumulator map[string]policyreportv1alpha2.PolicyReportResult, reports ...kyvernov1alpha2.ReportInterface) {
	for _, report := range reports {
		for _, result := range report.GetResults() {
			key := result.Policy + "/" + result.Rule
			result.Resources = []corev1.ObjectReference{resource}
			if rule, exists := accumulator[key]; !exists {
				accumulator[key] = result
			} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
				accumulator[key] = result
			}
		}
	}
}

func (c *controller) aggregateReports(ctx context.Context, uid types.UID) (kyvernov1alpha2.ReportInterface, []kyvernov1alpha2.ReportInterface, error) {
	reports, err := c.fetchReports(ctx, uid)
	if err != nil {
		return nil, nil, err
	}
	if len(reports) == 0 {
		return nil, nil, nil
	}
	// do we have an aggregated report ?
	var aggregated kyvernov1alpha2.ReportInterface
	for _, report := range reports {
		if report.GetName() == string(uid) {
			aggregated = report
			break
		}
	}
	// if we dont, try to fetch the associated resource
	if aggregated == nil || len(aggregated.GetOwnerReferences()) == 0 {
		var res *unstructured.Unstructured
		var gvr schema.GroupVersionResource
		for _, report := range reports {
			// fetch resource using labels recorded on individual reports
			gvr = reportutils.GetResourceGVR(report)
			dyn := c.dclient.GetDynamicInterface().Resource(gvr)
			namespace, name := reportutils.GetResourceNamespaceAndName(report)
			var iface dynamic.ResourceInterface
			if namespace == "" {
				iface = dyn
			} else {
				iface = dyn.Namespace(namespace)
			}
			if got, err := iface.Get(ctx, name, metav1.GetOptions{}); err == nil {
				res = got
				break
			}
		}
		// if we found the resource, build an aggregated report for it
		if res != nil {
			if aggregated == nil {
				aggregated = reportutils.NewAdmissionReport(res.GetNamespace(), string(uid), gvr, *res)
				controllerutils.SetOwner(aggregated, res.GetAPIVersion(), res.GetKind(), res.GetName(), uid)
				controllerutils.SetLabel(aggregated, reportutils.LabelAggregatedReport, string(uid))
			}
		}
	}
	// if we have an aggregated report available, compute results
	var errs []error
	if aggregated != nil && len(aggregated.GetOwnerReferences()) != 0 {
		owner := aggregated.GetOwnerReferences()[0]
		resource := corev1.ObjectReference{
			APIVersion: owner.APIVersion,
			Kind:       owner.Kind,
			Namespace:  aggregated.GetNamespace(),
			Name:       owner.Name,
			UID:        owner.UID,
		}
		merged := map[string]policyreportv1alpha2.PolicyReportResult{}
		for _, report := range reports {
			mergeReports(resource, merged, report)
		}
		var results []policyreportv1alpha2.PolicyReportResult
		for _, result := range merged {
			results = append(results, result)
		}
		after := aggregated
		if aggregated.GetResourceVersion() != "" {
			after = reportutils.DeepCopy(aggregated)
		}
		reportutils.SetResults(after, results...)
		if after.GetResourceVersion() == "" {
			if len(results) > 0 {
				if _, err := reportutils.CreateReport(ctx, after, c.client); err != nil {
					errs = append(errs, err)
				}
			}
		} else {
			if len(results) == 0 {
				if err := c.deleteReport(ctx, after.GetNamespace(), after.GetName()); err != nil {
					errs = append(errs, err)
				}
			} else {
				if !utils.ReportsAreIdentical(aggregated, after) {
					if _, err = reportutils.UpdateReport(ctx, after, c.client); err != nil {
						errs = append(errs, err)
					}
				}
			}
		}
	}
	return aggregated, reports, multierr.Combine(errs...)
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, _, _ string) error {
	uid := types.UID(key)
	// catch invalid reports case
	if uid == "" {
		// find related reports
		objs, err := c.getReports(uid)
		if err != nil {
			return err
		}
		var errs []error
		for _, report := range objs {
			if err := c.deleteReport(ctx, report.GetNamespace(), report.GetName()); err != nil {
				errs = append(errs, err)
			}
		}
		return multierr.Combine(errs...)
	}
	// try to aggregate reports for the given uid
	aggregate, reports, err := c.aggregateReports(ctx, uid)
	var errs []error
	if err != nil {
		errs = append(errs, err)
	}
	// if we created an aggregated report, delete individual ones
	if aggregate != nil {
		for _, report := range reports {
			if aggregate != report {
				if err := c.deleteReport(ctx, report.GetNamespace(), report.GetName()); err != nil {
					errs = append(errs, err)
				}
			}
		}
	} else {
		// we didn't create an aggregated report, still we had some individual reports, let's requeue
		if reports != nil {
			c.queue.AddAfter(cache.ExplicitKey(uid), time.Second*15)
		}
		// delete outdated reports
		for _, report := range reports {
			if report.GetCreationTimestamp().Add(deletionGrace).Before(time.Now()) {
				if err := c.deleteReport(ctx, report.GetNamespace(), report.GetName()); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	return multierr.Combine(errs...)
}
