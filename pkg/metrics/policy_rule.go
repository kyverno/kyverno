package metrics

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func GetPolicyInfoMetrics() PolicyRuleMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.PolicyRuleMetrics()
}

type PolicyRuleMetrics interface {
	RecordPolicyRuleInfo(ctx context.Context, policy kyvernov1.PolicyInterface, observer metric.Observer) error
	RegisterCallback(f metric.Callback) (metric.Registration, error)
}

type policyRuleMetrics struct {
	infoMetric metric.Float64ObservableGauge
	meter      metric.Meter
	callback   metric.Callback

	logger logr.Logger
}

func (m *policyRuleMetrics) init(meter metric.Meter) {
	var err error

	m.infoMetric, err = meter.Float64ObservableGauge(
		"kyverno_policy_rule_info_total",
		metric.WithDescription("can be used to track the info of the rules or/and policies present in the cluster. 0 means the rule doesn't exist and has been deleted, 1 means the rule is currently existent in the cluster"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_policy_rule_info_total")
	}

	m.meter = meter

	if m.callback != nil {
		if _, err := m.meter.RegisterCallback(m.callback, m.infoMetric); err != nil {
			m.logger.Error(err, "failed to register callback for policy rule info metric")
		}
	}
}

func (m *policyRuleMetrics) RecordPolicyRuleInfo(ctx context.Context, policy kyvernov1.PolicyInterface, observer metric.Observer) error {
	if m.infoMetric == nil {
		return nil
	}

	name, namespace, policyType, backgroundMode, validationMode, err := GetPolicyInfos(policy)
	if err != nil {
		return err
	}

	if GetManager().Config().CheckNamespace(namespace) {
		if policyType == Cluster {
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
		for _, rule := range autogen.Default.ComputeRules(policy, "") {
			ruleType := ParseRuleType(rule)
			ruleAttributes := []attribute.KeyValue{
				attribute.String("rule_name", rule.Name),
				attribute.String("rule_type", string(ruleType)),
			}
			observer.ObserveFloat64(m.infoMetric, 1, metric.WithAttributes(append(ruleAttributes, policyAttributes...)...))
		}
	}
	return nil
}

func (m *policyRuleMetrics) RegisterCallback(f metric.Callback) (metric.Registration, error) {
	if m.meter == nil {
		return nil, nil
	}

	m.callback = f
	return m.meter.RegisterCallback(f, m.infoMetric)
}
