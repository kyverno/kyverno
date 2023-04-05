package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	kconfig "github.com/kyverno/kyverno/pkg/config"
	tlsutils "github.com/kyverno/kyverno/pkg/utils/tls"
	"github.com/kyverno/kyverno/pkg/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/instrument"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"k8s.io/client-go/kubernetes"
)

const (
	MeterName = "kyverno"
)

type MetricsConfig struct {
	// instruments
	policyChangesMetric instrument.Int64Counter
	clientQueriesMetric instrument.Int64Counter

	// config
	config kconfig.MetricsConfiguration
	Log    logr.Logger
}

type MetricsConfigManager interface {
	Config() kconfig.MetricsConfiguration
	RecordPolicyChanges(ctx context.Context, policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, policyChangeType string)
	RecordClientQueries(ctx context.Context, clientQueryOperation ClientQueryOperation, clientType ClientType, resourceKind string, resourceNamespace string)
}

func (m *MetricsConfig) Config() kconfig.MetricsConfiguration {
	return m.config
}

func (m *MetricsConfig) initializeMetrics(meterProvider metric.MeterProvider) error {
	var err error
	meter := meterProvider.Meter(MeterName)
	m.policyChangesMetric, err = meter.Int64Counter("kyverno_policy_changes", instrument.WithDescription("can be used to track all the changes associated with the Kyverno policies present on the cluster such as creation, updates and deletions"))
	if err != nil {
		m.Log.Error(err, "Failed to create instrument, kyverno_policy_changes")
		return err
	}
	m.clientQueriesMetric, err = meter.Int64Counter("kyverno_client_queries", instrument.WithDescription("can be used to track the number of client queries sent from Kyverno to the API-server"))
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

func aggregationSelector(ik sdkmetric.InstrumentKind) aggregation.Aggregation {
	switch ik {
	case sdkmetric.InstrumentKindHistogram:
		return aggregation.ExplicitBucketHistogram{
			Boundaries: []float64{
				0.005,
				0.01,
				0.025,
				0.05,
				0.1,
				0.25,
				0.5,
				1,
				2.5,
				5,
				10,
				15,
				20,
				25,
				30,
			},
			NoMinMax: false,
		}
	default:
		return sdkmetric.DefaultAggregationSelector(ik)
	}
}

func NewOTLPGRPCConfig(
	ctx context.Context,
	endpoint string,
	certs string,
	kubeClient kubernetes.Interface,
	log logr.Logger,
) (metric.MeterProvider, error) {
	options := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(endpoint), otlpmetricgrpc.WithAggregationSelector(aggregationSelector)}
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
		prometheus.WithAggregationSelector(aggregationSelector),
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

func (m *MetricsConfig) RecordClientQueries(ctx context.Context, clientQueryOperation ClientQueryOperation, clientType ClientType, resourceKind string, resourceNamespace string) {
	commonLabels := []attribute.KeyValue{
		attribute.String("operation", string(clientQueryOperation)),
		attribute.String("client_type", string(clientType)),
		attribute.String("resource_kind", resourceKind),
		attribute.String("resource_namespace", resourceNamespace),
	}
	m.clientQueriesMetric.Add(ctx, 1, commonLabels...)
}
