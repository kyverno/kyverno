package metrics

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/propagation"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
)

const (
	meterName  = "kyverno"
	tracerName = "cluster_policy_tracer"
)

func NewOTLPConfig(endpoint string, log logr.Logger) error {
	ctx := context.Background()
	client := otlpmetricgrpc.NewClient(
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithEndpoint(endpoint),
	)

	// create New Exporter for exporting metrics
	metricExp, err := otlpmetric.New(ctx, client)
	if err != nil {
		log.Error(err, "Failed to create the collector exporter")
		return err
	}

	// create controller and bind the exporter with it
	pusher := controller.New(
		processor.NewFactory(
			simple.NewWithHistogramDistribution(),
			metricExp,
		),
		controller.WithExporter(metricExp),
		controller.WithCollectPeriod(2*time.Second),
	)
	global.SetMeterProvider(pusher)
	// meterProvider = pusher

	if err := pusher.Start(ctx); err != nil {
		log.Error(err, "could not start metric exporter")
		return err
	}

	traceClient := otlptracegrpc.NewClient(
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithDialOption(grpc.WithBlock()))
	traceExp, err := otlptrace.New(ctx, traceClient)
	if err != nil {
		log.Error(err, "Count not create collector trace exporter")
		return err
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExp)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(bsp),
	)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})
	otel.SetTracerProvider(tracerProvider)

	return nil
}

func RecordPolicyResults(policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string,
	resourceKind string, resourceNamespace string, resourceRequestOperation ResourceRequestOperation, ruleName string, ruleResult RuleResult, ruleType RuleType,
	ruleExecutionCause RuleExecutionCause, log logr.Logger) {
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

	meter := global.MeterProvider().Meter(meterName)
	policyResultsCounter, err := meter.SyncInt64().Counter("kyverno_policy_results_total")

	if err != nil {
		log.Error(err, "Failed to create the instrument")
	}

	policyResultsCounter.Add(ctx, 1, commonLabels...)
}

func RecordPolicyChanges(policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string, policyChangeType string, log logr.Logger) {
	ctx := context.Background()

	commonLabels := []attribute.KeyValue{
		attribute.String("policy_validation_mode", string(policyValidationMode)),
		attribute.String("policy_type", string(policyType)),
		attribute.String("policy_background_mode", string(policyBackgroundMode)),
		attribute.String("policy_namespace", policyNamespace),
		attribute.String("policy_name", policyName),
		attribute.String("policy_change_type", policyChangeType),
	}

	meter := global.MeterProvider().Meter(meterName)
	policyChangesCounter, err := meter.SyncInt64().Counter("kyverno_policy_changes_total")

	if err != nil {
		log.Error(err, "Failed to create the instrument")
	}

	policyChangesCounter.Add(ctx, 1, commonLabels...)
}

func RecordPolicyRuleInfo(policyValidationMode PolicyValidationMode, policyType PolicyType, policyBackgroundMode PolicyBackgroundMode, policyNamespace string, policyName string,
	ruleName string, ruleType RuleType, status string, metricValue float64, log logr.Logger) {

	// labels to be associated with a cluster policy
	commonLabels := []attribute.KeyValue{
		attribute.String("policy_validation_mode", string(policyValidationMode)),
		attribute.String("policy_type", string(policyType)),
		attribute.String("policy_background_mode", string(policyBackgroundMode)),
		attribute.String("policy_namespace", policyNamespace),
		attribute.String("policy_name", policyName),
		attribute.String("rule_name", ruleName),
		attribute.String("rule_name", string(ruleType)),
		attribute.String("status", status),
	}

	meter := global.MeterProvider().Meter(meterName)
	ruleInfoRecorder, err := meter.AsyncFloat64().Gauge("kyverno_policy_rule_info_total")

	if err != nil {
		log.Error(err, "Failed to create the instrument")
	}

	err = meter.RegisterCallback([]instrument.Asynchronous{ruleInfoRecorder},
		func(ctx context.Context) {
			ruleInfoRecorder.Observe(ctx, metricValue, commonLabels...)
		})
	if err != nil {
		log.Error(err, "Failed to record rule info metrics")
	}
}

func RecordAdmissionRequests(resourceKind string, resourceNamespace string, resourceRequestOperation ResourceRequestOperation, log logr.Logger) {
	ctx := context.Background()

	commonLabels := []attribute.KeyValue{
		attribute.String("resource_kind", resourceKind),
		attribute.String("resource_namespace", resourceNamespace),
		attribute.String("resource_request_operation", string(resourceRequestOperation)),
	}

	// create a meter
	meter := global.MeterProvider().Meter(meterName)
	admissionRequestsCounter, err := meter.SyncInt64().Counter("kyverno_admission_requests_total")

	if err != nil {
		log.Error(err, "Failed to create the instrument")
	}

	admissionRequestsCounter.Add(ctx, 1, commonLabels...)
}
