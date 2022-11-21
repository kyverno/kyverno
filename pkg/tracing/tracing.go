package tracing

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/kyverno/pkg/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
)


// NewTraceConfig generates the initial tracing configuration with 'address' as the endpoint to connect to the Opentelemetry Collector
func NewTraceConfig(log logr.Logger, name, address, certs string, kubeClient kubernetes.Interface) (func(), error) {
	ctx := context.Background()

	var client otlptrace.Client

	if certs != "" {
		// here the certificates are stored as configmaps
		transportCreds, err := kube.FetchCert(ctx, certs, kubeClient)
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
			otlptracegrpc.WithDialOption(grpc.WithBlock()),
		)
	}

	// create New Exporter for exporting metrics
	traceExp, err := otlptrace.New(ctx, client)
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

	// create controller and bind the exporter with it
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})
	otel.SetTracerProvider(tp)
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		// pushes any last exports to the receiver
		if err := tp.Shutdown(ctx); err != nil {
			otel.Handle(err)
		}
	}, nil
}

// DoInSpan executes function doFn inside new span with `operationName` name and hooking as child to a span found within given context if any.
func DoInSpan(ctx context.Context, tracerName string, operationName string, doFn func(context.Context)) {
	newCtx, span := otel.Tracer(tracerName).Start(ctx, operationName)
	defer span.End()
	doFn(newCtx)
}

// StartSpan creates a span from a context with `operationName` name
func StartSpan(ctx context.Context, tracerName string, operationName string, attributes ...attribute.KeyValue) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, operationName, trace.WithAttributes(attributes...))
}

// Span executes function doFn inside new span with `operationName` name and hooking as child to a span found within given context if any.
func Span(ctx context.Context, tracerName string, operationName string, doFn func(context.Context, trace.Span), opts ...trace.SpanStartOption) {
	newCtx, span := otel.Tracer(tracerName).Start(ctx, operationName, opts...)
	defer span.End()
	doFn(newCtx, span)
}

// Span executes function doFn inside new span with `operationName` name and hooking as child to a span found within given context if any.
func Span1[T any](ctx context.Context, tracerName string, operationName string, doFn func(context.Context, trace.Span) T, opts ...trace.SpanStartOption) T {
	newCtx, span := otel.Tracer(tracerName).Start(ctx, operationName, opts...)
	defer span.End()
	return doFn(newCtx, span)
}
