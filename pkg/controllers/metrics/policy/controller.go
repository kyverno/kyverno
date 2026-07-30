package policy

import (
	"context"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/metrics"
	policyChangesMetric "github.com/kyverno/kyverno/pkg/metrics/policychanges"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.opentelemetry.io/otel/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

const (
	ControllerName = "policy-metrics-controller"
	Workers        = 3
)

type policyEvent struct {
	eventType policyChangesMetric.PolicyChangeType
	old       kyvernov1.PolicyInterface
	cur       kyvernov1.PolicyInterface
}

type controller struct {
	ruleInfo metrics.PolicyRuleMetrics

	// listers
	cpolLister kyvernov1listers.ClusterPolicyLister
	polLister  kyvernov1listers.PolicyLister

	queue workqueue.TypedRateLimitingInterface[policyEvent]
}

func NewController(
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
) controllers.Controller {
	c := &controller{
		ruleInfo:   metrics.GetPolicyInfoMetrics(),
		cpolLister: cpolInformer.Lister(),
		polLister:  polInformer.Lister(),
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[policyEvent](),
			workqueue.TypedRateLimitingQueueConfig[policyEvent]{Name: ControllerName},
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
	return c
}

func (c *controller) Run(ctx context.Context, workers int) {
	logger.Info("starting")
	defer logger.Info("shutting down")
	defer c.queue.ShutDown()

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.worker, time.Second)
	}

	<-ctx.Done()
}

func (c *controller) worker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *controller) processNextWorkItem(ctx context.Context) bool {
	event, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(event)

	err := c.reconcile(ctx, event)
	if err == nil {
		c.queue.Forget(event)
		return true
	}

	logger.Error(err, "failed to process event", "event", event.eventType, "policy", event.cur.GetName())
	c.queue.AddRateLimited(event)
	return true
}

func (c *controller) reconcile(ctx context.Context, event policyEvent) error {
	switch event.eventType {
	case policyChangesMetric.PolicyCreated:
		c.registerPolicyChangesMetricAddPolicy(ctx, logger, event.cur)
	case policyChangesMetric.PolicyUpdated:
		c.registerPolicyChangesMetricUpdatePolicy(ctx, logger, event.old, event.cur)
	case policyChangesMetric.PolicyDeleted:
		c.registerPolicyChangesMetricDeletePolicy(ctx, logger, event.cur)
	}
	return nil
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

func (c *controller) addPolicy(obj interface{}) {
	if p, ok := obj.(*kyvernov1.ClusterPolicy); ok {
		c.queue.Add(policyEvent{eventType: policyChangesMetric.PolicyCreated, cur: p})
	}
}

func (c *controller) updatePolicy(old, cur interface{}) {
	oldP, ok1 := old.(*kyvernov1.ClusterPolicy)
	curP, ok2 := cur.(*kyvernov1.ClusterPolicy)
	if ok1 && ok2 {
		c.queue.Add(policyEvent{eventType: policyChangesMetric.PolicyUpdated, old: oldP, cur: curP})
	}
}

func (c *controller) deletePolicy(obj interface{}) {
	if p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy); ok {
		c.queue.Add(policyEvent{eventType: policyChangesMetric.PolicyDeleted, cur: p})
	} else {
		logger.Info("Failed to get deleted object", "obj", obj)
	}
}

func (c *controller) addNsPolicy(obj interface{}) {
	if p, ok := obj.(*kyvernov1.Policy); ok {
		c.queue.Add(policyEvent{eventType: policyChangesMetric.PolicyCreated, cur: p})
	}
}

func (c *controller) updateNsPolicy(old, cur interface{}) {
	oldP, ok1 := old.(*kyvernov1.Policy)
	curP, ok2 := cur.(*kyvernov1.Policy)
	if ok1 && ok2 {
		c.queue.Add(policyEvent{eventType: policyChangesMetric.PolicyUpdated, old: oldP, cur: curP})
	}
}

func (c *controller) deleteNsPolicy(obj interface{}) {
	if p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.Policy); ok {
		c.queue.Add(policyEvent{eventType: policyChangesMetric.PolicyDeleted, cur: p})
	} else {
		logger.Info("Failed to get deleted object", "obj", obj)
	}
}
