package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/robfig/cron/v3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

const (
	meterName = "kyverno"
)

// TODO: Clear map memory after certain intervals
var (
	ruleInfo = make(map[[8]string]float64)
)

type MetricsConfig struct {
	// instruments
	policyChangesMetric           syncint64.Counter
	policyResultsMetric           syncint64.Counter
	policyRuleInfoMetric          asyncfloat64.Gauge
	policyExecutionDurationMetric syncfloat64.Histogram
	admissionRequestsMetric       syncint64.Counter
	admissionReviewDurationMetric syncfloat64.Histogram

	// config
	Config *config.MetricsConfigData
	cron   *cron.Cron
	Log    logr.Logger
}

func initializeMetrics(m *MetricsConfig) (*MetricsConfig, error) {
	var err error
	meter := global.MeterProvider().Meter(meterName)

	m.policyResultsMetric, err = meter.SyncInt64().Counter("kyverno_policy_results_total")
	if err != nil {
		m.Log.Error(err, "Failed to create instrument")
		return nil, err
	}

	m.policyChangesMetric, err = meter.SyncInt64().Counter("kyverno_policy_changes_total")
	if err != nil {
		m.Log.Error(err, "Failed to create instrument")
		return nil, err
	}

	m.admissionRequestsMetric, err = meter.SyncInt64().Counter("kyverno_admission_requests_total")
	if err != nil {
		m.Log.Error(err, "Failed to create instrument")
		return nil, err
	}

	m.policyExecutionDurationMetric, err = meter.SyncFloat64().Histogram("kyverno_policy_execution_duration_seconds")
	if err != nil {
		m.Log.Error(err, "Failed to create instrument")
		return nil, err
	}

	m.admissionReviewDurationMetric, err = meter.SyncFloat64().Histogram("kyverno_admission_review_duration_seconds")
	if err != nil {
		m.Log.Error(err, "Failed to create instrument")
		return nil, err
	}

	// Register Async Callbacks
	m.policyRuleInfoMetric, err = meter.AsyncFloat64().Gauge("kyverno_policy_rule_info_total")
	if err != nil {
		m.Log.Error(err, "Failed to create instrument")
		return nil, err
	}

	err = meter.RegisterCallback([]instrument.Asynchronous{m.policyRuleInfoMetric},
		func(ctx context.Context) {
			lock := sync.RWMutex{}
			lock.RLock()
			defer lock.RUnlock()

			for k, v := range ruleInfo {
				commonLabels := []attribute.KeyValue{
					attribute.String("policy_validation_mode", k[0]),
					attribute.String("policy_type", k[1]),
					attribute.String("policy_background_mode", k[2]),
					attribute.String("policy_namespace", k[3]),
					attribute.String("policy_name", k[4]),
					attribute.String("rule_name", k[5]),
					attribute.String("rule_type", k[6]),
					attribute.String("status", k[7]),
				}
				m.policyRuleInfoMetric.Observe(ctx, v, commonLabels...)
			}
		})

	if err != nil {
		m.Log.Error(err, "failed to register callback")
		return nil, err
	}

	return m, nil
}

func NewOTLPGRPCConfig(endpoint string, metricsConfigData *config.MetricsConfigData, log logr.Logger) (*MetricsConfig, error) {
	ctx := context.Background()
	client := otlpmetricgrpc.NewClient(
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithEndpoint(endpoint),
	)

	// create New Exporter for exporting metrics
	metricExp, err := otlpmetric.New(ctx, client)
	if err != nil {
		log.Error(err, "Failed to create the collector exporter")
		return nil, err
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(semconv.ServiceNameKey.String("kyverno-svc-metrics")),
		resource.WithAttributes(semconv.ServiceNamespaceKey.String("kyverno")),
		resource.WithSchemaURL(semconv.SchemaURL),
	)
	if err != nil {
		log.Error(err, "failed creating resource")
	}

	// create controller and bind the exporter with it
	pusher := controller.New(
		processor.NewFactory(
			simple.NewWithHistogramDistribution(),
			aggregation.CumulativeTemporalitySelector(),
			processor.WithMemory(true),
		),
		controller.WithExporter(metricExp),
		controller.WithResource(res),
		controller.WithCollectPeriod(2*time.Second),
	)
	global.SetMeterProvider(pusher)
	// meterProvider = pusher

	m := new(MetricsConfig)
	m.Log = log
	m.Config = metricsConfigData
	m.cron = cron.New()

	m, err = initializeMetrics(m)

	if err != nil {
		log.Error(err, "Failed initializing metrics")
		return nil, err
	}

	if err := pusher.Start(ctx); err != nil {
		log.Error(err, "could not start metric exporter")
		return nil, err
	}

	// configuring metrics periodic refresh
	if m.Config.GetMetricsRefreshInterval() != 0 {
		if len(m.cron.Entries()) > 0 {
			m.Log.Info("Skipping the configuration of metrics refresh. Already found cron expiration to be set.")
		} else {
			_, err := m.cron.AddFunc(fmt.Sprintf("@every %s", m.Config.GetMetricsRefreshInterval()), func() {
				m.Log.Info("Resetting the metrics as per their periodic refresh")
				// reset metrics here - clear map values
				for k := range ruleInfo {
					delete(ruleInfo, k)
				}
			})
			if err != nil {
				return nil, err
			}
			log.Info(fmt.Sprintf("Configuring metrics refresh at a periodic rate of %s", m.Config.GetMetricsRefreshInterval()))
			m.cron.Start()
		}
	} else {
		m.Log.Info("Skipping the configuration of metrics refresh as 'metricsRefreshInterval' wasn't specified in values.yaml at the time of installing kyverno")
	}
	return m, nil
}

func NewPrometheusConfig(metricsConfigData *config.MetricsConfigData, log logr.Logger) (*MetricsConfig, *http.ServeMux, error) {
	config := prometheus.Config{}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(semconv.ServiceNameKey.String("kyverno-svc-metrics")),
		resource.WithAttributes(semconv.ServiceNamespaceKey.String("kyverno")),
		resource.WithSchemaURL(semconv.SchemaURL),
	)
	if err != nil {
		log.Error(err, "failed creating resource")
	}

	c := controller.New(
		processor.NewFactory(
			simple.NewWithHistogramDistribution(),
			aggregation.CumulativeTemporalitySelector(),
			processor.WithMemory(true),
		),
		controller.WithResource(res),
	)

	exporter, err := prometheus.New(config, c)
	if err != nil {
		log.Error(err, "failed to initialize prometheus exporter")
		return nil, nil, err
	}

	global.SetMeterProvider(exporter.MeterProvider())

	m := new(MetricsConfig)
	m.Log = log
	m.Config = metricsConfigData
	m.cron = cron.New()

	m, err = initializeMetrics(m)

	if err != nil {
		log.Error(err, "failed to initialize metrics config")
	}

	metricsServerMux := http.NewServeMux()
	metricsServerMux.HandleFunc("/metrics", exporter.ServeHTTP)

	// configuring metrics periodic refresh
	if m.Config.GetMetricsRefreshInterval() != 0 {
		if len(m.cron.Entries()) > 0 {
			m.Log.Info("Skipping the configuration of metrics refresh. Already found cron expiration to be set.")
		} else {
			_, err := m.cron.AddFunc(fmt.Sprintf("@every %s", m.Config.GetMetricsRefreshInterval()), func() {
				m.Log.Info("Resetting the metrics as per their periodic refresh")
				// reset metrics here - clear map values
				for k := range ruleInfo {
					delete(ruleInfo, k)
				}
			})
			if err != nil {
				return nil, nil, err
			}
			log.Info(fmt.Sprintf("Configuring metrics refresh at a periodic rate of %s", m.Config.GetMetricsRefreshInterval()))
			m.cron.Start()
		}
	} else {
		m.Log.Info("Skipping the configuration of metrics refresh as 'metricsRefreshInterval' wasn't specified in values.yaml at the time of installing kyverno")
	}

	return m, metricsServerMux, nil
}

func (m *MetricsConfig) RecordPolicyResults(policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string,
	resourceKind string, resourceNamespace string, resourceRequestOperation ResourceRequestOperation, ruleName string, ruleResult RuleResult, ruleType RuleType,
	ruleExecutionCause RuleExecutionCause, log logr.Logger) {
	ctx := context.Background()

	commonLabels := []attribute.KeyValue{
		attribute.String("policy_validation_mode", string(policyValidationMode)),
		attribute.String("policy_type", string(policyType)),
		attribute.String("policy_background_mode", string(policyBackgroundMode)),
		attribute.String("policy_namespace", policyNamespace),
		attribute.String("policy_name", policyName),
		attribute.String("resource_kind", resourceKind),
		attribute.String("resource_namespace", resourceNamespace),
		attribute.String("resource_request_operation", string(resourceRequestOperation)),
		attribute.String("rule_name", ruleName),
		attribute.String("rule_result", string(ruleResult)),
		attribute.String("rule_type", string(ruleType)),
		attribute.String("rule_execution_cause", string(ruleExecutionCause)),
	}

	m.policyResultsMetric.Add(ctx, 1, commonLabels...)
}

func (m *MetricsConfig) RecordPolicyChanges(policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, policyChangeType string, log logr.Logger) {
	ctx := context.Background()

	commonLabels := []attribute.KeyValue{
		attribute.String("policy_validation_mode", string(policyValidationMode)),
		attribute.String("policy_type", string(policyType)),
		attribute.String("policy_background_mode", string(policyBackgroundMode)),
		attribute.String("policy_namespace", policyNamespace),
		attribute.String("policy_name", policyName),
		attribute.String("policy_change_type", policyChangeType),
	}

	m.policyChangesMetric.Add(ctx, 1, commonLabels...)
}

func (m *MetricsConfig) RecordPolicyRuleInfo(policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string,
	ruleName string, ruleType RuleType, status string, metricValue float64, log logr.Logger) {

	// TODO: store the metric labels and value in a map, delete after 24 hrs, register callback to observe these metrics
	lock := sync.RWMutex{}
	lock.Lock()
	defer lock.Unlock()

	var labels = [8]string{string(policyValidationMode), string(policyType), string(policyBackgroundMode), policyNamespace, policyName, ruleName, string(ruleType), status}
	ruleInfo[labels] = metricValue
}

func (m MetricsConfig) RecordAdmissionRequests(resourceKind string, resourceNamespace string, resourceRequestOperation ResourceRequestOperation, log logr.Logger) {
	ctx := context.Background()

	commonLabels := []attribute.KeyValue{
		attribute.String("resource_kind", resourceKind),
		attribute.String("resource_namespace", resourceNamespace),
		attribute.String("resource_request_operation", string(resourceRequestOperation)),
	}

	m.admissionRequestsMetric.Add(ctx, 1, commonLabels...)
}

func (m *MetricsConfig) RecordPolicyExecutionDuration(policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string,
	resourceKind string, resourceNamespace string, resourceRequestOperation ResourceRequestOperation, ruleName string, ruleResult RuleResult, ruleType RuleType,
	ruleExecutionCause RuleExecutionCause, generalRuleLatencyType string, ruleExecutionLatency float64, log logr.Logger) {
	ctx := context.Background()

	commonLabels := []attribute.KeyValue{
		attribute.String("policy_validation_mode", string(policyValidationMode)),
		attribute.String("policy_type", string(policyType)),
		attribute.String("policy_background_mode", string(policyBackgroundMode)),
		attribute.String("policy_namespace", policyNamespace),
		attribute.String("policy_name", policyName),
		attribute.String("resource_kind", resourceKind),
		attribute.String("resource_namespace", resourceNamespace),
		attribute.String("resource_request_operation", string(resourceRequestOperation)),
		attribute.String("rule_name", ruleName),
		attribute.String("rule_result", string(ruleResult)),
		attribute.String("rule_type", string(ruleType)),
		attribute.String("rule_execution_cause", string(ruleExecutionCause)),
		attribute.String("general_rule_latency_type", string(generalRuleLatencyType)),
	}

	m.policyExecutionDurationMetric.Record(ctx, ruleExecutionLatency, commonLabels...)
}

func (m *MetricsConfig) RecordAdmissionReviewDuration(resourceKind string, resourceNamespace string, resourceRequestOperation string, admissionRequestLatency float64, log logr.Logger) {
	ctx := context.Background()

	commonLabels := []attribute.KeyValue{
		attribute.String("resource_kind", resourceKind),
		attribute.String("resource_namespace", resourceNamespace),
		attribute.String("resource_request_operation", string(resourceRequestOperation)),
	}

	m.admissionReviewDurationMetric.Record(ctx, admissionRequestLatency, commonLabels...)
}
