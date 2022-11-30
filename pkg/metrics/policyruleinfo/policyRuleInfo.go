package policyruleinfo

import (
	"context"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/metrics"
)

func registerPolicyRuleInfoMetric(
	ctx context.Context,
	m metrics.MetricsConfigManager,
	policyValidationMode metrics.PolicyValidationMode,
	policyType metrics.PolicyType,
	policyBackgroundMode metrics.PolicyBackgroundMode,
	policyNamespace, policyName, ruleName string,
	ruleType metrics.RuleType,
	metricChangeType PolicyRuleInfoMetricChangeType,
	ready bool,
) error {
	var metricValue float64
	switch metricChangeType {
	case PolicyRuleCreated:
		metricValue = float64(1)
	case PolicyRuleDeleted:
		metricValue = float64(0)
	default:
		return fmt.Errorf("unknown metric change type found:  %s", metricChangeType)
	}
	if m.Config().CheckNamespace(policyNamespace) {
		if policyType == metrics.Cluster {
			policyNamespace = "-"
		}
		status := "false"
		if ready {
			status = "true"
		}
		m.RecordPolicyRuleInfo(ctx, policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, ruleName, ruleType, status, metricValue)
	}
	return nil
}

func AddPolicy(ctx context.Context, m metrics.MetricsConfigManager, policy kyvernov1.PolicyInterface) error {
	name, namespace, policyType, backgroundMode, validationMode, err := metrics.GetPolicyInfos(policy)
	if err != nil {
		return err
	}
	ready := policy.IsReady()
	for _, rule := range autogen.ComputeRules(policy) {
		ruleName := rule.Name
		ruleType := metrics.ParseRuleType(rule)
		if err = registerPolicyRuleInfoMetric(ctx, m, validationMode, policyType, backgroundMode, namespace, name, ruleName, ruleType, PolicyRuleCreated, ready); err != nil {
			return err
		}
	}
	return nil
}

func RemovePolicy(ctx context.Context, m metrics.MetricsConfigManager, policy kyvernov1.PolicyInterface) error {
	name, namespace, policyType, backgroundMode, validationMode, err := metrics.GetPolicyInfos(policy)
	if err != nil {
		return err
	}
	ready := policy.IsReady()
	for _, rule := range autogen.ComputeRules(policy) {
		ruleName := rule.Name
		ruleType := metrics.ParseRuleType(rule)
		if err = registerPolicyRuleInfoMetric(ctx, m, validationMode, policyType, backgroundMode, namespace, name, ruleName, ruleType, PolicyRuleDeleted, ready); err != nil {
			return err
		}
	}
	return nil
}
