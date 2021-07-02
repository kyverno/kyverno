package policyruleinfo

import (
	"fmt"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	prom "github.com/prometheus/client_golang/prometheus"
)

func (pm PromMetrics) registerPolicyRuleInfoMetric(
	policyValidationMode metrics.PolicyValidationMode,
	policyType metrics.PolicyType,
	policyBackgroundMode metrics.PolicyBackgroundMode,
	policyNamespace, policyName, ruleName string,
	ruleType metrics.RuleType,
	metricChangeType PolicyRuleInfoMetricChangeType,
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

	if policyType == metrics.Cluster {
		policyNamespace = "-"
	}

	pm.PolicyRuleInfo.With(prom.Labels{
		"policy_validation_mode": string(policyValidationMode),
		"policy_type":            string(policyType),
		"policy_background_mode": string(policyBackgroundMode),
		"policy_namespace":       policyNamespace,
		"policy_name":            policyName,
		"rule_name":              ruleName,
		"rule_type":              string(ruleType),
	}).Set(metricValue)

	return nil
}

func (pm PromMetrics) AddPolicy(policy interface{}) error {
	switch inputPolicy := policy.(type) {
	case *kyverno.ClusterPolicy:
		policyValidationMode, err := metrics.ParsePolicyValidationMode(inputPolicy.Spec.ValidationFailureAction)
		if err != nil {
			return err
		}
		policyBackgroundMode := metrics.ParsePolicyBackgroundMode(inputPolicy.Spec.Background)
		policyType := metrics.Cluster
		policyNamespace := "" // doesn't matter for cluster policy
		policyName := inputPolicy.ObjectMeta.Name
		// registering the metrics on a per-rule basis
		for _, rule := range inputPolicy.Spec.Rules {
			ruleName := rule.Name
			ruleType := metrics.ParseRuleType(rule)

			if err = pm.registerPolicyRuleInfoMetric(policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, ruleName, ruleType, PolicyRuleCreated); err != nil {
				return err
			}
		}
		return nil
	case *kyverno.Policy:
		policyValidationMode, err := metrics.ParsePolicyValidationMode(inputPolicy.Spec.ValidationFailureAction)
		if err != nil {
			return err
		}
		policyBackgroundMode := metrics.ParsePolicyBackgroundMode(inputPolicy.Spec.Background)
		policyType := metrics.Namespaced
		policyNamespace := inputPolicy.ObjectMeta.Namespace
		policyName := inputPolicy.ObjectMeta.Name
		// registering the metrics on a per-rule basis
		for _, rule := range inputPolicy.Spec.Rules {
			ruleName := rule.Name
			ruleType := metrics.ParseRuleType(rule)

			if err = pm.registerPolicyRuleInfoMetric(policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, ruleName, ruleType, PolicyRuleCreated); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("wrong input type provided %T. Only kyverno.Policy and kyverno.ClusterPolicy allowed", inputPolicy)
	}
}

func (pm PromMetrics) RemovePolicy(policy interface{}) error {
	switch inputPolicy := policy.(type) {
	case *kyverno.ClusterPolicy:
		for _, rule := range inputPolicy.Spec.Rules {
			policyValidationMode, err := metrics.ParsePolicyValidationMode(inputPolicy.Spec.ValidationFailureAction)
			if err != nil {
				return err
			}
			policyBackgroundMode := metrics.ParsePolicyBackgroundMode(inputPolicy.Spec.Background)
			policyType := metrics.Cluster
			policyNamespace := "" // doesn't matter for cluster policy
			policyName := inputPolicy.ObjectMeta.Name
			ruleName := rule.Name
			ruleType := metrics.ParseRuleType(rule)

			if err = pm.registerPolicyRuleInfoMetric(policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, ruleName, ruleType, PolicyRuleDeleted); err != nil {
				return err
			}
		}
		return nil
	case *kyverno.Policy:
		for _, rule := range inputPolicy.Spec.Rules {
			policyValidationMode, err := metrics.ParsePolicyValidationMode(inputPolicy.Spec.ValidationFailureAction)
			if err != nil {
				return err
			}
			policyBackgroundMode := metrics.ParsePolicyBackgroundMode(inputPolicy.Spec.Background)
			policyType := metrics.Namespaced
			policyNamespace := inputPolicy.ObjectMeta.Namespace
			policyName := inputPolicy.ObjectMeta.Name
			ruleName := rule.Name
			ruleType := metrics.ParseRuleType(rule)

			if err = pm.registerPolicyRuleInfoMetric(policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, ruleName, ruleType, PolicyRuleDeleted); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("wrong input type provided %T. Only kyverno.Policy and kyverno.ClusterPolicy allowed", inputPolicy)
	}

}
