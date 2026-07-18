package policy

import (
	"context"
	"sync"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

type controller struct {
	ruleInfo metrics.PolicyRuleMetrics

	mu       sync.RWMutex
	policies map[types.UID]kyvernov1.PolicyInterface

	waitGroup *wait.Group
}

func NewController(
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	waitGroup *wait.Group,
) {
	c := controller{
		ruleInfo:  metrics.GetPolicyInfoMetrics(),
		policies:  make(map[types.UID]kyvernov1.PolicyInterface),
		waitGroup: waitGroup,
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
	c.mu.RLock()
	snapshot := make([]kyvernov1.PolicyInterface, 0, len(c.policies))
	for _, p := range c.policies {
		snapshot = append(snapshot, p)
	}
	c.mu.RUnlock()

	for _, policy := range snapshot {
		if err := c.ruleInfo.RecordPolicyRuleInfo(ctx, policy, observer); err != nil {
			logger.Error(err, "failed to report policy metric", "policy", policy)
			return err
		}
	}
	return nil
}

func (c *controller) startRountine(routine func()) {
	c.waitGroup.Start(routine)
}

func (c *controller) addPolicy(obj interface{}) {
	p := obj.(*kyvernov1.ClusterPolicy)
	c.mu.Lock()
	c.policies[p.GetUID()] = p
	c.mu.Unlock()
	c.startRountine(func() { c.registerPolicyChangesMetricAddPolicy(context.TODO(), logger, p) })
}

func (c *controller) updatePolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.ClusterPolicy), cur.(*kyvernov1.ClusterPolicy)
	c.mu.Lock()
	c.policies[curP.GetUID()] = curP
	c.mu.Unlock()
	c.startRountine(func() { c.registerPolicyChangesMetricUpdatePolicy(context.TODO(), logger, oldP, curP) })
}

func (c *controller) deletePolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}
	c.mu.Lock()
	delete(c.policies, p.GetUID())
	c.mu.Unlock()
	c.startRountine(func() { c.registerPolicyChangesMetricDeletePolicy(context.TODO(), logger, p) })
}

func (c *controller) addNsPolicy(obj interface{}) {
	p := obj.(*kyvernov1.Policy)
	c.mu.Lock()
	c.policies[p.GetUID()] = p
	c.mu.Unlock()
	c.startRountine(func() { c.registerPolicyChangesMetricAddPolicy(context.TODO(), logger, p) })
}

func (c *controller) updateNsPolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.Policy), cur.(*kyvernov1.Policy)
	c.mu.Lock()
	c.policies[curP.GetUID()] = curP
	c.mu.Unlock()
	c.startRountine(func() { c.registerPolicyChangesMetricUpdatePolicy(context.TODO(), logger, oldP, curP) })
}

func (c *controller) deleteNsPolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.Policy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}
	c.mu.Lock()
	delete(c.policies, p.GetUID())
	c.mu.Unlock()
	c.startRountine(func() { c.registerPolicyChangesMetricDeletePolicy(context.TODO(), logger, p) })
}
