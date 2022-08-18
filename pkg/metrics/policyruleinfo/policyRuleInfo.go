package policyruleinfo

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/utils"
)

func registerPolicyRuleInfoMetric(
	m *metrics.MetricsConfig,
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
	includeNamespaces, excludeNamespaces := m.Config.GetIncludeNamespaces(), m.Config.GetExcludeNamespaces()
	if (policyNamespace != "" && policyNamespace != "-") && utils.ContainsString(excludeNamespaces, policyNamespace) {
		m.Log.V(2).Info(fmt.Sprintf("Skipping the registration of kyverno_policy_rule_info_total metric as the operation belongs to the namespace '%s' which is one of 'namespaces.exclude' %+v in values.yaml", policyNamespace, excludeNamespaces))
		return nil
	}
	if (policyNamespace != "" && policyNamespace != "-") && len(includeNamespaces) > 0 && !utils.ContainsString(includeNamespaces, policyNamespace) {
		m.Log.V(2).Info(fmt.Sprintf("Skipping the registration of kyverno_policy_rule_info_total metric as the operation belongs to the namespace '%s' which is not one of 'namespaces.include' %+v in values.yaml", policyNamespace, includeNamespaces))
		return nil
	}
	if policyType == metrics.Cluster {
		policyNamespace = "-"
	}
	status := "false"
	if ready {
		status = "true"
	}
	m.RecordPolicyRuleInfo(policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, ruleName, ruleType, status, metricValue)

	return nil
}

func AddPolicy(m *metrics.MetricsConfig, policy kyvernov1.PolicyInterface) error {
	name, namespace, policyType, backgroundMode, validationMode, err := metrics.GetPolicyInfos(policy)
	if err != nil {
		return err
	}
	ready := policy.IsReady()
	for _, rule := range autogen.ComputeRules(policy) {
		ruleName := rule.Name
		ruleType := metrics.ParseRuleType(rule)
		if err = registerPolicyRuleInfoMetric(m, validationMode, policyType, backgroundMode, namespace, name, ruleName, ruleType, PolicyRuleCreated, ready); err != nil {
			return err
		}
	}
	return nil
}

func RemovePolicy(m *metrics.MetricsConfig, policy kyvernov1.PolicyInterface) error {
	name, namespace, policyType, backgroundMode, validationMode, err := metrics.GetPolicyInfos(policy)
	if err != nil {
		return err
	}
	ready := policy.IsReady()
	for _, rule := range autogen.ComputeRules(policy) {
		ruleName := rule.Name
		ruleType := metrics.ParseRuleType(rule)
		if err = registerPolicyRuleInfoMetric(m, validationMode, policyType, backgroundMode, namespace, name, ruleName, ruleType, PolicyRuleDeleted, ready); err != nil {
			return err
		}
	}
	return nil
}
