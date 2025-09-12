package metrics

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kconfig "github.com/kyverno/kyverno/pkg/config"
	tlsutils "github.com/kyverno/kyverno/pkg/utils/tls"
	"github.com/kyverno/kyverno/pkg/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"k8s.io/client-go/kubernetes"
)

const (
	MeterName = "kyverno"
)

var metricsConfig MetricsConfigManager

func GetManager() MetricsConfigManager {
	return metricsConfig
}

func SetManager(manager MetricsConfigManager) {
	metricsConfig = manager
}

type MetricsConfig struct {
	// instruments
	policyChangesMetric metric.Int64Counter
	clientQueriesMetric metric.Int64Counter
	kyvernoInfoMetric   metric.Int64Gauge
	breakerMetrics      *breakerMetrics
	controllerMetrics   *controllerMetrics
	cleanupMetrics      *cleanupMetrics
	deletingMetrics     *deletingMetrics
	policyRuleMetrics   *policyRuleMetrics
	ttlInfoMetrics      *ttlInfoMetrics
	policyEngineMetrics *policyEngineMetrics
	eventMetrics        *eventMetrics
	admissionMetrics    *admissionMetrics
	httpMetrics         *httpMetrics
	vpolMetrics         *validatingMetrics
	ivpolMetrics        *imageValidatingMetrics
	mpolMetrics         *mutatingMetrics

	// config
	config kconfig.MetricsConfiguration
	Log    logr.Logger
}

type MetricsConfigManager interface {
	Config() kconfig.MetricsConfiguration
	RecordPolicyChanges(ctx context.Context, policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, policyChangeType string)
	RecordClientQueries(ctx context.Context, clientQueryOperation ClientQueryOperation, clientType ClientType, resourceKind string, resourceNamespace string)
	BreakerMetrics() BreakerMetrics
	ControllerMetrics() ControllerMetrics
	CleanupMetrics() CleanupMetrics
	DeletingMetrics() DeletingMetrics
	PolicyRuleMetrics() PolicyRuleMetrics
	TTLInfoMetrics() TTLInfoMetrics
	PolicyEngineMetrics() PolicyEngineMetrics
	EventMetrics() EventMetrics
	AdmissionMetrics() AdmissionMetrics
	HTTPMetrics() HTTPMetrics
	VPOLMetrics() ValidatingMetrics
	IVPOLMetrics() ImageValidatingMetrics
	MPOLMetrics() MutatingMetrics
}

func (m *MetricsConfig) Config() kconfig.MetricsConfiguration {
	return m.config
}

func (m *MetricsConfig) BreakerMetrics() BreakerMetrics {
	return m.breakerMetrics
}

func (m *MetricsConfig) ControllerMetrics() ControllerMetrics {
	return m.controllerMetrics
}

func (m *MetricsConfig) CleanupMetrics() CleanupMetrics {
	return m.cleanupMetrics
}

func (m *MetricsConfig) DeletingMetrics() DeletingMetrics {
	return m.deletingMetrics
}

func (m *MetricsConfig) PolicyRuleMetrics() PolicyRuleMetrics {
	return m.policyRuleMetrics
}

func (m *MetricsConfig) TTLInfoMetrics() TTLInfoMetrics {
	return m.ttlInfoMetrics
}

func (m *MetricsConfig) PolicyEngineMetrics() PolicyEngineMetrics {
	return m.policyEngineMetrics
}

func (m *MetricsConfig) EventMetrics() EventMetrics {
	return m.eventMetrics
}

func (m *MetricsConfig) AdmissionMetrics() AdmissionMetrics {
	return m.admissionMetrics
}

func (m *MetricsConfig) HTTPMetrics() HTTPMetrics {
	return m.httpMetrics
}

func (m *MetricsConfig) VPOLMetrics() ValidatingMetrics {
	return m.vpolMetrics
}

func (m *MetricsConfig) IVPOLMetrics() ImageValidatingMetrics {
	return m.ivpolMetrics
}

func (m *MetricsConfig) MPOLMetrics() MutatingMetrics {
	return m.mpolMetrics
}

func (m *MetricsConfig) initializeMetrics(meterProvider metric.MeterProvider) error {
	var err error
	meter := meterProvider.Meter(MeterName)
	if meter == nil {
		return nil
	}

	m.policyChangesMetric, err = meter.Int64Counter("kyverno_policy_changes", metric.WithDescription("can be used to track all the changes associated with the Kyverno policies present on the cluster such as creation, updates and deletions"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_policy_changes")
		return err
	}
	m.clientQueriesMetric, err = meter.Int64Counter("kyverno_client_queries", metric.WithDescription("can be used to track the number of client queries sent from Kyverno to the API-server"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_client_queries")
		return err
	}
	m.kyvernoInfoMetric, err = meter.Int64Gauge("kyverno_info",
		metric.WithDescription("Kyverno version information"),
	)
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_info")
		return err
	}

	m.breakerMetrics.init(meter)
	m.controllerMetrics.init(meter)
	m.cleanupMetrics.init(meter)
	m.deletingMetrics.init(meter)
	m.policyRuleMetrics.init(meter)
	m.ttlInfoMetrics.init(meter)
	m.policyEngineMetrics.init(meter)
	m.eventMetrics.init(meter)
	m.admissionMetrics.init(meter)
	m.httpMetrics.init(meter)
	m.vpolMetrics.init(meter)
	m.ivpolMetrics.init(meter)
	m.mpolMetrics.init(meter)

	initKyvernoInfoMetric(m)
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

func aggregationSelector(metricsConfiguration kconfig.MetricsConfiguration) func(ik sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return func(ik sdkmetric.InstrumentKind) sdkmetric.Aggregation {
		switch ik {
		case sdkmetric.InstrumentKindHistogram:
			return sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: metricsConfiguration.GetBucketBoundaries(),
				NoMinMax:   false,
			}
		default:
			return sdkmetric.DefaultAggregationSelector(ik)
		}
	}
}

func NewOTLPGRPCConfig(ctx context.Context, endpoint string, certs string, kubeClient kubernetes.Interface, log logr.Logger, configuration kconfig.MetricsConfiguration) (metric.MeterProvider, error) {
	options := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(endpoint), otlpmetricgrpc.WithAggregationSelector(aggregationSelector(configuration))}
	if certs != "" {
		// here the certificates are stored as configmaps
		transportCreds, err := tlsutils.FetchCert(ctx, certs, kubeClient)
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
		resource.NewSchemaless(
			semconv.ServiceNameKey.String(MeterName),
			semconv.ServiceVersionKey.String(version.Version()),
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
		sdkmetric.WithView(configuration.BuildMeterProviderViews()...),
	)
	return provider, nil
}

func NewPrometheusConfig(ctx context.Context, log logr.Logger, configuration kconfig.MetricsConfiguration) (metric.MeterProvider, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceNameKey.String("kyverno-svc-metrics"),
			semconv.ServiceNamespaceKey.String(kconfig.KyvernoNamespace()),
			semconv.ServiceVersionKey.String(version.Version()),
		),
	)
	if err != nil {
		log.Error(err, "failed creating resource")
		return nil, err
	}
	exporter, err := prometheus.New(
		prometheus.WithoutUnits(),
		prometheus.WithoutTargetInfo(),
		prometheus.WithAggregationSelector(aggregationSelector(configuration)),
	)
	if err != nil {
		log.Error(err, "failed to initialize prometheus exporter")
		return nil, err
	}
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(res),
		sdkmetric.WithView(configuration.BuildMeterProviderViews()...),
	)
	return provider, nil
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
	m.policyChangesMetric.Add(ctx, 1, metric.WithAttributes(commonLabels...))
}

func (m *MetricsConfig) RecordClientQueries(ctx context.Context, clientQueryOperation ClientQueryOperation, clientType ClientType, resourceKind string, resourceNamespace string) {
	commonLabels := []attribute.KeyValue{
		attribute.String("operation", string(clientQueryOperation)),
		attribute.String("client_type", string(clientType)),
		attribute.String("resource_kind", resourceKind),
		attribute.String("resource_namespace", resourceNamespace),
	}
	m.clientQueriesMetric.Add(ctx, 1, metric.WithAttributes(commonLabels...))
}

func initKyvernoInfoMetric(m *MetricsConfig) {
	info := GetKyvernoInfo()
	commonLabels := []attribute.KeyValue{
		attribute.String("version", info.Version),
	}
	m.kyvernoInfoMetric.Record(context.Background(), 1, metric.WithAttributes(commonLabels...))
}

func NewMetricsConfigManager(logger logr.Logger, metricsConfiguration kconfig.MetricsConfiguration) *MetricsConfig {
	config := &MetricsConfig{
		Log:                 logger,
		config:              metricsConfiguration,
		breakerMetrics:      &breakerMetrics{logger: logger.WithName("circuit-breaker")},
		controllerMetrics:   &controllerMetrics{logger: logger.WithName("controller")},
		cleanupMetrics:      &cleanupMetrics{logger: logger.WithName("cleanup")},
		deletingMetrics:     &deletingMetrics{logger: logger.WithName("deleting")},
		policyRuleMetrics:   &policyRuleMetrics{logger: logger.WithName("policy-rule")},
		ttlInfoMetrics:      &ttlInfoMetrics{logger: logger.WithName("ttl-info")},
		policyEngineMetrics: &policyEngineMetrics{logger: logger.WithName("policy-engine")},
		eventMetrics:        &eventMetrics{logger: logger.WithName("event")},
		admissionMetrics:    &admissionMetrics{logger: logger.WithName("admission")},
		httpMetrics:         &httpMetrics{logger: logger.WithName("http")},
		vpolMetrics:         &validatingMetrics{logger: logger.WithName("validating-policy")},
		ivpolMetrics:        &imageValidatingMetrics{logger: logger.WithName("image-validating-policy")},
		mpolMetrics:         &mutatingMetrics{logger: logger.WithName("mutating-policy")},
	}

	return config
}
