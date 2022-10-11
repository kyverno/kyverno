package policy

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
)

type controller struct {
	// config
	metricsConfig *metrics.MetricsConfig
}

// TODO: this is a very strange controller, it only processes events, this should be changed to a real controller
// but this is difficult as we currently can't remove existing metrics. To be reviewed when we implement a more
// solid metrics system.
func NewController(metricsConfig *metrics.MetricsConfig, cpolInformer kyvernov1informers.ClusterPolicyInformer, polInformer kyvernov1informers.PolicyInformer) {
	c := controller{
		metricsConfig: metricsConfig,
	}
	controllerutils.AddEventHandlers(cpolInformer.Informer(), c.addPolicy, c.updatePolicy, c.deletePolicy)
	controllerutils.AddEventHandlers(polInformer.Informer(), c.addNsPolicy, c.updateNsPolicy, c.deleteNsPolicy)
}

func (c *controller) addPolicy(obj interface{}) {
	p := obj.(*kyvernov1.ClusterPolicy)
	// register kyverno_policy_rule_info_total metric concurrently
	go c.registerPolicyRuleInfoMetricAddPolicy(logger, p)
	// register kyverno_policy_changes_total metric concurrently
	go c.registerPolicyChangesMetricAddPolicy(logger, p)
}

func (c *controller) updatePolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.ClusterPolicy), cur.(*kyvernov1.ClusterPolicy)
	// register kyverno_policy_rule_info_total metric concurrently
	go c.registerPolicyRuleInfoMetricUpdatePolicy(logger, oldP, curP)
	// register kyverno_policy_changes_total metric concurrently
	go c.registerPolicyChangesMetricUpdatePolicy(logger, oldP, curP)
}

func (c *controller) deletePolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}
	// register kyverno_policy_rule_info_total metric concurrently
	go c.registerPolicyRuleInfoMetricDeletePolicy(logger, p)
	// register kyverno_policy_changes_total metric concurrently
	go c.registerPolicyChangesMetricDeletePolicy(logger, p)
}

func (c *controller) addNsPolicy(obj interface{}) {
	p := obj.(*kyvernov1.Policy)
	// register kyverno_policy_rule_info_total metric concurrently
	go c.registerPolicyRuleInfoMetricAddPolicy(logger, p)
	// register kyverno_policy_changes_total metric concurrently
	go c.registerPolicyChangesMetricAddPolicy(logger, p)
}

func (c *controller) updateNsPolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.Policy), cur.(*kyvernov1.Policy)
	// register kyverno_policy_rule_info_total metric concurrently
	go c.registerPolicyRuleInfoMetricUpdatePolicy(logger, oldP, curP)
	// register kyverno_policy_changes_total metric concurrently
	go c.registerPolicyChangesMetricUpdatePolicy(logger, oldP, curP)
}

func (c *controller) deleteNsPolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.Policy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}
	// register kyverno_policy_rule_info_total metric concurrently
	go c.registerPolicyRuleInfoMetricDeletePolicy(logger, p)
	// register kyverno_policy_changes_total metric concurrently
	go c.registerPolicyChangesMetricDeletePolicy(logger, p)
}
