package exception

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"github.com/kyverno/kyverno/pkg/logging"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// ControllerName is the name of the exception controller.
	ControllerName = "exception-controller"
	// Workers is the number of workers for this controller.
	Workers = 2

	maxRetries    = 10
	resyncPeriod  = 10 * time.Minute

	// prefixClusterPolicy is used as a queue key prefix for ClusterPolicy resources.
	prefixClusterPolicy = "cpol/"
	// prefixPolicy is used as a queue key prefix for namespaced Policy resources.
	prefixPolicy = "pol/"
)

var logger = logging.WithName(ControllerName)

type controller struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface
	engineClient  engineapi.Engine

	// listers
	cpolLister  kyvernov1listers.ClusterPolicyLister
	polLister   kyvernov1listers.PolicyLister
	polexLister kyvernov2listers.PolicyExceptionLister

	// config
	config config.Configuration
	jp     jmespath.Interface

	// queue
	queue workqueue.TypedRateLimitingInterface[any]
}

// NewController creates a new exception controller that auto-generates PolicyExceptions
// for existing resources that violate Enforce-mode policies.
func NewController(
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	eng engineapi.Engine,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polexInformer kyvernov2informers.PolicyExceptionInformer,
	configuration config.Configuration,
	jp jmespath.Interface,
) controllers.Controller {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: ControllerName},
	)

	c := &controller{
		client:        client,
		kyvernoClient: kyvernoClient,
		engineClient:  eng,
		cpolLister:    cpolInformer.Lister(),
		polLister:     polInformer.Lister(),
		polexLister:   polexInformer.Lister(),
		config:        configuration,
		jp:            jp,
		queue:         queue,
	}

	// Watch ClusterPolicies
	if _, err := controllerutils.AddEventHandlersT(
		cpolInformer.Informer(),
		c.addClusterPolicy,
		c.updateClusterPolicy,
		c.deleteClusterPolicy,
	); err != nil {
		logger.Error(err, "failed to register ClusterPolicy event handlers")
	}

	// Watch Policies
	if _, err := controllerutils.AddEventHandlersT(
		polInformer.Informer(),
		c.addPolicy,
		c.updatePolicy,
		c.deletePolicy,
	); err != nil {
		logger.Error(err, "failed to register Policy event handlers")
	}

	return c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile, c.resyncLoop)
}

// resyncLoop periodically re-enqueues all policies with generateExceptions enabled
// to clean up stale exceptions when violations are remediated.
func (c *controller) resyncLoop(ctx context.Context, logger logr.Logger) {
	ticker := time.NewTicker(resyncPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.enqueueAllExceptionPolicies(logger)
		}
	}
}

func (c *controller) enqueueAllExceptionPolicies(logger logr.Logger) {
	// Re-enqueue all ClusterPolicies with generateExceptions
	cpols, err := c.cpolLister.List(labels.Everything())
	if err != nil {
		logger.Error(err, "failed to list cluster policies for resync")
		return
	}
	for _, cpol := range cpols {
		if cpol.GetSpec().HasGenerateExceptions() {
			c.queue.Add(prefixClusterPolicy + cpol.GetName())
		}
	}

	// Re-enqueue all Policies with generateExceptions
	pols, err := c.polLister.List(labels.Everything())
	if err != nil {
		logger.Error(err, "failed to list policies for resync")
		return
	}
	for _, pol := range pols {
		if pol.GetSpec().HasGenerateExceptions() {
			c.queue.Add(prefixPolicy + pol.GetNamespace() + "/" + pol.GetName())
		}
	}
}

// Event handlers for ClusterPolicy
func (c *controller) addClusterPolicy(obj *kyvernov1.ClusterPolicy) {
	if obj.GetSpec().HasGenerateExceptions() {
		c.queue.Add(prefixClusterPolicy + obj.GetName())
	}
}

func (c *controller) updateClusterPolicy(oldObj, newObj *kyvernov1.ClusterPolicy) {
	if oldObj.GetResourceVersion() != newObj.GetResourceVersion() {
		if newObj.GetSpec().HasGenerateExceptions() || oldObj.GetSpec().HasGenerateExceptions() {
			c.queue.Add(prefixClusterPolicy + newObj.GetName())
		}
	}
}

func (c *controller) deleteClusterPolicy(obj *kyvernov1.ClusterPolicy) {
	c.queue.Add(prefixClusterPolicy + obj.GetName())
}

// Event handlers for Policy
func (c *controller) addPolicy(obj *kyvernov1.Policy) {
	if obj.GetSpec().HasGenerateExceptions() {
		c.queue.Add(prefixPolicy + obj.GetNamespace() + "/" + obj.GetName())
	}
}

func (c *controller) updatePolicy(oldObj, newObj *kyvernov1.Policy) {
	if oldObj.GetResourceVersion() != newObj.GetResourceVersion() {
		if newObj.GetSpec().HasGenerateExceptions() || oldObj.GetSpec().HasGenerateExceptions() {
			c.queue.Add(prefixPolicy + newObj.GetNamespace() + "/" + newObj.GetName())
		}
	}
}

func (c *controller) deletePolicy(obj *kyvernov1.Policy) {
	c.queue.Add(prefixPolicy + obj.GetNamespace() + "/" + obj.GetName())
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key string, namespace string, name string) error {
	// Determine if this is a ClusterPolicy or Policy from the key prefix
	var policy kyvernov1.PolicyInterface
	var policyName string
	var isClusterPolicy bool

	if len(key) > len(prefixClusterPolicy) && key[:len(prefixClusterPolicy)] == prefixClusterPolicy {
		policyName = key[len(prefixClusterPolicy):]
		isClusterPolicy = true
		cpol, err := c.cpolLister.Get(policyName)
		if err != nil {
			if errors.IsNotFound(err) {
				return c.cleanupExceptions(ctx, logger, policyName)
			}
			return err
		}
		policy = cpol
	} else if len(key) > len(prefixPolicy) && key[:len(prefixPolicy)] == prefixPolicy {
		rest := key[len(prefixPolicy):]
		ns, n, err := cache.SplitMetaNamespaceKey(rest)
		if err != nil {
			return err
		}
		policyName = rest
		pol, err := c.polLister.Policies(ns).Get(n)
		if err != nil {
			if errors.IsNotFound(err) {
				return c.cleanupExceptions(ctx, logger, policyName)
			}
			return err
		}
		policy = pol
	} else {
		logger.V(4).Info("unknown key format, skipping", "key", key)
		return nil
	}

	spec := policy.GetSpec()

	// If generateExceptions is disabled or there are no enforce validate rules, clean up
	if !spec.HasGenerateExceptions() || !spec.HasValidateEnforce() {
		return c.cleanupExceptions(ctx, logger, policyName)
	}

	logger.V(2).Info("reconciling policy for exception generation", "policy", policyName)

	// Scan existing resources for violations
	violatingOwners, ruleNames, err := c.findViolatingOwners(ctx, logger, policy)
	if err != nil {
		return fmt.Errorf("failed to find violating owners: %w", err)
	}

	// Build desired exceptions
	desiredExceptions := map[string]*kyvernov2.PolicyException{}
	for ownerKey, owner := range violatingOwners {
		polex := buildPolicyException(policyName, isClusterPolicy, ruleNames[ownerKey], owner)
		desiredExceptions[polex.Name] = polex
	}

	// Diff against existing auto-generated exceptions
	return c.syncExceptions(ctx, logger, policyName, desiredExceptions)
}

// findViolatingOwners scans resources matching the policy and returns the set of
// owner resources that have violations, along with the violated rule names.
func (c *controller) findViolatingOwners(
	ctx context.Context,
	logger logr.Logger,
	policy kyvernov1.PolicyInterface,
) (map[string]ownerInfo, map[string][]string, error) {
	owners := map[string]ownerInfo{}
	ownerRules := map[string][]string{}
	ruleNameSet := map[string]sets.Set[string]{}

	spec := policy.GetSpec()
	rules := autogen.Default.ComputeRules(policy, "")

	// Collect all kinds from validate rules
	kindsToCheck := sets.New[string]()
	for _, rule := range rules {
		if !rule.HasValidate() {
			continue
		}
		action := rule.Validation.FailureAction
		if action == nil {
			// Check policy-level default
			if !spec.ValidationFailureAction.Enforce() {
				continue
			}
		} else if !action.Enforce() {
			continue
		}
		kindsToCheck.Insert(rule.MatchResources.GetKinds()...)
	}

	for kind := range kindsToCheck {
		// List resources of this kind
		resources, err := c.client.ListResource(ctx, "", kind, "", nil)
		if err != nil {
			logger.V(3).Info("failed to list resources", "kind", kind, "error", err)
			continue
		}

		for _, resource := range resources.Items {
			// Evaluate policy against this resource
			policyCtx, err := engine.NewPolicyContext(c.jp, resource, kyvernov1.Create, nil, c.config)
			if err != nil {
				logger.V(4).Info("failed to create policy context", "resource", resource.GetName(), "error", err)
				continue
			}

			nsLabels := map[string]string{}
			if resource.GetNamespace() != "" {
				nsObj, err := c.client.GetResource(ctx, "v1", "Namespace", "", resource.GetNamespace())
				if err == nil && nsObj != nil {
					nsLabels = nsObj.GetLabels()
				}
			}

			policyCtx = policyCtx.
				WithNewResource(resource).
				WithPolicy(policy).
				WithNamespaceLabels(nsLabels)

			response := c.engineClient.Validate(ctx, policyCtx)

			if !response.IsFailed() {
				continue
			}

			// Collect failed rule names
			failedRules := sets.New[string]()
			for _, ruleResp := range response.PolicyResponse.Rules {
				if ruleResp.HasStatus(engineapi.RuleStatusFail) {
					failedRules.Insert(ruleResp.Name())
				}
			}

			if failedRules.Len() == 0 {
				continue
			}

			// Resolve owner
			owner, err := resolveRootOwner(ctx, c.client, resource)
			if err != nil {
				logger.V(4).Info("failed to resolve owner", "resource", resource.GetName(), "error", err)
				// Fall back to the resource itself
				owner = ownerInfo{
					kind:      resource.GetKind(),
					name:      resource.GetName(),
					namespace: resource.GetNamespace(),
				}
			}

			ownerKey := fmt.Sprintf("%s/%s/%s", owner.kind, owner.namespace, owner.name)
			owners[ownerKey] = owner
			if _, ok := ruleNameSet[ownerKey]; !ok {
				ruleNameSet[ownerKey] = sets.New[string]()
			}
			ruleNameSet[ownerKey] = ruleNameSet[ownerKey].Union(failedRules)
		}
	}

	// Convert rule name sets to slices
	for key, ruleSet := range ruleNameSet {
		ownerRules[key] = sets.List(ruleSet)
	}

	return owners, ownerRules, nil
}

// syncExceptions creates, updates, or deletes auto-generated PolicyExceptions to match
// the desired state.
func (c *controller) syncExceptions(
	ctx context.Context,
	logger logr.Logger,
	policyName string,
	desired map[string]*kyvernov2.PolicyException,
) error {
	// List existing auto-generated exceptions for this policy
	existing, err := c.listAutoGeneratedExceptions(policyName)
	if err != nil {
		return fmt.Errorf("failed to list existing exceptions: %w", err)
	}

	existingByName := map[string]*kyvernov2.PolicyException{}
	for i := range existing {
		existingByName[existing[i].Name] = existing[i]
	}

	// Create or update desired exceptions
	for name, polex := range desired {
		if _, exists := existingByName[name]; exists {
			// Already exists, skip (we don't update since the name is deterministic)
			delete(existingByName, name)
			continue
		}
		logger.V(2).Info("creating auto-generated exception", "name", name, "namespace", polex.Namespace)
		_, err := c.kyvernoClient.KyvernoV2().PolicyExceptions(polex.Namespace).Create(ctx, polex, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				continue
			}
			return fmt.Errorf("failed to create exception %s: %w", name, err)
		}
	}

	// Delete stale exceptions (exist but no longer desired)
	for name, polex := range existingByName {
		logger.V(2).Info("deleting stale auto-generated exception", "name", name, "namespace", polex.Namespace)
		err := c.kyvernoClient.KyvernoV2().PolicyExceptions(polex.Namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete exception %s: %w", name, err)
		}
	}

	return nil
}

// cleanupExceptions removes all auto-generated exceptions for the given policy.
func (c *controller) cleanupExceptions(ctx context.Context, logger logr.Logger, policyName string) error {
	existing, err := c.listAutoGeneratedExceptions(policyName)
	if err != nil {
		return err
	}
	for _, polex := range existing {
		logger.V(2).Info("cleaning up auto-generated exception", "name", polex.Name, "namespace", polex.Namespace)
		err := c.kyvernoClient.KyvernoV2().PolicyExceptions(polex.Namespace).Delete(ctx, polex.Name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// listAutoGeneratedExceptions returns all PolicyExceptions that were auto-generated
// for the given policy.
func (c *controller) listAutoGeneratedExceptions(policyName string) ([]*kyvernov2.PolicyException, error) {
	selector, err := labels.Parse(fmt.Sprintf("%s=true,%s=%s", LabelAutoGenerated, LabelSourcePolicy, policyName))
	if err != nil {
		return nil, err
	}
	return c.polexLister.List(selector)
}
