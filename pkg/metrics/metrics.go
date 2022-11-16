package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	kconfig "github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/kyverno/pkg/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"k8s.io/client-go/kubernetes"
)

const (
	meterName = "kyverno"
)

type MetricsConfig struct {
	// instruments
	policyChangesMetric           syncint64.Counter
	policyResultsMetric           syncint64.Counter
	policyRuleInfoMetric          asyncfloat64.Gauge
	policyExecutionDurationMetric syncfloat64.Histogram
	admissionRequestsMetric       syncint64.Counter
	admissionReviewDurationMetric syncfloat64.Histogram
	clientQueriesMetric           syncint64.Counter

	// config
	Config *kconfig.MetricsConfigData
	Log    logr.Logger
}

type MetricsConfigManager interface {
	RecordPolicyResults(policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, resourceKind string, resourceNamespace string, resourceRequestOperation ResourceRequestOperation, ruleName string, ruleResult RuleResult, ruleType RuleType, ruleExecutionCause RuleExecutionCause)
	RecordPolicyChanges(policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, policyChangeType string)
	RecordPolicyRuleInfo(policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, ruleName string, ruleType RuleType, status string, metricValue float64)
	RecordAdmissionRequests(resourceKind string, resourceNamespace string, resourceRequestOperation ResourceRequestOperation)
	RecordPolicyExecutionDuration(policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, ruleName string, ruleResult RuleResult, ruleType RuleType, ruleExecutionCause RuleExecutionCause, ruleExecutionLatency float64)
	RecordAdmissionReviewDuration(resourceKind string, resourceNamespace string, resourceRequestOperation string, admissionRequestLatency float64)
	RecordClientQueries(clientQueryOperation ClientQueryOperation, clientType ClientType, resourceKind string, resourceNamespace string)
}

func initializeMetrics(m *MetricsConfig) (*MetricsConfig, error) {
	var err error
	meter := global.MeterProvider().Meter(meterName)

	m.policyResultsMetric, err = meter.SyncInt64().Counter("kyverno_policy_results_total", instrument.WithDescription("can be used to track the results associated with the policies applied in the user’s cluster, at the level from rule to policy to admission requests"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_policy_results_total")
		return nil, err
	}

	m.policyChangesMetric, err = meter.SyncInt64().Counter("kyverno_policy_changes_total", instrument.WithDescription("can be used to track all the changes associated with the Kyverno policies present on the cluster such as creation, updates and deletions"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_policy_changes_total")
		return nil, err
	}

	m.admissionRequestsMetric, err = meter.SyncInt64().Counter("kyverno_admission_requests_total", instrument.WithDescription("can be used to track the number of admission requests encountered by Kyverno in the cluster"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_admission_requests_total")
		return nil, err
	}

	m.policyExecutionDurationMetric, err = meter.SyncFloat64().Histogram("kyverno_policy_execution_duration_seconds", instrument.WithDescription("can be used to track the latencies (in seconds) associated with the execution/processing of the individual rules under Kyverno policies whenever they evaluate incoming resource requests"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_policy_execution_duration_seconds")
		return nil, err
	}

	m.admissionReviewDurationMetric, err = meter.SyncFloat64().Histogram("kyverno_admission_review_duration_seconds", instrument.WithDescription("can be used to track the latencies (in seconds) associated with the entire individual admission review. For example, if an incoming request trigger, say, five policies, this metric will track the e2e latency associated with the execution of all those policies"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_admission_review_duration_seconds")
		return nil, err
	}

	// Register Async Callbacks
	m.policyRuleInfoMetric, err = meter.AsyncFloat64().Gauge("kyverno_policy_rule_info_total", instrument.WithDescription("can be used to track the info of the rules or/and policies present in the cluster. 0 means the rule doesn't exist and has been deleted, 1 means the rule is currently existent in the cluster"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_policy_rule_info_total")
		return nil, err
	}

	m.clientQueriesMetric, err = meter.SyncInt64().Counter("kyverno_client_queries_total", instrument.WithDescription("can be used to track the number of client queries sent from Kyverno to the API-server"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_client_queries_total")
		return nil, err
	}

	return m, nil
}

func ShutDownController(ctx context.Context, pusher *metric.MeterProvider) {
	if pusher != nil {
		// pushes any last exports to the receiver
		if err := pusher.Shutdown(ctx); err != nil {
			otel.Handle(err)
		}
	}
}

func NewOTLPGRPCConfig(
	endpoint string,
	certs string,
	kubeClient kubernetes.Interface,
	log logr.Logger,
) (*metric.MeterProvider, error) {
	ctx := context.TODO()
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
			semconv.ServiceNameKey.String("kyverno"),
			semconv.ServiceVersionKey.String(version.BuildVersion),
		),
	)
	if err != nil {
		log.Error(err, "failed creating resource")
		return nil, err
	}
	reader := metric.NewPeriodicReader(
		exporter,
		metric.WithInterval(2*time.Second),
	)
	// create controller and bind the exporter with it
	provider := metric.NewMeterProvider(
		metric.WithReader(reader),
		metric.WithResource(res),
	)
	global.SetMeterProvider(provider)

	return provider, nil
}

func NewPrometheusConfig(
	log logr.Logger,
) (*http.ServeMux, error) {
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
		return nil, err
	}

	exporter, err := prometheus.New()
	if err != nil {
		log.Error(err, "failed to initialize prometheus exporter")
		return nil, err
	}

	provider := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(res),
	)

	global.SetMeterProvider(provider)

	metricsServerMux := http.NewServeMux()
	metricsServerMux.Handle("/metrics", promhttp.Handler())

	return metricsServerMux, nil
}

func (m *MetricsConfig) RecordPolicyResults(policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string,
	resourceKind string, resourceNamespace string, resourceRequestOperation ResourceRequestOperation, ruleName string, ruleResult RuleResult, ruleType RuleType,
	ruleExecutionCause RuleExecutionCause,
) {
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

func (m *MetricsConfig) RecordPolicyChanges(policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, policyChangeType string) {
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
	ruleName string, ruleType RuleType, status string, metricValue float64,
) {
	ctx := context.Background()
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

func (m *MetricsConfig) RecordAdmissionRequests(resourceKind string, resourceNamespace string, resourceRequestOperation ResourceRequestOperation) {
	ctx := context.Background()

	commonLabels := []attribute.KeyValue{
		attribute.String("resource_kind", resourceKind),
		attribute.String("resource_namespace", resourceNamespace),
		attribute.String("resource_request_operation", string(resourceRequestOperation)),
	}

	m.admissionRequestsMetric.Add(ctx, 1, commonLabels...)
}

func (m *MetricsConfig) RecordPolicyExecutionDuration(policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string,
	ruleName string, ruleResult RuleResult, ruleType RuleType, ruleExecutionCause RuleExecutionCause, ruleExecutionLatency float64,
) {
	ctx := context.Background()

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

func (m *MetricsConfig) RecordAdmissionReviewDuration(resourceKind string, resourceNamespace string, resourceRequestOperation string, admissionRequestLatency float64) {
	ctx := context.Background()

	commonLabels := []attribute.KeyValue{
		attribute.String("resource_kind", resourceKind),
		attribute.String("resource_namespace", resourceNamespace),
		attribute.String("resource_request_operation", resourceRequestOperation),
	}

	m.admissionReviewDurationMetric.Record(ctx, admissionRequestLatency, commonLabels...)
}

func (m *MetricsConfig) RecordClientQueries(clientQueryOperation ClientQueryOperation, clientType ClientType, resourceKind string, resourceNamespace string) {
	ctx := context.Background()

	commonLabels := []attribute.KeyValue{
		attribute.String("operation", string(clientQueryOperation)),
		attribute.String("client_type", string(clientType)),
		attribute.String("resource_kind", resourceKind),
		attribute.String("resource_namespace", resourceNamespace),
	}

	m.clientQueriesMetric.Add(ctx, 1, commonLabels...)
}
