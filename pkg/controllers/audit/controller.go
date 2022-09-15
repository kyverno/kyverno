package audit

import (
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha2"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/policy"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// TODO: skip resources to be filtered
// TODO: get discovery schema
// TODO: clean up dangling reports

// DONE: cache background policies
// DONE: validate variables
// DONE: transmit logger
// DONE: build kinds
// DONE: filter out unnecessary rules
// DONE: managed by kyverno label
// DONE: deep copy if coming from cache

const (
	maxRetries = 10
	workers    = 3
)

type controller struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface

	// listers
	polLister  kyvernov1listers.PolicyLister
	cpolLister kyvernov1listers.ClusterPolicyLister
	rcrLister  kyvernov1alpha2listers.ReportChangeRequestLister
	crcrLister kyvernov1alpha2listers.ClusterReportChangeRequestLister

	// queue
	queue workqueue.RateLimitingInterface
}

func NewController(
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	rcrInformer kyvernov1alpha2informers.ReportChangeRequestInformer,
	crcrInformer kyvernov1alpha2informers.ClusterReportChangeRequestInformer,
) *controller {
	c := controller{
		client:        client,
		kyvernoClient: kyvernoClient,
		polLister:     polInformer.Lister(),
		cpolLister:    cpolInformer.Lister(),
		rcrLister:     rcrInformer.Lister(),
		crcrLister:    crcrInformer.Lister(),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
	}
	controllerutils.AddDefaultEventHandlers(logger, polInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, cpolInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	go c.ticker(stopCh)
	controllerutils.Run(controllerName, logger, c.queue, workers, maxRetries, c.reconcile, stopCh /*, c.configmapSynced*/)
}

// TODO: can be static
func (c *controller) canBackgroundProcess(logger logr.Logger, p kyvernov1.PolicyInterface) bool {
	if !p.BackgroundProcessingEnabled() {
		return false
	}
	if err := policy.ValidateVariables(p, true); err != nil {
		return false
	}
	return true
}

// TODO: can be static
func (c *controller) buildKindSet(logger logr.Logger, policies ...kyvernov1.PolicyInterface) sets.String {
	kinds := sets.NewString()
	for _, policy := range policies {
		for _, rule := range autogen.ComputeRules(policy) {
			if rule.HasValidate() || rule.HasVerifyImages() {
				kinds.Insert(rule.MatchResources.GetKinds()...)
			}
		}
	}
	return kinds
}

func (c *controller) fetchBackgroundPolicies(logger logr.Logger) ([]kyvernov1.PolicyInterface, error) {
	var policies []kyvernov1.PolicyInterface
	if pols, err := c.polLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			if c.canBackgroundProcess(logger, pol) {
				policies = append(policies, pol.DeepCopy())
			}
		}
	}
	if cpols, err := c.cpolLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, cpol := range cpols {
			if c.canBackgroundProcess(logger, cpol) {
				policies = append(policies, cpol.DeepCopy())
			}
		}
	}
	return policies, nil
}

func (c *controller) fetchResources(logger logr.Logger, policies ...kyvernov1.PolicyInterface) ([]unstructured.Unstructured, error) {
	var resources []unstructured.Unstructured
	kinds := c.buildKindSet(logger, policies...)
	for kind := range kinds {
		list, err := c.client.ListResource("", kind, "" /*labelSelector*/, nil)
		if err != nil {
			logger.Error(err, "failed to list resources", "kind", kind)
			return nil, err
		}
		resources = append(resources, list.Items...)
	}
	return resources, nil
}

func (c *controller) runEngineValidation(logger logr.Logger, policy kyvernov1.PolicyInterface, resource unstructured.Unstructured, excludeGroupRole []string, namespaceLabels map[string]string) (*response.EngineResponse, error) {
	ctx := context.NewContext()
	err := ctx.AddResource(resource.Object)
	if err != nil {
		return nil, err
	}
	err = ctx.AddNamespace(resource.GetNamespace())
	if err != nil {
		return nil, err
	}
	if err := ctx.AddImageInfos(&resource); err != nil {
		return nil, err
	}
	// TODO: mutation
	// engineResponseMutation, err = mutation(policy, resource, logger, ctx, namespaceLabels)
	// if err != nil {
	// 	logger.Error(err, "failed to process mutation rule")
	// }

	policyCtx := &engine.PolicyContext{
		Policy:           policy,
		NewResource:      resource,
		ExcludeGroupRole: excludeGroupRole,
		JSONContext:      ctx,
		Client:           c.client,
		NamespaceLabels:  namespaceLabels,
	}

	return engine.Validate(policyCtx), nil
}

func (c *controller) runPolicyScan(logger logr.Logger, resource unstructured.Unstructured, policy kyvernov1.PolicyInterface) (*response.EngineResponse, error) {
	return c.runEngineValidation(logger, policy, resource, nil, nil)
}

func calculateSummary(results []policyreportv1alpha2.PolicyReportResult) (summary policyreportv1alpha2.PolicyReportSummary) {
	for _, res := range results {
		switch string(res.Result) {
		case policyreportv1alpha2.StatusPass:
			summary.Pass++
		case policyreportv1alpha2.StatusFail:
			summary.Fail++
		case policyreportv1alpha2.StatusWarn:
			summary.Warn++
		case policyreportv1alpha2.StatusError:
			summary.Error++
		case policyreportv1alpha2.StatusSkip:
			summary.Skip++
		}
	}
	return
}

func toPolicyResult(status response.RuleStatus) policyreportv1alpha2.PolicyResult {
	switch status {
	case response.RuleStatusPass:
		return policyreportv1alpha2.StatusPass
	case response.RuleStatusFail:
		return policyreportv1alpha2.StatusFail
	case response.RuleStatusError:
		return policyreportv1alpha2.StatusError
	case response.RuleStatusWarn:
		return policyreportv1alpha2.StatusWarn
	case response.RuleStatusSkip:
		return policyreportv1alpha2.StatusSkip
	}

	return ""
}

func (c *controller) runScan(logger logr.Logger) error {
	logger.Info("start scan ...")
	defer logger.Info("stop scan ...")
	policies, err := c.fetchBackgroundPolicies(logger)
	if err != nil {
		return err
	}
	resources, err := c.fetchResources(logger, policies...)
	if err != nil {
		return err
	}
	// run validation for all resources against all policies
	for _, resource := range resources {
		var responses []*response.EngineResponse
		for _, policy := range policies {
			if response, err := c.runPolicyScan(logger, resource, policy); err != nil {
				return err
			} else {
				responses = append(responses, response)
			}
		}
		_, err := controllerutils.CreateOrUpdate(
			string(resource.GetUID()),
			c.rcrLister.ReportChangeRequests(resource.GetNamespace()),
			c.kyvernoClient.KyvernoV1alpha2().ReportChangeRequests(resource.GetNamespace()),
			func(obj *v1alpha2.ReportChangeRequest) error {
				obj.SetNamespace(resource.GetNamespace())
				controllerutils.SetLabel(obj, kyvernov1.ManagedByLabel, kyvernov1.KyvernoAppValue)
				controllerutils.SetOwner(obj, resource.GetAPIVersion(), resource.GetKind(), resource.GetName(), resource.GetUID())
				for _, policy := range policies {
					key, _ := cache.MetaNamespaceKeyFunc(policy)
					controllerutils.SetLabel(obj, "scan.kyverno.io/"+key, policy.GetResourceVersion())
				}
				var results []policyreportv1alpha2.PolicyReportResult
				for _, response := range responses {
					for _, rule := range response.PolicyResponse.Rules {
						results = append(
							results,
							policyreportv1alpha2.PolicyReportResult{
								// TODO: incorrect
								Policy:  response.PolicyResponse.Policy.Name,
								Rule:    rule.Name,
								Message: rule.Message,
								Result:  toPolicyResult(rule.Status),
								Source:  kyvernov1.KyvernoAppValue,
								Timestamp: metav1.Timestamp{
									Seconds: time.Now().Unix(),
								},
								// Resources: []corev1.ObjectReference{
								// 	{
								// 		Kind:       resource.Kind,
								// 		Namespace:  resource.Namespace,
								// 		APIVersion: resource.APIVersion,
								// 		Name:       resource.Name,
								// 		UID:        types.UID(resource.UID),
								// 	},
								// },
								// Scored:   av.scored,
								// Category: av.category,
								// Severity: av.severity,

								// if result.Result == "fail" && !av.scored {
								// 	result.Result = "warn"
								// }
							},
						)
					}
				}
				obj.Results = results
				obj.Summary = calculateSummary(results)
				return nil
			},
		)
		if err != nil {
			logger.Error(err, "failed to create or update rcr")
		}
	}
	return nil
}

func (c *controller) reconcile(key, namespace, name string) error {
	logger := logger.WithValues("key", key, "namespace", namespace, "name", name)
	logger.Info("reconciling ...")
	return nil
	// return c.runScan(logger)
}

func (c *controller) ticker(stopChan <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.runScan(logger)
		case <-stopChan:
			return
		}
	}
}
