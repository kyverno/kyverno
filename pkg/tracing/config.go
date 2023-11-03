package tracing

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	tlsutils "github.com/kyverno/kyverno/pkg/utils/tls"
	"github.com/kyverno/kyverno/pkg/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"k8s.io/client-go/kubernetes"
)

// NewTraceConfig generates the initial tracing configuration with 'address' as the endpoint to connect to the Opentelemetry Collector
func NewTraceConfig(log logr.Logger, tracerName, address, certs string, kubeClient kubernetes.Interface) (func(), error) {
	ctx := context.Background()
	var client otlptrace.Client
	if certs != "" {
		// here the certificates are stored as configmaps
		transportCreds, err := tlsutils.FetchCert(ctx, certs, kubeClient)
		if err != nil {
			log.Error(err, "Error fetching certificate from secret")
		}
		client = otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(address),
			otlptracegrpc.WithTLSCredentials(transportCreds),
		)
	} else {
		client = otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(address),
			otlptracegrpc.WithInsecure(),
		)
	}
	// create New Exporter for exporting metrics
	traceExp, err := otlptrace.New(ctx, client)
	if err != nil {
		log.Error(err, "Failed to create the collector exporter")
		return nil, err
	}
	res, err := resource.New(
		ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(tracerName),
			semconv.ServiceVersionKey.String(version.Version()),
		),
		resource.WithFromEnv(),
	)
	if err != nil {
		log.Error(err, "failed creating resource")
		return nil, err
	}
	// create controller and bind the exporter with it
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	// set global propagator to tracecontext (the default is no-op).
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		// pushes any last exports to the receiver
		if err := tp.Shutdown(ctx); err != nil {
			otel.Handle(err)
		}
	}, nil
}
