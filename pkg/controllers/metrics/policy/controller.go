package policy

import (
	"context"

	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type controller struct {
	// config
	metricsConfig metrics.MetricsConfigManager
}

// TODO: this is a very strange controller, it only processes events, this should be changed to a real controller
// but this is difficult as we currently can't remove existing metrics. To be reviewed when we implement a more
// solid metrics system.
func NewController(metricsConfig metrics.MetricsConfigManager, cpolInformer kyvernov1informers.ClusterPolicyInformer, polInformer kyvernov1informers.PolicyInformer) {
	meterProvider := global.MeterProvider()
	meter := meterProvider.Meter(metrics.MeterName)

	// Register Async Callbacks
	policyRuleInfoMetric, err := meter.AsyncFloat64().Gauge("kyverno_policy_rule_info_total", instrument.WithDescription("can be used to track the info of the rules or/and policies present in the cluster. 0 means the rule doesn't exist and has been deleted, 1 means the rule is currently existent in the cluster"))
	if err != nil {
		logger.Error(err, "Failed to create instrument, kyverno_policy_rule_info_total")
	}
	// c := controller{
	// 	metricsConfig: metricsConfig,
	// }
	if policyRuleInfoMetric != nil {
		err := meter.RegisterCallback([]instrument.Asynchronous{policyRuleInfoMetric}, func(ctx context.Context) {
			pols, err := polInformer.Lister().Policies(metav1.NamespaceAll).List(labels.Everything())
			if err != nil {
				return
			}
			for _, policy := range pols {
				policyRuleInfoMetric.Observe(
					ctx,
					1,
					// attribute.String("policy_validation_mode", string(policyValidationMode)),
					// attribute.String("policy_type", string(policyType)),
					// attribute.String("policy_background_mode", string(policyBackgroundMode)),
					attribute.String("policy_namespace", policy.Namespace),
					attribute.String("policy_name", policy.Name),
					// attribute.String("rule_name", ruleName),
					// attribute.String("rule_type", string(ruleType)),
					attribute.Bool("status_ready", policy.IsReady()),
				)
			}
			cpols, err := cpolInformer.Lister().List(labels.Everything())
			if err != nil {
				return
			}
			for _, policy := range cpols {
				policyRuleInfoMetric.Observe(
					ctx,
					1,
					// attribute.String("policy_validation_mode", string(policyValidationMode)),
					// attribute.String("policy_type", string(policyType)),
					// attribute.String("policy_background_mode", string(policyBackgroundMode)),
					attribute.String("policy_namespace", policy.Namespace),
					attribute.String("policy_name", policy.Name),
					// attribute.String("rule_name", ruleName),
					// attribute.String("rule_type", string(ruleType)),
					attribute.Bool("status_ready", policy.IsReady()),
				)
			}
		})
		if err != nil {
			logger.Error(err, "Failed to register callback")
		}
		// i++
		// 	commonLabels := []attribute.KeyValue{
		// 		attribute.Int("tralala", i),
		// 	}
		// 	m.policyRuleInfoMetric.Observe(ctx, 11, commonLabels...)
		// })
	}
}

// func (c *controller) addPolicy(obj interface{}) {
// 	p := obj.(*kyvernov1.ClusterPolicy)
// 	// register kyverno_policy_rule_info_total metric concurrently
// 	go c.registerPolicyRuleInfoMetricAddPolicy(context.TODO(), logger, p)
// 	// register kyverno_policy_changes_total metric concurrently
// 	go c.registerPolicyChangesMetricAddPolicy(context.TODO(), logger, p)
// }

// func (c *controller) updatePolicy(old, cur interface{}) {
// 	oldP, curP := old.(*kyvernov1.ClusterPolicy), cur.(*kyvernov1.ClusterPolicy)
// 	// register kyverno_policy_rule_info_total metric concurrently
// 	go c.registerPolicyRuleInfoMetricUpdatePolicy(context.TODO(), logger, oldP, curP)
// 	// register kyverno_policy_changes_total metric concurrently
// 	go c.registerPolicyChangesMetricUpdatePolicy(context.TODO(), logger, oldP, curP)
// }

// func (c *controller) deletePolicy(obj interface{}) {
// 	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
// 	if !ok {
// 		logger.Info("Failed to get deleted object", "obj", obj)
// 		return
// 	}
// 	// register kyverno_policy_rule_info_total metric concurrently
// 	go c.registerPolicyRuleInfoMetricDeletePolicy(context.TODO(), logger, p)
// 	// register kyverno_policy_changes_total metric concurrently
// 	go c.registerPolicyChangesMetricDeletePolicy(context.TODO(), logger, p)
// }

// func (c *controller) addNsPolicy(obj interface{}) {
// 	p := obj.(*kyvernov1.Policy)
// 	// register kyverno_policy_rule_info_total metric concurrently
// 	go c.registerPolicyRuleInfoMetricAddPolicy(context.TODO(), logger, p)
// 	// register kyverno_policy_changes_total metric concurrently
// 	go c.registerPolicyChangesMetricAddPolicy(context.TODO(), logger, p)
// }

// func (c *controller) updateNsPolicy(old, cur interface{}) {
// 	oldP, curP := old.(*kyvernov1.Policy), cur.(*kyvernov1.Policy)
// 	// register kyverno_policy_rule_info_total metric concurrently
// 	go c.registerPolicyRuleInfoMetricUpdatePolicy(context.TODO(), logger, oldP, curP)
// 	// register kyverno_policy_changes_total metric concurrently
// 	go c.registerPolicyChangesMetricUpdatePolicy(context.TODO(), logger, oldP, curP)
// }

// func (c *controller) deleteNsPolicy(obj interface{}) {
// 	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.Policy)
// 	if !ok {
// 		logger.Info("Failed to get deleted object", "obj", obj)
// 		return
// 	}
// 	// register kyverno_policy_rule_info_total metric concurrently
// 	go c.registerPolicyRuleInfoMetricDeletePolicy(context.TODO(), logger, p)
// 	// register kyverno_policy_changes_total metric concurrently
// 	go c.registerPolicyChangesMetricDeletePolicy(context.TODO(), logger, p)
// }
