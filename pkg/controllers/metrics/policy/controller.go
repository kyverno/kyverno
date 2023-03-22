package policy

import (
	"context"
	"sync"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type controller struct {
	metricsConfig metrics.MetricsConfigManager
	ruleInfo      instrument.Float64ObservableGauge

	// listers
	cpolLister kyvernov1listers.ClusterPolicyLister
	polLister  kyvernov1listers.PolicyLister

	waitGroup *sync.WaitGroup
}

// TODO: this is a strange controller, it only processes events, this should be changed to a real controller.
func NewController(
	metricsConfig metrics.MetricsConfigManager,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	waitGroup *sync.WaitGroup,
) {
	meterProvider := global.MeterProvider()
	meter := meterProvider.Meter(metrics.MeterName)
	policyRuleInfoMetric, err := meter.Float64ObservableGauge(
		"kyverno_policy_rule_info_total",
		instrument.WithDescription("can be used to track the info of the rules or/and policies present in the cluster. 0 means the rule doesn't exist and has been deleted, 1 means the rule is currently existent in the cluster"),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, kyverno_policy_rule_info_total")
	}
	c := controller{
		metricsConfig: metricsConfig,
		ruleInfo:      policyRuleInfoMetric,
		cpolLister:    cpolInformer.Lister(),
		polLister:     polInformer.Lister(),
		waitGroup:     waitGroup,
	}
	controllerutils.AddEventHandlers(cpolInformer.Informer(), c.addPolicy, c.updatePolicy, c.deletePolicy)
	controllerutils.AddEventHandlers(polInformer.Informer(), c.addNsPolicy, c.updateNsPolicy, c.deleteNsPolicy)
	if c.ruleInfo != nil {
		_, err := meter.RegisterCallback(c.report, c.ruleInfo)
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
		err := c.reportPolicy(ctx, policy, observer)
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
		err := c.reportPolicy(ctx, policy, observer)
		if err != nil {
			logger.Error(err, "failed to report policy metric", "policy", policy)
			return err
		}
	}
	return nil
}

func (c *controller) reportPolicy(ctx context.Context, policy kyvernov1.PolicyInterface, observer metric.Observer) error {
	name, namespace, policyType, backgroundMode, validationMode, err := metrics.GetPolicyInfos(policy)
	if err != nil {
		return err
	}
	if c.metricsConfig.Config().CheckNamespace(namespace) {
		if policyType == metrics.Cluster {
			namespace = "-"
		}
		policyAttributes := []attribute.KeyValue{
			attribute.String("policy_namespace", namespace),
			attribute.String("policy_name", name),
			attribute.Bool("status_ready", policy.IsReady()),
			attribute.String("policy_validation_mode", string(validationMode)),
			attribute.String("policy_type", string(policyType)),
			attribute.String("policy_background_mode", string(backgroundMode)),
		}
		for _, rule := range autogen.ComputeRules(policy) {
			ruleType := metrics.ParseRuleType(rule)
			ruleAttributes := []attribute.KeyValue{
				attribute.String("rule_name", rule.Name),
				attribute.String("rule_type", string(ruleType)),
			}
			observer.ObserveFloat64(c.ruleInfo, 1, append(ruleAttributes, policyAttributes...)...)
		}
	}
	return nil
}

func (c *controller) startRountine(routine func()) {
	c.waitGroup.Add(1)
	go func() {
		defer c.waitGroup.Done()
		routine()
	}()
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
