package aggregate

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/controllers/report/resource"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	admissionregistrationv1alpha1informers "k8s.io/client-go/informers/admissionregistration/v1alpha1"
	admissionregistrationv1alpha1listers "k8s.io/client-go/listers/admissionregistration/v1alpha1"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 10
	ControllerName = "aggregate-report-controller"
	maxRetries     = 10
	enqueueDelay   = 10 * time.Second
	deletionGrace  = time.Minute * 2
)

type controller struct {
	// clients
	client  versioned.Interface
	dclient dclient.Interface

	// listers
	polLister   kyvernov1listers.PolicyLister
	cpolLister  kyvernov1listers.ClusterPolicyLister
	vapLister   admissionregistrationv1alpha1listers.ValidatingAdmissionPolicyLister
	ephrLister  cache.GenericLister
	cephrLister cache.GenericLister

	// frontQueue
	frontQueue workqueue.RateLimitingInterface
	backQueue  workqueue.RateLimitingInterface

	// cache
	metadataCache resource.MetadataCache

	chunkSize int
}

type policyMapEntry struct {
	policy kyvernov1.PolicyInterface
	rules  sets.Set[string]
}

func NewController(
	client versioned.Interface,
	dclient dclient.Interface,
	metadataFactory metadatainformers.SharedInformerFactory,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	vapInformer admissionregistrationv1alpha1informers.ValidatingAdmissionPolicyInformer,
	metadataCache resource.MetadataCache,
	chunkSize int,
) controllers.Controller {
	ephrInformer := metadataFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("ephemeralreports"))
	cephrInformer := metadataFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("clusterephemeralreports"))
	// polrInformer := metadataFactory.ForResource(policyreportv1alpha2.SchemeGroupVersion.WithResource("policyreports"))
	// cpolrInformer := metadataFactory.ForResource(policyreportv1alpha2.SchemeGroupVersion.WithResource("clusterpolicyreports"))
	c := controller{
		client:        client,
		dclient:       dclient,
		polLister:     polInformer.Lister(),
		cpolLister:    cpolInformer.Lister(),
		ephrLister:    ephrInformer.Lister(),
		cephrLister:   cephrInformer.Lister(),
		frontQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
		backQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
		metadataCache: metadataCache,
		chunkSize:     chunkSize,
	}
	if _, _, err := controllerutils.AddDelayedDefaultEventHandlers(logger, ephrInformer.Informer(), c.frontQueue, enqueueDelay); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, _, err := controllerutils.AddDelayedDefaultEventHandlers(logger, cephrInformer.Informer(), c.frontQueue, enqueueDelay); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, _, err := controllerutils.AddDelayedDefaultEventHandlers(logger, ephrInformer.Informer(), c.backQueue, enqueueDelay); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, _, err := controllerutils.AddDelayedDefaultEventHandlers(logger, cephrInformer.Informer(), c.backQueue, enqueueDelay); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	var group wait.Group
	group.StartWithContext(ctx, func(ctx context.Context) {
		controllerutils.Run(ctx, logger, ControllerName, time.Second, c.frontQueue, workers, maxRetries, c.frontReconcile)
	})
	group.StartWithContext(ctx, func(ctx context.Context) {
		controllerutils.Run(ctx, logger, ControllerName, time.Second, c.backQueue, workers, maxRetries, c.backReconcile)
	})
	group.Wait()
}

// func (c *controller) createPolicyMap() (map[string]policyMapEntry, error) {
// 	results := map[string]policyMapEntry{}
// 	cpols, err := c.cpolLister.List(labels.Everything())
// 	if err != nil {
// 		return nil, err
// 	}
// 	for _, cpol := range cpols {
// 		key, err := cache.MetaNamespaceKeyFunc(cpol)
// 		if err != nil {
// 			return nil, err
// 		}
// 		results[key] = policyMapEntry{
// 			policy: cpol,
// 			rules:  sets.New[string](),
// 		}
// 		for _, rule := range autogen.ComputeRules(cpol) {
// 			results[key].rules.Insert(rule.Name)
// 		}
// 	}
// 	pols, err := c.polLister.List(labels.Everything())
// 	if err != nil {
// 		return nil, err
// 	}
// 	for _, pol := range pols {
// 		key, err := cache.MetaNamespaceKeyFunc(pol)
// 		if err != nil {
// 			return nil, err
// 		}
// 		results[key] = policyMapEntry{
// 			policy: pol,
// 			rules:  sets.New[string](),
// 		}
// 		for _, rule := range autogen.ComputeRules(pol) {
// 			results[key].rules.Insert(rule.Name)
// 		}
// 	}
// 	return results, nil
// }

// func (c *controller) createVapMap() (sets.Set[string], error) {
// 	results := sets.New[string]()
// 	if c.vapLister != nil {
// 		vaps, err := c.vapLister.List(labels.Everything())
// 		if err != nil {
// 			return nil, err
// 		}
// 		for _, vap := range vaps {
// 			key, err := cache.MetaNamespaceKeyFunc(vap)
// 			if err != nil {
// 				return nil, err
// 			}
// 			results.Insert(key)
// 		}
// 	}
// 	return results, nil
// }

// func (c *controller) getBackgroundScanReport(ctx context.Context, namespace, name string) (kyvernov1alpha2.ReportInterface, error) {
// 	if namespace == "" {
// 		report, err := c.client.ReportsV1().ClusterEphemeralReports().Get(ctx, name, metav1.GetOptions{})
// 		if err != nil {
// 			if apierrors.IsNotFound(err) {
// 				return nil, nil
// 			}
// 			return nil, err
// 		}
// 		return report, nil
// 	} else {
// 		report, err := c.client.ReportsV1().EphemeralReports(namespace).Get(ctx, name, metav1.GetOptions{})
// 		if err != nil {
// 			if apierrors.IsNotFound(err) {
// 				return nil, nil
// 			}
// 			return nil, err
// 		}
// 		return report, nil
// 	}
// }

// func (c *controller) getEphemeralReports(ctx context.Context, namespace, name string) ([]kyvernov1alpha2.ReportInterface, error) {
// 	selector, err := reportutils.SelectorResourceUidEquals(types.UID(name))
// 	if err != nil {
// 		return nil, err
// 	}
// 	var results []kyvernov1alpha2.ReportInterface
// 	if namespace == "" {
// 		reports, err := c.client.ReportsV1().ClusterEphemeralReports().List(ctx, metav1.ListOptions{
// 			LabelSelector: selector.String(),
// 		})
// 		if err != nil {
// 			if apierrors.IsNotFound(err) {
// 				return nil, nil
// 			}
// 			return nil, err
// 		}
// 		for _, report := range reports.Items {
// 			report := report
// 			results = append(results, &report)
// 		}
// 	} else {
// 		reports, err := c.client.ReportsV1().EphemeralReports(namespace).List(ctx, metav1.ListOptions{
// 			LabelSelector: selector.String(),
// 		})
// 		if err != nil {
// 			if apierrors.IsNotFound(err) {
// 				return nil, nil
// 			}
// 			return nil, err
// 		}
// 		for _, report := range reports.Items {
// 			report := report
// 			results = append(results, &report)
// 		}
// 	}
// 	return results, nil
// }

// func (c *controller) getPolicyReport(ctx context.Context, namespace, name string) (kyvernov1alpha2.ReportInterface, error) {
// 	if namespace == "" {
// 		report, err := c.client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Get(ctx, name, metav1.GetOptions{})
// 		if err != nil {
// 			if apierrors.IsNotFound(err) {
// 				return nil, nil
// 			}
// 			return nil, err
// 		}
// 		return report, nil
// 	} else {
// 		report, err := c.client.Wgpolicyk8sV1alpha2().PolicyReports(namespace).Get(ctx, name, metav1.GetOptions{})
// 		if err != nil {
// 			if apierrors.IsNotFound(err) {
// 				return nil, nil
// 			}
// 			return nil, err
// 		}
// 		return report, nil
// 	}
// }

//	func (c *controller) getReports(ctx context.Context, namespace, name string) ([]kyvernov1alpha2.ReportInterface, kyvernov1alpha2.ReportInterface, error) {
//		ephemeralReports, err := c.getEphemeralReports(ctx, namespace, name)
//		if err != nil {
//			return nil, nil, err
//		}
//		backgroundReport, err := c.getBackgroundScanReport(ctx, namespace, name)
//		if err != nil {
//			return nil, nil, err
//		}
//		return ephemeralReports, backgroundReport, nil
//	}

func (c *controller) lookupEphemeralReportMeta(_ context.Context, namespace, name string) (*metav1.PartialObjectMetadata, error) {
	if namespace == "" {
		obj, err := c.cephrLister.Get(name)
		if err != nil {
			return nil, err
		}
		return obj.(*metav1.PartialObjectMetadata), nil
	} else {
		obj, err := c.ephrLister.ByNamespace(namespace).Get(name)
		if err != nil {
			return nil, err
		}
		return obj.(*metav1.PartialObjectMetadata), nil
	}
}

func (c *controller) getEphemeralReport(ctx context.Context, namespace, name string) (kyvernov1alpha2.ReportInterface, error) {
	if namespace == "" {
		obj, err := c.client.ReportsV1().ClusterEphemeralReports().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return obj, err
	} else {
		obj, err := c.client.ReportsV1().EphemeralReports(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return obj, err
	}
}

func (c *controller) deleteEphemeralReport(ctx context.Context, namespace, name string) error {
	if namespace == "" {
		return c.client.ReportsV1().ClusterEphemeralReports().Delete(ctx, name, metav1.DeleteOptions{})
	} else {
		return c.client.ReportsV1().EphemeralReports(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}
}

func (c *controller) adopt(ctx context.Context, reportMeta *metav1.PartialObjectMetadata) bool {
	gvr := reportutils.GetResourceGVR(reportMeta)
	dyn := c.dclient.GetDynamicInterface().Resource(gvr)
	namespace, name := reportutils.GetResourceNamespaceAndName(reportMeta)
	var iface dynamic.ResourceInterface
	if namespace == "" {
		iface = dyn
	} else {
		iface = dyn.Namespace(namespace)
	}
	resource, err := iface.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false
	}
	if resource == nil {
		return false
	}
	report, err := c.getEphemeralReport(ctx, reportMeta.GetNamespace(), reportMeta.GetName())
	if err != nil {
		return false
	}
	if report == nil {
		return false
	}
	controllerutils.SetOwner(report, resource.GetAPIVersion(), resource.GetKind(), resource.GetName(), resource.GetUID())
	if _, err := updateReport(ctx, report, c.client); err != nil {
		return false
	}
	return true
}

func (c *controller) frontReconcile(ctx context.Context, logger logr.Logger, _, namespace, name string) error {
	reportMeta, err := c.lookupEphemeralReportMeta(ctx, namespace, name)
	// try to lookup metadata from lister
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}
	// check if it is owned already
	if len(reportMeta.OwnerReferences) != 0 {
		// if reportutils.GetSource(reportMeta) == "admission" && isTooOld(reportMeta) {
		if isTooOld(reportMeta) {
			return c.deleteEphemeralReport(ctx, reportMeta.GetNamespace(), reportMeta.GetName())
		}
		return nil
	}
	// try to find the owner
	if c.adopt(ctx, reportMeta) {
		return nil
	}
	// if not found and too old, forget about it
	if isTooOld(reportMeta) {
		return c.deleteEphemeralReport(ctx, reportMeta.GetNamespace(), reportMeta.GetName())
	}
	// else try again later
	c.frontQueue.AddAfter(reportMeta, enqueueDelay)
	return nil
}

func (c *controller) backReconcile(ctx context.Context, logger logr.Logger, _, namespace, name string) error {
	// reportMeta, err := c.lookupEphemeralReportMeta(ctx, namespace, name)
	// // try to lookup metadata from lister
	// if err != nil {
	// 	if !apierrors.IsNotFound(err) {
	// 		return err
	// 	}
	// 	return nil
	// }
	// // check if it is owned already
	// if len(reportMeta.OwnerReferences) != 0 {
	// 	// if reportutils.GetSource(reportMeta) == "admission" && isTooOld(reportMeta) {
	// 	if isTooOld(reportMeta) {
	// 		return c.deleteEphemeralReport(ctx, reportMeta.GetNamespace(), reportMeta.GetName())
	// 	}
	// 	return nil
	// }
	// // try to find the owner
	// if c.adopt(ctx, reportMeta) {
	// 	return nil
	// }
	// // if not found and too old, forget about it
	// if isTooOld(reportMeta) {
	// 	return c.deleteEphemeralReport(ctx, reportMeta.GetNamespace(), reportMeta.GetName())
	// }
	// // else try again later
	// c.frontQueue.AddAfter(reportMeta, enqueueDelay)
	return nil
}
