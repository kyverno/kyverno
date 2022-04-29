package policyruleinfo

import (
	"fmt"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/utils"
	prom "github.com/prometheus/client_golang/prometheus"
)

func registerPolicyRuleInfoMetric(
	pc *metrics.PromConfig,
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
	includeNamespaces, excludeNamespaces := pc.Config.GetIncludeNamespaces(), pc.Config.GetExcludeNamespaces()
	if (policyNamespace != "" && policyNamespace != "-") && utils.ContainsString(excludeNamespaces, policyNamespace) {
		metrics.Logger().Info(fmt.Sprintf("Skipping the registration of kyverno_policy_rule_info_total metric as the operation belongs to the namespace '%s' which is one of 'namespaces.exclude' %+v in values.yaml", policyNamespace, excludeNamespaces))
		return nil
	}
	if (policyNamespace != "" && policyNamespace != "-") && len(includeNamespaces) > 0 && !utils.ContainsString(includeNamespaces, policyNamespace) {
		metrics.Logger().Info(fmt.Sprintf("Skipping the registration of kyverno_policy_rule_info_total metric as the operation belongs to the namespace '%s' which is not one of 'namespaces.include' %+v in values.yaml", policyNamespace, includeNamespaces))
		return nil
	}
	if policyType == metrics.Cluster {
		policyNamespace = "-"
	}
	status := "false"
	if ready {
		status = "true"
	}
	pc.Metrics.PolicyRuleInfo.With(prom.Labels{
		"policy_validation_mode": string(policyValidationMode),
		"policy_type":            string(policyType),
		"policy_background_mode": string(policyBackgroundMode),
		"policy_namespace":       policyNamespace,
		"policy_name":            policyName,
		"rule_name":              ruleName,
		"rule_type":              string(ruleType),
		"status_ready":           status,
	}).Set(metricValue)
	return nil
}

func AddPolicy(pc *metrics.PromConfig, policy kyverno.PolicyInterface) error {
	name, namespace, policyType, backgroundMode, validationMode, err := metrics.GetPolicyInfos(policy)
	if err != nil {
		return err
	}
	ready := policy.IsReady()
	for _, rule := range autogen.ComputeRules(policy) {
		ruleName := rule.Name
		ruleType := metrics.ParseRuleType(rule)
		if err = registerPolicyRuleInfoMetric(pc, validationMode, policyType, backgroundMode, namespace, name, ruleName, ruleType, PolicyRuleCreated, ready); err != nil {
			return err
		}
	}
	return nil
}

func RemovePolicy(pc *metrics.PromConfig, policy kyverno.PolicyInterface) error {
	name, namespace, policyType, backgroundMode, validationMode, err := metrics.GetPolicyInfos(policy)
	if err != nil {
		return err
	}
	ready := policy.IsReady()
	for _, rule := range autogen.ComputeRules(policy) {
		ruleName := rule.Name
		ruleType := metrics.ParseRuleType(rule)
		if err = registerPolicyRuleInfoMetric(pc, validationMode, policyType, backgroundMode, namespace, name, ruleName, ruleType, PolicyRuleDeleted, ready); err != nil {
			return err
		}
	}
	return nil
}
