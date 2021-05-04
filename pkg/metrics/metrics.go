package metrics

import (
	prom "github.com/prometheus/client_golang/prometheus"
)

type PromConfig struct {
	MetricsRegistry *prom.Registry
	Metrics         *PromMetrics
}

type PromMetrics struct {
	PolicyRuleResults          *prom.GaugeVec
	PolicyRuleCount            *prom.GaugeVec
	PolicyChanges              *prom.GaugeVec
	PolicyRuleExecutionLatency *prom.GaugeVec
	AdmissionReviewLatency     *prom.GaugeVec
}

func NewPromConfig() *PromConfig {
	pc := new(PromConfig)

	pc.MetricsRegistry = prom.NewRegistry()

	policyRuleResultsLabels := []string{
		"policy_validation_mode", "policy_type", "policy_background_mode", "policy_name", "policy_namespace",
		"resource_name", "resource_kind", "resource_namespace", "resource_request_operation",
		"rule_name", "rule_result", "rule_type", "rule_execution_cause", "rule_response",
		"main_request_trigger_timestamp", "policy_execution_timestamp", "rule_execution_timestamp",
	}
	policyRuleResultsMetric := prom.NewGaugeVec(
		prom.GaugeOpts{
			Name: "kyverno_policy_rule_results",
			Help: "can be used to track the results associated with the policies applied in the user’s cluster, at the level from rule to policy to admission requests.",
		},
		policyRuleResultsLabels,
	)

	policyRuleCountLabels := []string{
		"policy_validation_mode", "policy_type", "policy_background_mode", "policy_namespace", "policy_name", "rule_name", "rule_type",
	}
	policyRuleCountMetric := prom.NewGaugeVec(
		prom.GaugeOpts{
			Name: "kyverno_policy_rule_count",
			Help: "can be used to track the number of rules or/and policies present in the cluster. 0 means the rule doesn't exist and has been deleted, 1 means the rule is currently existent in the cluster.",
		},
		policyRuleCountLabels,
	)

	policyChangesLabels := []string{
		"policy_validation_mode", "policy_type", "policy_background_mode", "policy_namespace", "policy_name", "policy_change_type", "timestamp",
	}
	policyChangesMetric := prom.NewGaugeVec(
		prom.GaugeOpts{
			Name: "kyverno_policy_changes",
			Help: "can be used to track all the Kyverno policies which have been created, updated or deleted.",
		},
		policyChangesLabels,
	)

	policyRuleExecutionLatencyLabels := []string{
		"policy_validation_mode", "policy_type", "policy_background_mode", "policy_name", "policy_namespace",
		"resource_name", "resource_kind", "resource_namespace", "resource_request_operation",
		"rule_name", "rule_result", "rule_type", "rule_execution_cause", "rule_response", "generate_rule_latency_type",
		"main_request_trigger_timestamp", "policy_execution_timestamp", "rule_execution_timestamp",
	}
	policyRuleExecutionLatencyMetric := prom.NewGaugeVec(
		prom.GaugeOpts{
			Name: "kyverno_policy_rule_execution_latency",
			Help: "can be used to track the latencies associated with the execution/processing of the individual rules under Kyverno policies whenever they evaluate incoming resource requests.",
		},
		policyRuleExecutionLatencyLabels,
	)

	admissionReviewLatency := []string{
		"cluster_policies_count", "namespaced_policies_count",
		"validate_rules_count", "mutate_rules_count", "generate_rules_count",
		"resource_name", "resource_kind", "resource_namespace", "resource_request_operation",
	}
	admissionReviewLatencyMetric := prom.NewGaugeVec(
		prom.GaugeOpts{
			Name: "kyverno_admission_review_latency",
			Help: "can be used to track the latencies associated with the entire individual admission review. For example, if an incoming request trigger, say, five policies, this metric will track the e2e latency associated with the execution of all those policies.",
		},
		admissionReviewLatency,
	)

	pc.Metrics = &PromMetrics{
		PolicyRuleResults:          policyRuleResultsMetric,
		PolicyRuleCount:            policyRuleCountMetric,
		PolicyChanges:              policyChangesMetric,
		PolicyRuleExecutionLatency: policyRuleExecutionLatencyMetric,
		AdmissionReviewLatency:     admissionReviewLatencyMetric,
	}

	pc.MetricsRegistry.MustRegister(pc.Metrics.PolicyRuleResults)
	pc.MetricsRegistry.MustRegister(pc.Metrics.PolicyRuleCount)
	pc.MetricsRegistry.MustRegister(pc.Metrics.PolicyChanges)
	pc.MetricsRegistry.MustRegister(pc.Metrics.PolicyRuleExecutionLatency)
	pc.MetricsRegistry.MustRegister(pc.Metrics.AdmissionReviewLatency)

	return pc
}
