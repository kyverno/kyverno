package metrics

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/config"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/robfig/cron/v3"
)

type PromConfig struct {
	MetricsRegistry *prom.Registry
	Metrics         *PromMetrics
	Config          *config.MetricsConfigData
	cron            *cron.Cron
}

type PromMetrics struct {
	PolicyResults           *prom.CounterVec
	PolicyRuleInfo          *prom.GaugeVec
	PolicyChanges           *prom.CounterVec
	PolicyExecutionDuration *prom.HistogramVec
	AdmissionReviewDuration *prom.HistogramVec
	AdmissionRequests       *prom.CounterVec
}

func NewPromConfig(metricsConfigData *config.MetricsConfigData) (*PromConfig, error) {
	pc := new(PromConfig)
	pc.Config = metricsConfigData
	pc.cron = cron.New()
	pc.MetricsRegistry = prom.NewRegistry()

	policyResultsLabels := []string{
		"policy_validation_mode", "policy_type", "policy_background_mode", "policy_name", "policy_namespace",
		"resource_kind", "resource_namespace", "resource_request_operation",
		"rule_name", "rule_result", "rule_type", "rule_execution_cause",
	}
	policyResultsMetric := prom.NewCounterVec(
		prom.CounterOpts{
			Name: "kyverno_policy_results_total",
			Help: "can be used to track the results associated with the policies applied in the user’s cluster, at the level from rule to policy to admission requests.",
		},
		policyResultsLabels,
	)

	policyRuleInfoLabels := []string{
		"policy_validation_mode", "policy_type", "policy_background_mode", "policy_namespace", "policy_name", "rule_name", "rule_type", "status_ready",
	}
	policyRuleInfoMetric := prom.NewGaugeVec(
		prom.GaugeOpts{
			Name: "kyverno_policy_rule_info_total",
			Help: "can be used to track the info of the rules or/and policies present in the cluster. 0 means the rule doesn't exist and has been deleted, 1 means the rule is currently existent in the cluster.",
		},
		policyRuleInfoLabels,
	)

	policyChangesLabels := []string{
		"policy_validation_mode", "policy_type", "policy_background_mode", "policy_namespace", "policy_name", "policy_change_type",
	}
	policyChangesMetric := prom.NewCounterVec(
		prom.CounterOpts{
			Name: "kyverno_policy_changes_total",
			Help: "can be used to track all the changes associated with the Kyverno policies present on the cluster such as creation, updates and deletions.",
		},
		policyChangesLabels,
	)

	policyExecutionDurationLabels := []string{
		"policy_validation_mode", "policy_type", "policy_background_mode", "policy_name", "policy_namespace",
		"resource_kind", "resource_namespace", "resource_request_operation",
		"rule_name", "rule_result", "rule_type", "rule_execution_cause", "generate_rule_latency_type",
	}
	policyExecutionDurationMetric := prom.NewHistogramVec(
		prom.HistogramOpts{
			Name: "kyverno_policy_execution_duration_seconds",
			Help: "can be used to track the latencies (in seconds) associated with the execution/processing of the individual rules under Kyverno policies whenever they evaluate incoming resource requests.",
		},
		policyExecutionDurationLabels,
	)

	admissionReviewDurationLabels := []string{
		"resource_kind", "resource_namespace", "resource_request_operation",
	}
	admissionReviewDurationMetric := prom.NewHistogramVec(
		prom.HistogramOpts{
			Name: "kyverno_admission_review_duration_seconds",
			Help: "can be used to track the latencies (in seconds) associated with the entire individual admission review. For example, if an incoming request trigger, say, five policies, this metric will track the e2e latency associated with the execution of all those policies.",
		},
		admissionReviewDurationLabels,
	)

	admissionRequestsLabels := []string{
		"resource_kind", "resource_namespace", "resource_request_operation",
	}
	admissionRequestsMetric := prom.NewCounterVec(
		prom.CounterOpts{
			Name: "kyverno_admission_requests_total",
			Help: "can be used to track the number of admission requests encountered by Kyverno in the cluster.",
		},
		admissionRequestsLabels,
	)

	pc.Metrics = &PromMetrics{
		PolicyResults:           policyResultsMetric,
		PolicyRuleInfo:          policyRuleInfoMetric,
		PolicyChanges:           policyChangesMetric,
		PolicyExecutionDuration: policyExecutionDurationMetric,
		AdmissionReviewDuration: admissionReviewDurationMetric,
		AdmissionRequests:       admissionRequestsMetric,
	}

	pc.MetricsRegistry.MustRegister(pc.Metrics.PolicyResults)
	pc.MetricsRegistry.MustRegister(pc.Metrics.PolicyRuleInfo)
	pc.MetricsRegistry.MustRegister(pc.Metrics.PolicyChanges)
	pc.MetricsRegistry.MustRegister(pc.Metrics.PolicyExecutionDuration)
	pc.MetricsRegistry.MustRegister(pc.Metrics.AdmissionReviewDuration)
	pc.MetricsRegistry.MustRegister(pc.Metrics.AdmissionRequests)

	// configuring metrics periodic refresh
	if pc.Config.GetMetricsRefreshInterval() != 0 {
		if len(pc.cron.Entries()) > 0 {
			logger.Info("Skipping the configuration of metrics refresh. Already found cron expiration to be set.")
		} else {
			_, err := pc.cron.AddFunc(fmt.Sprintf("@every %s", pc.Config.GetMetricsRefreshInterval()), func() {
				logger.Info("Resetting the metrics as per their periodic refresh")
				pc.Metrics.PolicyResults.Reset()
				pc.Metrics.PolicyRuleInfo.Reset()
				pc.Metrics.PolicyChanges.Reset()
				pc.Metrics.PolicyExecutionDuration.Reset()
				pc.Metrics.AdmissionReviewDuration.Reset()
				pc.Metrics.AdmissionRequests.Reset()
			})
			if err != nil {
				return nil, err
			}
			logger.Info(fmt.Sprintf("Configuring metrics refresh at a periodic rate of %s", pc.Config.GetMetricsRefreshInterval()))
			pc.cron.Start()
		}
	} else {
		logger.Info("Skipping the configuration of metrics refresh as 'metricsRefreshInterval' wasn't specified in values.yaml at the time of installing kyverno")
	}
	return pc, nil
}
