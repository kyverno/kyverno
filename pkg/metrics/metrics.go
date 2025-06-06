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
	internalmetric "go.opentelemetry.io/otel/metric"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"k8s.io/client-go/kubernetes"
)

const (
	MeterName = "kyverno"
)

type MetricsConfig struct {
	// instruments
	policyChangesMetric internalmetric.Int64Counter
	clientQueriesMetric internalmetric.Int64Counter
	kyvernoInfoMetric   internalmetric.Int64Gauge

	// config
	metricsAddress string
	otelProvider   string // prometheus, grpc
	config         kconfig.MetricsConfiguration
	Log            logr.Logger
}

type MetricsConfigManager interface {
	Config() kconfig.MetricsConfiguration
	RecordPolicyChanges(ctx context.Context, policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, policyChangeType string)
	RecordClientQueries(ctx context.Context, clientQueryOperation ClientQueryOperation, clientType ClientType, resourceKind string, resourceNamespace string)
	MetricsProviderMode() string
	MetricsAddress() string
}

func (m *MetricsConfig) Config() kconfig.MetricsConfiguration {
	return m.config
}

func (m *MetricsConfig) MetricsProviderMode() string {
	return m.otelProvider
}
func (m *MetricsConfig) MetricsAddress() string {
	return m.metricsAddress
}

func (m *MetricsConfig) initializeMetrics(meterProvider internalmetric.MeterProvider) error {
	var err error
	meter := meterProvider.Meter(MeterName)
	m.policyChangesMetric, err = meter.Int64Counter("kyverno_policy_changes", internalmetric.WithDescription("can be used to track all the changes associated with the Kyverno policies present on the cluster such as creation, updates and deletions"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_policy_changes")
		return err
	}
	m.clientQueriesMetric, err = meter.Int64Counter("kyverno_client_queries", internalmetric.WithDescription("can be used to track the number of client queries sent from Kyverno to the API-server"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_client_queries")
		return err
	}
	m.kyvernoInfoMetric, err = meter.Int64Gauge("kyverno_info",
		internalmetric.WithDescription("Kyverno version information"),
	)
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_info")
		return err
	}
	initKyvernoInfoMetric(m)
	return nil
}

func ShutDownController(ctx context.Context, pusher *metric.MeterProvider) {
	if pusher != nil {
		// pushes any last exports to the receiver
		if err := pusher.Shutdown(ctx); err != nil {
			otel.Handle(err)
		}
	}
}

func aggregationSelector(metricsConfiguration kconfig.MetricsConfiguration) func(ik metric.InstrumentKind) metric.Aggregation {
	return func(ik metric.InstrumentKind) metric.Aggregation {
		switch ik {
		case metric.InstrumentKindHistogram:
			return metric.AggregationExplicitBucketHistogram{
				Boundaries: metricsConfiguration.GetBucketBoundaries(),
				NoMinMax:   false,
			}
		default:
			return metric.DefaultAggregationSelector(ik)
		}
	}
}

func NewOTLPGRPCConfig(ctx context.Context, endpoint string, certs string, kubeClient kubernetes.Interface, log logr.Logger, configuration kconfig.MetricsConfiguration) (*metric.MeterProvider, error) {
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
	reader := metric.NewPeriodicReader(
		exporter,
		metric.WithInterval(2*time.Second),
	)
	// create controller and bind the exporter with it
	provider := metric.NewMeterProvider(
		metric.WithReader(reader),
		metric.WithResource(res),
		metric.WithView(configuration.BuildMeterProviderViews()...),
	)
	return provider, nil
}

func NewPrometheusConfig(ctx context.Context, log logr.Logger, configuration kconfig.MetricsConfiguration) (*metric.MeterProvider, error) {
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
		// use the registry from controller-runtime metrics
		prometheus.WithRegisterer(metrics.Registry),
		prometheus.WithoutUnits(),
		prometheus.WithoutTargetInfo(),
		prometheus.WithAggregationSelector(aggregationSelector(configuration)),
	)
	if err != nil {
		log.Error(err, "failed to initialize prometheus exporter")
		return nil, err
	}
	provider := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(res),
		metric.WithView(configuration.BuildMeterProviderViews()...),
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
	m.policyChangesMetric.Add(ctx, 1, internalmetric.WithAttributes(commonLabels...))
}

func (m *MetricsConfig) RecordClientQueries(ctx context.Context, clientQueryOperation ClientQueryOperation, clientType ClientType, resourceKind string, resourceNamespace string) {
	commonLabels := []attribute.KeyValue{
		attribute.String("operation", string(clientQueryOperation)),
		attribute.String("client_type", string(clientType)),
		attribute.String("resource_kind", resourceKind),
		attribute.String("resource_namespace", resourceNamespace),
	}
	m.clientQueriesMetric.Add(ctx, 1, internalmetric.WithAttributes(commonLabels...))
}

func initKyvernoInfoMetric(m *MetricsConfig) {
	info := GetKyvernoInfo()
	commonLabels := []attribute.KeyValue{
		attribute.String("version", info.Version),
	}
	m.kyvernoInfoMetric.Record(context.Background(), 1, internalmetric.WithAttributes(commonLabels...))
}
