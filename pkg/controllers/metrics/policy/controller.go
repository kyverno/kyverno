package policy

import (
	"context"
	"sync"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.opentelemetry.io/otel/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
)

type controller struct {
	ruleInfo metrics.PolicyRuleMetrics

	// listers
	cpolLister kyvernov1listers.ClusterPolicyLister
	polLister  kyvernov1listers.PolicyLister

	waitGroup *wait.Group

	mu               sync.RWMutex
	initMu           sync.Mutex
	policies         map[string]kyvernov1.PolicyInterface
	cacheInitialized bool
}

// TODO: this is a strange controller, it only processes events, this should be changed to a real controller.
func NewController(
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	waitGroup *wait.Group,
) {
	c := controller{
		ruleInfo:   metrics.GetPolicyInfoMetrics(),
		cpolLister: cpolInformer.Lister(),
		polLister:  polInformer.Lister(),
		waitGroup:  waitGroup,
		policies:   map[string]kyvernov1.PolicyInterface{},
	}
	if _, err := controllerutils.AddEventHandlers(cpolInformer.Informer(), c.addPolicy, c.updatePolicy, c.deletePolicy); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlers(polInformer.Informer(), c.addNsPolicy, c.updateNsPolicy, c.deleteNsPolicy); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if c.ruleInfo != nil {
		_, err := c.ruleInfo.RegisterCallback(c.report)
		if err != nil {
			logger.Error(err, "Failed to register callback")
		}
	}
}

func (c *controller) report(ctx context.Context, observer metric.Observer) error {
	if err := c.initializePolicyCache(); err != nil {
		return err
	}

	for _, policy := range c.policySnapshot() {
		if err := c.ruleInfo.RecordPolicyRuleInfo(ctx, policy, observer); err != nil {
			logger.Error(err, "failed to report policy metric", "policy", policy)
			return err
		}
	}
	return nil
}

func (c *controller) initializePolicyCache() error {
	c.mu.RLock()
	if c.cacheInitialized {
		c.mu.RUnlock()
		return nil
	}
	c.mu.RUnlock()

	c.initMu.Lock()
	defer c.initMu.Unlock()

	c.mu.RLock()
	if c.cacheInitialized {
		c.mu.RUnlock()
		return nil
	}
	c.mu.RUnlock()

	pols, err := c.polLister.Policies(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		logger.Error(err, "failed to list policies")
		return err
	}
	cpols, err := c.cpolLister.List(labels.Everything())
	if err != nil {
		logger.Error(err, "failed to list cluster policies")
		return err
	}

	policies := make(map[string]kyvernov1.PolicyInterface, len(pols)+len(cpols))
	for _, policy := range pols {
		policies[policyKey(policy)] = policy.CreateDeepCopy()
	}
	for _, policy := range cpols {
		policies[policyKey(policy)] = policy.CreateDeepCopy()
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.cacheInitialized {
		if c.policies == nil {
			c.policies = map[string]kyvernov1.PolicyInterface{}
		}
		for key, policy := range policies {
			if _, ok := c.policies[key]; !ok {
				c.policies[key] = policy
			}
		}
		c.cacheInitialized = true
	}
	return nil
}

func (c *controller) policySnapshot() []kyvernov1.PolicyInterface {
	c.mu.RLock()
	defer c.mu.RUnlock()

	policies := make([]kyvernov1.PolicyInterface, 0, len(c.policies))
	for _, policy := range c.policies {
		policies = append(policies, policy)
	}
	return policies
}

func (c *controller) storePolicy(policy kyvernov1.PolicyInterface) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.policies == nil {
		c.policies = map[string]kyvernov1.PolicyInterface{}
	}
	c.policies[policyKey(policy)] = policy.CreateDeepCopy()
}

func (c *controller) deletePolicyFromCache(policy kyvernov1.PolicyInterface) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.policies, policyKey(policy))
}

func policyKey(policy kyvernov1.PolicyInterface) string {
	return policy.GetNamespace() + "/" + policy.GetName()
}

func (c *controller) startRountine(routine func()) {
	if c.waitGroup == nil {
		go routine()
		return
	}
	c.waitGroup.Start(routine)
}

func (c *controller) addPolicy(obj interface{}) {
	p := obj.(*kyvernov1.ClusterPolicy)
	c.storePolicy(p)
	// register kyverno_policy_changes_total metric concurrently
	c.startRountine(func() { c.registerPolicyChangesMetricAddPolicy(context.TODO(), logger, p) })
}

func (c *controller) updatePolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.ClusterPolicy), cur.(*kyvernov1.ClusterPolicy)
	c.storePolicy(curP)
	// register kyverno_policy_changes_total metric concurrently
	c.startRountine(func() { c.registerPolicyChangesMetricUpdatePolicy(context.TODO(), logger, oldP, curP) })
}

func (c *controller) deletePolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}
	c.deletePolicyFromCache(p)
	// register kyverno_policy_changes_total metric concurrently
	c.startRountine(func() { c.registerPolicyChangesMetricDeletePolicy(context.TODO(), logger, p) })
}

func (c *controller) addNsPolicy(obj interface{}) {
	p := obj.(*kyvernov1.Policy)
	c.storePolicy(p)
	// register kyverno_policy_changes_total metric concurrently
	c.startRountine(func() { c.registerPolicyChangesMetricAddPolicy(context.TODO(), logger, p) })
}

func (c *controller) updateNsPolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.Policy), cur.(*kyvernov1.Policy)
	c.storePolicy(curP)
	// register kyverno_policy_changes_total metric concurrently
	c.startRountine(func() { c.registerPolicyChangesMetricUpdatePolicy(context.TODO(), logger, oldP, curP) })
}

func (c *controller) deleteNsPolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.Policy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}
	c.deletePolicyFromCache(p)
	// register kyverno_policy_changes_total metric concurrently
	c.startRountine(func() { c.registerPolicyChangesMetricDeletePolicy(context.TODO(), logger, p) })
}
