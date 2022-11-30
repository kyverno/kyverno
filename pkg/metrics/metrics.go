package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	kconfig "github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/utils/kube"
	"go.opentelemetry.io/otel"
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
	"k8s.io/client-go/kubernetes"
)

const (
	MeterName = "kyverno"
)

type MetricsConfig struct {
	// instruments
	policyChangesMetric           syncint64.Counter
	policyResultsMetric           syncint64.Counter
	policyRuleInfoMetric          asyncfloat64.Gauge
	policyExecutionDurationMetric syncfloat64.Histogram
	clientQueriesMetric           syncint64.Counter

	// config
	config kconfig.MetricsConfiguration
	Log    logr.Logger
}

type MetricsConfigManager interface {
	Config() kconfig.MetricsConfiguration
	RecordPolicyResults(ctx context.Context, policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, resourceKind string, resourceNamespace string, resourceRequestOperation ResourceRequestOperation, ruleName string, ruleResult RuleResult, ruleType RuleType, ruleExecutionCause RuleExecutionCause)
	RecordPolicyChanges(ctx context.Context, policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, policyChangeType string)
	RecordPolicyRuleInfo(ctx context.Context, policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, ruleName string, ruleType RuleType, status string, metricValue float64)
	RecordPolicyExecutionDuration(ctx context.Context, policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, ruleName string, ruleResult RuleResult, ruleType RuleType, ruleExecutionCause RuleExecutionCause, ruleExecutionLatency float64)
	RecordClientQueries(ctx context.Context, clientQueryOperation ClientQueryOperation, clientType ClientType, resourceKind string, resourceNamespace string)
}

func (m *MetricsConfig) Config() kconfig.MetricsConfiguration {
	return m.config
}

func (m *MetricsConfig) initializeMetrics() error {
	var err error
	meter := global.MeterProvider().Meter(MeterName)

	m.policyResultsMetric, err = meter.SyncInt64().Counter("kyverno_policy_results_total", instrument.WithDescription("can be used to track the results associated with the policies applied in the user’s cluster, at the level from rule to policy to admission requests"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_policy_results_total")
		return err
	}

	m.policyChangesMetric, err = meter.SyncInt64().Counter("kyverno_policy_changes_total", instrument.WithDescription("can be used to track all the changes associated with the Kyverno policies present on the cluster such as creation, updates and deletions"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_policy_changes_total")
		return err
	}

	m.policyExecutionDurationMetric, err = meter.SyncFloat64().Histogram("kyverno_policy_execution_duration_seconds", instrument.WithDescription("can be used to track the latencies (in seconds) associated with the execution/processing of the individual rules under Kyverno policies whenever they evaluate incoming resource requests"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_policy_execution_duration_seconds")
		return err
	}

	// Register Async Callbacks
	m.policyRuleInfoMetric, err = meter.AsyncFloat64().Gauge("kyverno_policy_rule_info_total", instrument.WithDescription("can be used to track the info of the rules or/and policies present in the cluster. 0 means the rule doesn't exist and has been deleted, 1 means the rule is currently existent in the cluster"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_policy_rule_info_total")
		return err
	}

	m.clientQueriesMetric, err = meter.SyncInt64().Counter("kyverno_client_queries_total", instrument.WithDescription("can be used to track the number of client queries sent from Kyverno to the API-server"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_client_queries_total")
		return err
	}

	return nil
}

func ShutDownController(ctx context.Context, pusher *controller.Controller) {
	if pusher != nil {
		// pushes any last exports to the receiver
		if err := pusher.Stop(ctx); err != nil {
			otel.Handle(err)
		}
	}
}

func NewOTLPGRPCConfig(
	ctx context.Context,
	endpoint string,
	certs string,
	kubeClient kubernetes.Interface,
	log logr.Logger,
) (*controller.Controller, error) {
	var client otlpmetric.Client

	if certs != "" {
		// here the certificates are stored as configmaps
		transportCreds, err := kube.FetchCert(ctx, certs, kubeClient)
		if err != nil {
			log.Error(err, "Error fetching certificate from secret")
			return nil, err
		}

		client = otlpmetricgrpc.NewClient(
			otlpmetricgrpc.WithEndpoint(endpoint),
			otlpmetricgrpc.WithTLSCredentials(transportCreds),
		)
	} else {
		client = otlpmetricgrpc.NewClient(
			otlpmetricgrpc.WithEndpoint(endpoint),
			otlpmetricgrpc.WithInsecure(),
		)
	}

	// create New Exporter for exporting metrics
	metricExp, err := otlpmetric.New(ctx, client)
	if err != nil {
		log.Error(err, "Failed to create the collector exporter")
		return nil, err
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String("kyverno_metrics")),
		resource.WithSchemaURL(semconv.SchemaURL),
	)
	if err != nil {
		log.Error(err, "failed creating resource")
		return nil, err
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

	if err := pusher.Start(ctx); err != nil {
		log.Error(err, "could not start metric exporter")
		return nil, err
	}

	return pusher, nil
}

func NewPrometheusConfig(
	ctx context.Context,
	log logr.Logger,
) (*http.ServeMux, error) {
	config := prometheus.Config{}
	res, err := resource.New(
		ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String("kyverno-svc-metrics")),
		resource.WithAttributes(semconv.ServiceNamespaceKey.String(kconfig.KyvernoNamespace())),
		resource.WithSchemaURL(semconv.SchemaURL),
	)
	if err != nil {
		log.Error(err, "failed creating resource")
		return nil, err
	}

	c := controller.New(
		processor.NewFactory(
			simple.NewWithHistogramDistribution(),
			aggregation.CumulativeTemporalitySelector(),
			processor.WithMemory(true),
		),
		controller.WithResource(res),
		controller.WithCollectPeriod(10*time.Second),
	)

	exporter, err := prometheus.New(config, c)
	if err != nil {
		log.Error(err, "failed to initialize prometheus exporter")
		return nil, err
	}

	global.SetMeterProvider(exporter.MeterProvider())

	metricsServerMux := http.NewServeMux()
	metricsServerMux.HandleFunc("/metrics", exporter.ServeHTTP)

	return metricsServerMux, nil
}

func (m *MetricsConfig) RecordPolicyResults(ctx context.Context, policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string,
	resourceKind string, resourceNamespace string, resourceRequestOperation ResourceRequestOperation, ruleName string, ruleResult RuleResult, ruleType RuleType,
	ruleExecutionCause RuleExecutionCause,
) {
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

func (m *MetricsConfig) RecordPolicyChanges(ctx context.Context, policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, policyChangeType string) {
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

func (m *MetricsConfig) RecordPolicyRuleInfo(ctx context.Context, policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string,
	ruleName string, ruleType RuleType, status string, metricValue float64,
) {
	commonLabels := []attribute.KeyValue{
		attribute.String("policy_validation_mode", string(policyValidationMode)),
		attribute.String("policy_type", string(policyType)),
		attribute.String("policy_background_mode", string(policyBackgroundMode)),
		attribute.String("policy_namespace", policyNamespace),
		attribute.String("policy_name", policyName),
		attribute.String("rule_name", ruleName),
		attribute.String("rule_type", string(ruleType)),
		attribute.String("status_ready", status),
	}
	m.policyRuleInfoMetric.Observe(ctx, metricValue, commonLabels...)
}

func (m *MetricsConfig) RecordPolicyExecutionDuration(ctx context.Context, policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string,
	ruleName string, ruleResult RuleResult, ruleType RuleType, ruleExecutionCause RuleExecutionCause, ruleExecutionLatency float64,
) {
	commonLabels := []attribute.KeyValue{
		attribute.String("policy_validation_mode", string(policyValidationMode)),
		attribute.String("policy_type", string(policyType)),
		attribute.String("policy_background_mode", string(policyBackgroundMode)),
		attribute.String("policy_namespace", policyNamespace),
		attribute.String("policy_name", policyName),
		attribute.String("rule_name", ruleName),
		attribute.String("rule_result", string(ruleResult)),
		attribute.String("rule_type", string(ruleType)),
		attribute.String("rule_execution_cause", string(ruleExecutionCause)),
	}
	m.policyExecutionDurationMetric.Record(ctx, ruleExecutionLatency, commonLabels...)
}

func (m *MetricsConfig) RecordClientQueries(ctx context.Context, clientQueryOperation ClientQueryOperation, clientType ClientType, resourceKind string, resourceNamespace string) {
	commonLabels := []attribute.KeyValue{
		attribute.String("operation", string(clientQueryOperation)),
		attribute.String("client_type", string(clientType)),
		attribute.String("resource_kind", resourceKind),
		attribute.String("resource_namespace", resourceNamespace),
	}
	m.clientQueriesMetric.Add(ctx, 1, commonLabels...)
}
