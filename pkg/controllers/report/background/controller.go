package background

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/controllers/report/resource"
	"github.com/kyverno/kyverno/pkg/controllers/report/utils"
	"github.com/kyverno/kyverno/pkg/engine/context/resolvers"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers                = 2
	ControllerName         = "background-scan-controller"
	maxRetries             = 10
	annotationLastScanTime = "audit.kyverno.io/last-scan-time"
	enqueueDelay           = 30 * time.Second
)

type controller struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface
	rclient       registryclient.Client

	// listers
	polLister      kyvernov1listers.PolicyLister
	cpolLister     kyvernov1listers.ClusterPolicyLister
	bgscanrLister  cache.GenericLister
	cbgscanrLister cache.GenericLister
	nsLister       corev1listers.NamespaceLister

	// queue
	queue          workqueue.RateLimitingInterface
	bgscanEnqueue  controllerutils.EnqueueFunc
	cbgscanEnqueue controllerutils.EnqueueFunc

	// cache
	metadataCache          resource.MetadataCache
	informerCacheResolvers resolvers.ConfigmapResolver
	forceDelay             time.Duration

	// config
	config   config.Configuration
	eventGen event.Interface
}

func NewController(
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	rclient registryclient.Client,
	metadataFactory metadatainformers.SharedInformerFactory,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	nsInformer corev1informers.NamespaceInformer,
	metadataCache resource.MetadataCache,
	informerCacheResolvers resolvers.ConfigmapResolver,
	forceDelay time.Duration,
	config config.Configuration,
	eventGen event.Interface,
) controllers.Controller {
	bgscanr := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("backgroundscanreports"))
	cbgscanr := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("clusterbackgroundscanreports"))
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName)
	c := controller{
		client:                 client,
		kyvernoClient:          kyvernoClient,
		rclient:                rclient,
		polLister:              polInformer.Lister(),
		cpolLister:             cpolInformer.Lister(),
		bgscanrLister:          bgscanr.Lister(),
		cbgscanrLister:         cbgscanr.Lister(),
		nsLister:               nsInformer.Lister(),
		queue:                  queue,
		bgscanEnqueue:          controllerutils.AddDefaultEventHandlers(logger, bgscanr.Informer(), queue),
		cbgscanEnqueue:         controllerutils.AddDefaultEventHandlers(logger, cbgscanr.Informer(), queue),
		metadataCache:          metadataCache,
		informerCacheResolvers: informerCacheResolvers,
		forceDelay:             forceDelay,
		config:                 config,
		eventGen:               eventGen,
	}
	controllerutils.AddEventHandlersT(polInformer.Informer(), c.addPolicy, c.updatePolicy, c.deletePolicy)
	controllerutils.AddEventHandlersT(cpolInformer.Informer(), c.addPolicy, c.updatePolicy, c.deletePolicy)
	c.metadataCache.AddEventHandler(func(eventType resource.EventType, uid types.UID, _ schema.GroupVersionKind, res resource.Resource) {
		// if it's a deletion, nothing to do
		if eventType == resource.Deleted {
			return
		}
		if res.Namespace == "" {
			c.queue.AddAfter(string(uid), enqueueDelay)
		} else {
			c.queue.AddAfter(res.Namespace+"/"+string(uid), enqueueDelay)
		}
	})
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) addPolicy(obj kyvernov1.PolicyInterface) {
	selector, err := reportutils.SelectorPolicyDoesNotExist(obj)
	if err != nil {
		logger.Error(err, "failed to create label selector")
	}
	if err := c.enqueue(selector); err != nil {
		logger.Error(err, "failed to enqueue")
	}
}

func (c *controller) updatePolicy(old, obj kyvernov1.PolicyInterface) {
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		selector, err := reportutils.SelectorPolicyNotEquals(obj)
		if err != nil {
			logger.Error(err, "failed to create label selector")
		}
		if err := c.enqueue(selector); err != nil {
			logger.Error(err, "failed to enqueue")
		}
	}
}

func (c *controller) deletePolicy(obj kyvernov1.PolicyInterface) {
	selector, err := reportutils.SelectorPolicyExists(obj)
	if err != nil {
		logger.Error(err, "failed to create label selector")
	}
	if err := c.enqueue(selector); err != nil {
		logger.Error(err, "failed to enqueue")
	}
}

func (c *controller) enqueue(selector labels.Selector) error {
	bgscans, err := c.bgscanrLister.List(selector)
	if err != nil {
		return err
	}
	for _, bgscan := range bgscans {
		err = c.bgscanEnqueue(bgscan)
		if err != nil {
			logger.Error(err, "failed to enqueue")
		}
	}
	cbgscans, err := c.cbgscanrLister.List(selector)
	if err != nil {
		return err
	}
	for _, cbgscan := range cbgscans {
		err = c.cbgscanEnqueue(cbgscan)
		if err != nil {
			logger.Error(err, "failed to enqueue")
		}
	}
	return nil
}

// TODO: utils
func (c *controller) fetchClusterPolicies(logger logr.Logger) ([]kyvernov1.PolicyInterface, error) {
	var policies []kyvernov1.PolicyInterface
	if cpols, err := c.cpolLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, cpol := range cpols {
			policies = append(policies, cpol)
		}
	}
	return policies, nil
}

// TODO: utils
func (c *controller) fetchPolicies(logger logr.Logger, namespace string) ([]kyvernov1.PolicyInterface, error) {
	var policies []kyvernov1.PolicyInterface
	if pols, err := c.polLister.Policies(namespace).List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, pol)
		}
	}
	return policies, nil
}

func (c *controller) updateReport(ctx context.Context, meta metav1.Object, gvk schema.GroupVersionKind, resource resource.Resource) error {
	namespace := meta.GetNamespace()
	metaLabels := meta.GetLabels()
	// load all policies
	policies, err := c.fetchClusterPolicies(logger)
	if err != nil {
		return err
	}
	if namespace != "" {
		pols, err := c.fetchPolicies(logger, namespace)
		if err != nil {
			return err
		}
		policies = append(policies, pols...)
	}
	// 	load background policies
	backgroundPolicies := utils.RemoveNonBackgroundPolicies(logger, policies...)
	if err != nil {
		return err
	}
	force := false
	metaAnnotations := meta.GetAnnotations()
	if metaAnnotations == nil || metaAnnotations[annotationLastScanTime] == "" {
		force = true
	} else {
		annTime, err := time.Parse(time.RFC3339, metaAnnotations[annotationLastScanTime])
		if err != nil {
			logging.Error(err, "failed to parse last scan time annotation", "namespace", resource.Namespace, "name", resource.Name, "hash", resource.Hash)
			force = true
		} else {
			force = time.Now().After(annTime.Add(c.forceDelay))
		}
	}
	if force {
		logging.Info("force bg scan report", "namespace", resource.Namespace, "name", resource.Name, "hash", resource.Hash)
	}
	//	if the resource changed, we need to rebuild the report
	if force || !reportutils.CompareHash(meta, resource.Hash) {
		scanner := utils.NewScanner(logger, c.client, c.rclient, c.informerCacheResolvers, c.config, c.eventGen)
		before, err := c.getReport(ctx, meta.GetNamespace(), meta.GetName())
		if err != nil {
			return nil
		}
		report := reportutils.DeepCopy(before)
		resource, err := c.client.GetResource(ctx, gvk.GroupVersion().String(), gvk.Kind, resource.Namespace, resource.Name)
		if err != nil {
			return err
		}
		reportutils.SetResourceVersionLabels(report, resource)
		if resource == nil {
			return nil
		}
		var nsLabels map[string]string
		if namespace != "" {
			ns, err := c.nsLister.Get(namespace)
			if err != nil {
				return err
			}
			nsLabels = ns.GetLabels()
		}
		var responses []*response.EngineResponse
		for _, result := range scanner.ScanResource(ctx, *resource, nsLabels, backgroundPolicies...) {
			if result.Error != nil {
				logger.Error(result.Error, "failed to apply policy")
			} else {
				responses = append(responses, result.EngineResponse)
			}
		}
		controllerutils.SetAnnotation(report, annotationLastScanTime, time.Now().Format(time.RFC3339))
		reportutils.SetResponses(report, responses...)
		if utils.ReportsAreIdentical(before, report) {
			return nil
		}
		_, err = reportutils.UpdateReport(ctx, report, c.kyvernoClient)
		return err
	} else {
		expected := map[string]kyvernov1.PolicyInterface{}
		for _, policy := range backgroundPolicies {
			expected[reportutils.PolicyLabel(policy)] = policy
		}
		toDelete := map[string]string{}
		for label := range metaLabels {
			if reportutils.IsPolicyLabel(label) {
				// if the policy doesn't exist anymore
				if expected[label] == nil {
					if name, err := reportutils.PolicyNameFromLabel(namespace, label); err != nil {
						return err
					} else {
						toDelete[name] = label
					}
				}
			}
		}
		var toCreate []kyvernov1.PolicyInterface
		for label, policy := range expected {
			// if the background policy changed, we need to recreate entries
			if metaLabels[label] != policy.GetResourceVersion() {
				if name, err := reportutils.PolicyNameFromLabel(namespace, label); err != nil {
					return err
				} else {
					toDelete[name] = label
				}
				toCreate = append(toCreate, policy)
			}
		}
		if len(toDelete) == 0 && len(toCreate) == 0 {
			return nil
		}
		before, err := c.getReport(ctx, meta.GetNamespace(), meta.GetName())
		if err != nil {
			return err
		}
		report := reportutils.DeepCopy(before)
		var ruleResults []policyreportv1alpha2.PolicyReportResult
		// deletions
		reportLabels := report.GetLabels()
		if reportLabels != nil {
			for _, label := range toDelete {
				delete(reportLabels, label)
			}
		}
		for _, result := range report.GetResults() {
			if _, ok := toDelete[result.Policy]; !ok {
				ruleResults = append(ruleResults, result)
			}
		}
		// creations
		if len(toCreate) > 0 {
			scanner := utils.NewScanner(logger, c.client, c.rclient, c.informerCacheResolvers, c.config, c.eventGen)
			resource, err := c.client.GetResource(ctx, gvk.GroupVersion().String(), gvk.Kind, resource.Namespace, resource.Name)
			if err != nil {
				return err
			}
			reportutils.SetResourceVersionLabels(report, resource)
			var nsLabels map[string]string
			if namespace != "" {
				ns, err := c.nsLister.Get(namespace)
				if err != nil {
					return err
				}
				nsLabels = ns.GetLabels()
			}
			for _, result := range scanner.ScanResource(ctx, *resource, nsLabels, toCreate...) {
				if result.Error != nil {
					return result.Error
				} else {
					reportutils.SetPolicyLabel(report, result.EngineResponse.Policy)
					ruleResults = append(ruleResults, reportutils.EngineResponseToReportResults(result.EngineResponse)...)
				}
			}
		}
		reportutils.SetResults(report, ruleResults...)
		if utils.ReportsAreIdentical(before, report) {
			return nil
		}
		_, err = reportutils.UpdateReport(ctx, report, c.kyvernoClient)
		return err
	}
}

func (c *controller) getReport(ctx context.Context, namespace, name string) (kyvernov1alpha2.ReportInterface, error) {
	if namespace == "" {
		return c.kyvernoClient.KyvernoV1alpha2().ClusterBackgroundScanReports().Get(ctx, name, metav1.GetOptions{})
	} else {
		return c.kyvernoClient.KyvernoV1alpha2().BackgroundScanReports(namespace).Get(ctx, name, metav1.GetOptions{})
	}
}

func (c *controller) getMeta(namespace, name string) (metav1.Object, error) {
	if namespace == "" {
		obj, err := c.cbgscanrLister.Get(name)
		if err != nil {
			return nil, err
		}
		return obj.(metav1.Object), err
	} else {
		obj, err := c.bgscanrLister.ByNamespace(namespace).Get(name)
		if err != nil {
			return nil, err
		}
		return obj.(metav1.Object), err
	}
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, _, namespace, name string) error {
	// try to find resource from the cache
	uid := types.UID(name)
	resource, gvk, exists := c.metadataCache.GetResourceHash(uid)
	// if the resource is not present it means we shouldn't have a report for it
	// we can delete the report, we will recreate one if the resource comes back
	if !exists {
		report, err := c.getMeta(namespace, name)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		} else {
			if report.GetNamespace() == "" {
				return c.kyvernoClient.KyvernoV1alpha2().ClusterBackgroundScanReports().Delete(ctx, report.GetName(), metav1.DeleteOptions{})
			} else {
				return c.kyvernoClient.KyvernoV1alpha2().BackgroundScanReports(report.GetNamespace()).Delete(ctx, report.GetName(), metav1.DeleteOptions{})
			}
		}
		return nil
	}
	// try to find report from the cache
	report, err := c.getMeta(namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// if there's no report yet, try to create an empty one
			_, err = reportutils.CreateReport(ctx, reportutils.NewBackgroundScanReport(namespace, name, gvk, resource.Name, uid), c.kyvernoClient)
			return err
		}
		return err
	}
	return c.updateReport(ctx, report, gvk, resource)
}
