package tracing

import (
	"context"
	"crypto/x509"
	"fmt"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NewTraceConfig generates the initial tracing configuration with 'endpoint' as the endpoint to connect to the Opentelemetry Collector
func NewTraceConfig(endpoint string, certs string, kubeClient kubernetes.Interface, log logr.Logger) error {
	log.Info("Enabling tracing for Kyverno...")
	ctx := context.Background()

	var client otlptrace.Client

	if certs != "" {
		// here the certificates are stored as configmaps
		configmap, err := kubeClient.CoreV1().ConfigMaps("kyverno").Get(ctx, certs, v1.GetOptions{})
		if err != nil {
			log.Error(err, "Error fetching certificate from configmap")
		}

		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM([]byte(configmap.Data["ca.pem"])) {
			return fmt.Errorf("credentials: failed to append certificates")
		}

		transportCreds := credentials.NewClientTLSFromCert(cp, "")

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
		return err
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(semconv.ServiceNameKey.String("kyverno_traces")),
		resource.WithSchemaURL(semconv.SchemaURL),
	)
	if err != nil {
		log.Error(err, "failed creating resource")
		return err
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
	return nil
}

// DoInSpan executes function doFn inside new span with `operationName` name and hooking as child to a span found within given context if any.
// It uses opentracing.Tracer propagated in context. If no found, it uses noop tracer notification.
func DoInSpan(ctx context.Context, tracerName string, operationName string, doFn func(context.Context)) {
	newCtx, span := otel.Tracer(tracerName).Start(ctx, operationName)
	defer span.End()
	doFn(newCtx)
}

func StartSpan(ctx context.Context, tracerName string, operationName string) trace.Span {
	_, span := otel.Tracer(tracerName).Start(ctx, operationName)
	return span
}
