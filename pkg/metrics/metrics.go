package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	kconfig "github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/kyverno/pkg/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"k8s.io/client-go/kubernetes"
)

const (
	MeterName = "kyverno"
)

type MetricsConfig struct {
	// instruments
	policyChangesMetric           syncint64.Counter
	policyResultsMetric           syncint64.Counter
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
	RecordPolicyExecutionDuration(ctx context.Context, policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, ruleName string, ruleResult RuleResult, ruleType RuleType, ruleExecutionCause RuleExecutionCause, ruleExecutionLatency float64)
	RecordClientQueries(ctx context.Context, clientQueryOperation ClientQueryOperation, clientType ClientType, resourceKind string, resourceNamespace string)
}

func (m *MetricsConfig) Config() kconfig.MetricsConfiguration {
	return m.config
}

func (m *MetricsConfig) initializeMetrics(meterProvider metric.MeterProvider) error {
	var err error
	meter := meterProvider.Meter(MeterName)
	m.policyResultsMetric, err = meter.SyncInt64().Counter("kyverno_policy_results", instrument.WithDescription("can be used to track the results associated with the policies applied in the user’s cluster, at the level from rule to policy to admission requests"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_policy_results")
		return err
	}
	m.policyChangesMetric, err = meter.SyncInt64().Counter("kyverno_policy_changes", instrument.WithDescription("can be used to track all the changes associated with the Kyverno policies present on the cluster such as creation, updates and deletions"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_policy_changes")
		return err
	}
	m.policyExecutionDurationMetric, err = meter.SyncFloat64().Histogram("kyverno_policy_execution_duration_seconds", instrument.WithDescription("can be used to track the latencies (in seconds) associated with the execution/processing of the individual rules under Kyverno policies whenever they evaluate incoming resource requests"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_policy_execution_duration_seconds")
		return err
	}
	m.clientQueriesMetric, err = meter.SyncInt64().Counter("kyverno_client_queries", instrument.WithDescription("can be used to track the number of client queries sent from Kyverno to the API-server"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_client_queries")
		return err
	}
	return nil
}

func ShutDownController(ctx context.Context, pusher *sdkmetric.MeterProvider) {
	if pusher != nil {
		// pushes any last exports to the receiver
		if err := pusher.Shutdown(ctx); err != nil {
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
) (metric.MeterProvider, error) {
	options := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(endpoint)}
	if certs != "" {
		// here the certificates are stored as configmaps
		transportCreds, err := kube.FetchCert(ctx, certs, kubeClient)
		if err != nil {
			log.Error(err, "Error fetching certificate from secret")
			return nil, err
		}
		options = append(options, otlpmetricgrpc.WithTLSCredentials(transportCreds))
	} else {
		options = append(options, otlpmetricgrpc.WithInsecure())
	}
	// create new exporter for exporting metrics
	exporter, err := otlpmetricgrpc.New(ctx, options...)
	if err != nil {
		log.Error(err, "Failed to create the collector exporter")
		return nil, err
	}
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(MeterName),
			semconv.ServiceVersionKey.String(version.BuildVersion),
		),
	)
	if err != nil {
		log.Error(err, "failed creating resource")
		return nil, err
	}
	reader := sdkmetric.NewPeriodicReader(
		exporter,
		sdkmetric.WithInterval(2*time.Second),
	)
	// create controller and bind the exporter with it
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	)
	return provider, nil
}

func NewPrometheusConfig(
	ctx context.Context,
	log logr.Logger,
) (metric.MeterProvider, *http.ServeMux, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("kyverno-svc-metrics"),
			semconv.ServiceNamespaceKey.String(kconfig.KyvernoNamespace()),
			semconv.ServiceVersionKey.String(version.BuildVersion),
		),
	)
	if err != nil {
		log.Error(err, "failed creating resource")
		return nil, nil, err
	}
	exporter, err := prometheus.New(
		prometheus.WithoutUnits(),
		prometheus.WithoutTargetInfo(),
	)
	if err != nil {
		log.Error(err, "failed to initialize prometheus exporter")
		return nil, nil, err
	}
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(res),
	)
	metricsServerMux := http.NewServeMux()
	metricsServerMux.Handle(config.MetricsPath, promhttp.Handler())
	return provider, metricsServerMux, nil
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
