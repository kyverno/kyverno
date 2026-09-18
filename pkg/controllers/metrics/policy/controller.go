package policy

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.opentelemetry.io/otel/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

const (
	ControllerName = "policy-metrics"
	Workers        = 1
)

type PolicyChangeType int

const (
	PolicyAdded PolicyChangeType = iota
	PolicyUpdated
	PolicyDeleted
)

type policyMetricsTask struct {
	action    PolicyChangeType
	policy    kyvernov1.PolicyInterface
	oldPolicy kyvernov1.PolicyInterface
}

type controller struct {
	ruleInfo metrics.PolicyRuleMetrics

	// listers
	cpolLister kyvernov1listers.ClusterPolicyLister
	polLister  kyvernov1listers.PolicyLister

	queue workqueue.TypedRateLimitingInterface[policyMetricsTask]
}

func NewController(
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
) controllers.Controller {
	c := controller{
		ruleInfo:   metrics.GetPolicyInfoMetrics(),
		cpolLister: cpolInformer.Lister(),
		polLister:  polInformer.Lister(),
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[policyMetricsTask](),
			workqueue.TypedRateLimitingQueueConfig[policyMetricsTask]{Name: ControllerName},
		),
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
	return &c
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

func (c *controller) Run(ctx context.Context, workers int) {
	logger.V(2).Info("starting ...")
	defer logger.V(2).Info("stopped")
	defer c.queue.ShutDown()

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.worker, 0)
	}
	<-ctx.Done()
}

func (c *controller) worker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *controller) processNextWorkItem(ctx context.Context) bool {
	task, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(task)

	switch task.action {
	case PolicyAdded:
		c.registerPolicyChangesMetricAddPolicy(ctx, logger, task.policy)
	case PolicyUpdated:
		c.registerPolicyChangesMetricUpdatePolicy(ctx, logger, task.oldPolicy, task.policy)
	case PolicyDeleted:
		c.registerPolicyChangesMetricDeletePolicy(ctx, logger, task.policy)
	}

	c.queue.Forget(task)
	return true
}

func (c *controller) addPolicy(obj interface{}) {
	p := obj.(*kyvernov1.ClusterPolicy)
	c.queue.Add(policyMetricsTask{action: PolicyAdded, policy: p})
}

func (c *controller) updatePolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.ClusterPolicy), cur.(*kyvernov1.ClusterPolicy)
	c.queue.Add(policyMetricsTask{action: PolicyUpdated, oldPolicy: oldP, policy: curP})
}

func (c *controller) deletePolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}
	c.queue.Add(policyMetricsTask{action: PolicyDeleted, policy: p})
}

func (c *controller) addNsPolicy(obj interface{}) {
	p := obj.(*kyvernov1.Policy)
	c.queue.Add(policyMetricsTask{action: PolicyAdded, policy: p})
}

func (c *controller) updateNsPolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.Policy), cur.(*kyvernov1.Policy)
	c.queue.Add(policyMetricsTask{action: PolicyUpdated, oldPolicy: oldP, policy: curP})
}

func (c *controller) deleteNsPolicy(obj interface{}) {
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.Policy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}
	c.queue.Add(policyMetricsTask{action: PolicyDeleted, policy: p})
}
