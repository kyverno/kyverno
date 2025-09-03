package policy

import (
	"context"

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
	pols, err := c.polLister.Policies(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		logger.Error(err, "failed to list policies")
		return err
	}
	for _, policy := range pols {
		err := c.ruleInfo.RecordPolicyRuleInfo(ctx, policy, observer)
		if err != nil {
			logger.Error(err, "failed to report policy metric", "policy", policy)
			return err
		}
	}
	cpols, err := c.cpolLister.List(labels.Everything())
	if err != nil {
		logger.Error(err, "failed to list cluster policies")
		return err
	}
	for _, policy := range cpols {
		err := c.ruleInfo.RecordPolicyRuleInfo(ctx, policy, observer)
		if err != nil {
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
	// register kyverno_policy_changes_total metric concurrently
	c.startRountine(func() { c.registerPolicyChangesMetricAddPolicy(context.TODO(), logger, p) })
}

func (c *controller) updatePolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.ClusterPolicy), cur.(*kyvernov1.ClusterPolicy)
	// register kyverno_policy_changes_total metric concurrently
	c.startRountine(func() { c.registerPolicyChangesMetricUpdatePolicy(context.TODO(), logger, oldP, curP) })
}

func (c *controller) deletePolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}
	// register kyverno_policy_changes_total metric concurrently
	c.startRountine(func() { c.registerPolicyChangesMetricDeletePolicy(context.TODO(), logger, p) })
}

func (c *controller) addNsPolicy(obj interface{}) {
	p := obj.(*kyvernov1.Policy)
	// register kyverno_policy_changes_total metric concurrently
	c.startRountine(func() { c.registerPolicyChangesMetricAddPolicy(context.TODO(), logger, p) })
}

func (c *controller) updateNsPolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.Policy), cur.(*kyvernov1.Policy)
	// register kyverno_policy_changes_total metric concurrently
	c.startRountine(func() { c.registerPolicyChangesMetricUpdatePolicy(context.TODO(), logger, oldP, curP) })
}

func (c *controller) deleteNsPolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.Policy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}
	// register kyverno_policy_changes_total metric concurrently
	c.startRountine(func() { c.registerPolicyChangesMetricDeletePolicy(context.TODO(), logger, p) })
}
