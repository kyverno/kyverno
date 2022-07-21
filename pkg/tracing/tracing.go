package tracing

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/utils/kube"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/client-go/kubernetes"
)

func ShutDownController(ctx context.Context, tp *sdktrace.TracerProvider) {
	// pushes any last exports to the receiver
	if err := tp.Shutdown(ctx); err != nil {
		otel.Handle(err)
	}
}

// NewTraceConfig generates the initial tracing configuration with 'endpoint' as the endpoint to connect to the Opentelemetry Collector
func NewTraceConfig(endpoint string, certs string, kubeClient kubernetes.Interface, log logr.Logger) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	var client otlptrace.Client

	if certs != "" {
		// here the certificates are stored as configmaps
		transportCreds, err := kube.FetchCert(ctx, certs, kubeClient)
		if err != nil {
			log.Error(err, "Error fetching certificate from secret")
		}

		client = otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithTLSCredentials(transportCreds),
		)
	} else {
		client = otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithInsecure(),
		)
	}

	// create New Exporter for exporting metrics
	traceExp, err := otlptrace.New(ctx, client)
	if err != nil {
		log.Error(err, "Failed to create the collector exporter")
		return nil, err
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(semconv.ServiceNameKey.String("kyverno_traces")),
		resource.WithSchemaURL(semconv.SchemaURL),
	)
	if err != nil {
		log.Error(err, "failed creating resource")
		return nil, err
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExp)
	// create controller and bind the exporter with it
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(res),
	)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})
	otel.SetTracerProvider(tp)
	return tp, nil
}

// DoInSpan executes function doFn inside new span with `operationName` name and hooking as child to a span found within given context if any.
func DoInSpan(ctx context.Context, tracerName string, operationName string, doFn func(context.Context)) {
	newCtx, span := otel.Tracer(tracerName).Start(ctx, operationName)
	defer span.End()
	doFn(newCtx)
}

// StartSpan creates a span from a context with `operationName` name
func StartSpan(ctx context.Context, tracerName string, operationName string, attributes []attribute.KeyValue) trace.Span {
	_, span := otel.Tracer(tracerName).Start(ctx, operationName, trace.WithAttributes(attributes...))
	return span
}
