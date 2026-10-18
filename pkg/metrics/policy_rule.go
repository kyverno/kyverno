package metrics

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// PolicyRuleObservation holds the series a single policy contributes to the
// kyverno_policy_rule_info_total gauge. Building it once per policy event
// allows the scrape callback to replay it instead of walking the policy again.
type PolicyRuleObservation []metric.ObserveOption

func GetPolicyInfoMetrics() PolicyRuleMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.PolicyRuleMetrics()
}

type PolicyRuleMetrics interface {
	BuildPolicyRuleObservations(policy kyvernov1.PolicyInterface) (PolicyRuleObservation, error)
	RecordPolicyRuleObservations(obs PolicyRuleObservation, observer metric.Observer)
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

func (m *policyRuleMetrics) BuildPolicyRuleObservations(policy kyvernov1.PolicyInterface) (PolicyRuleObservation, error) {
	if m.infoMetric == nil {
		return nil, nil
	}

	name, namespace, policyType, backgroundMode, validationMode, err := GetPolicyInfos(policy)
	if err != nil {
		return nil, err
	}

	if !GetManager().Config().CheckNamespace(namespace) {
		return nil, nil
	}

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
	rules := autogen.Default.ComputeRules(policy, "")
	observations := make(PolicyRuleObservation, 0, len(rules))
	for _, rule := range rules {
		ruleType := ParseRuleType(rule)
		ruleAttributes := make([]attribute.KeyValue, 0, 2+len(policyAttributes))
		ruleAttributes = append(ruleAttributes,
			attribute.String("rule_name", rule.Name),
			attribute.String("rule_type", string(ruleType)),
		)
		observations = append(observations, metric.WithAttributes(append(ruleAttributes, policyAttributes...)...))
	}
	return observations, nil
}

func (m *policyRuleMetrics) RecordPolicyRuleObservations(obs PolicyRuleObservation, observer metric.Observer) {
	if m.infoMetric == nil {
		return
	}

	for _, o := range obs {
		observer.ObserveFloat64(m.infoMetric, 1, o)
	}
}

func (m *policyRuleMetrics) RegisterCallback(f metric.Callback) (metric.Registration, error) {
	if m.meter == nil {
		return nil, nil
	}

	m.callback = f
	return m.meter.RegisterCallback(f, m.infoMetric)
}
