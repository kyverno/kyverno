package tracing

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/utils/kube"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/client-go/kubernetes"
)

const (
	PolicyGroupKey     = attribute.Key("kyverno.policy.group")
	PolicyVersionKey   = attribute.Key("kyverno.policy.version")
	PolicyKindKey      = attribute.Key("kyverno.policy.kind")
	PolicyNameKey      = attribute.Key("kyverno.policy.name")
	PolicyNamespaceKey = attribute.Key("kyverno.policy.namespace")
	RuleNameKey        = attribute.Key("kyverno.rule.name")
	// ResourceNameKey       = attribute.Key("admission.resource.name")
	// ResourceNamespaceKey  = attribute.Key("admission.resource.namespace")
	// ResourceGroupKey      = attribute.Key("admission.resource.group")
	// ResourceVersionKey    = attribute.Key("admission.resource.version")
	// ResourceKindKey       = attribute.Key("admission.resource.kind")
	// ResourceUidKey        = attribute.Key("admission.resource.uid")
	RequestNameKey                    = attribute.Key("admission.request.name")
	RequestNamespaceKey               = attribute.Key("admission.request.namespace")
	RequestUidKey                     = attribute.Key("admission.request.uid")
	RequestOperationKey               = attribute.Key("admission.request.operation")
	RequestDryRunKey                  = attribute.Key("admission.request.dryrun")
	RequestKindGroupKey               = attribute.Key("admission.request.kind.group")
	RequestKindVersionKey             = attribute.Key("admission.request.kind.version")
	RequestKindKindKey                = attribute.Key("admission.request.kind.kind")
	RequestSubResourceKey             = attribute.Key("admission.request.subresource")
	RequestRequestKindGroupKey        = attribute.Key("admission.request.requestkind.group")
	RequestRequestKindVersionKey      = attribute.Key("admission.request.requestkind.version")
	RequestRequestKindKindKey         = attribute.Key("admission.request.requestkind.kind")
	RequestRequestSubResourceKey      = attribute.Key("admission.request.requestsubresource")
	RequestResourceGroupKey           = attribute.Key("admission.request.resource.group")
	RequestResourceVersionKey         = attribute.Key("admission.request.resource.version")
	RequestResourceResourceKey        = attribute.Key("admission.request.resource.resource")
	RequestRequestResourceGroupKey    = attribute.Key("admission.request.requestresource.group")
	RequestRequestResourceVersionKey  = attribute.Key("admission.request.requestresource.version")
	RequestRequestResourceResourceKey = attribute.Key("admission.request.requestresource.resource")
	RequestUserNameKey                = attribute.Key("admission.request.user.name")
	RequestUserUidKey                 = attribute.Key("admission.request.user.uid")
	RequestUserGroupsKey              = attribute.Key("admission.request.user.groups")
	ResponseUidKey                    = attribute.Key("admission.response.uid")
	ResponseAllowedKey                = attribute.Key("admission.response.allowed")
	ResponseWarningsKey               = attribute.Key("admission.response.warnings")
	ResponseResultStatusKey           = attribute.Key("admission.response.result.status")
	ResponseResultMessageKey          = attribute.Key("admission.response.result.message")
	ResponseResultReasonKey           = attribute.Key("admission.response.result.reason")
	ResponseResultCodeKey             = attribute.Key("admission.response.result.code")
	ResponsePatchTypeKey              = attribute.Key("admission.response.patchtype")
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
		)
	}

	// create New Exporter for exporting metrics
	traceExp, err := otlptrace.New(ctx, client)
	if err != nil {
		log.Error(err, "Failed to create the collector exporter")
		return nil, err
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(semconv.ServiceNameKey.String(name)),
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

func SetHttpStatus(ctx context.Context, err error, code int) {
	span := trace.SpanFromContext(ctx)
	if err != nil {
		span.RecordError(err)
	}
	span.SetAttributes(semconv.HTTPStatusCodeKey.Int(code))
	if code >= 400 {
		span.SetStatus(codes.Error, http.StatusText(code))
	} else {
		span.SetStatus(codes.Ok, http.StatusText(code))
	}
}
