package metrics

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernoconfig "github.com/kyverno/kyverno/pkg/config"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
)

// fakeCounter is a mock for the Int64Counter instrument.
type fakeCounter struct {
	noop.Int64Counter
	mu         sync.Mutex
	calls      int
	attributes []attribute.KeyValue
}

func (f *fakeCounter) Add(ctx context.Context, value int64, opts ...metric.AddOption) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	cfg := metric.NewAddConfig(opts)
	attrSet := cfg.Attributes()
	it := (&attrSet).Iter()
	f.attributes = append(f.attributes, (&it).ToSlice()...)
}

// fakeHistogram is a mock for the Float64Histogram instrument.
type fakeHistogram struct {
	noop.Float64Histogram
	mu         sync.Mutex
	calls      int
	lastValue  float64
	attributes []attribute.KeyValue
}

func (f *fakeHistogram) Record(ctx context.Context, value float64, opts ...metric.RecordOption) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastValue = value
	cfg := metric.NewRecordConfig(opts)
	attrSet := cfg.Attributes()
	it := (&attrSet).Iter()
	f.attributes = append(f.attributes, (&it).ToSlice()...)
}

// setupAdmissionMetrics is a helper function to initialize metrics with fake instruments.
func setupAdmissionMetrics(t *testing.T, cfg kyvernoconfig.MetricsConfiguration) (*admissionMetrics, *fakeCounter, *fakeHistogram) {
	// Use the real metrics manager with the provided configuration.
	// This assumes a SetManager function exists to set the package-level manager.
	// If GetManager() returns a package-level variable, we would set that instead.
	SetManager(NewMetricsConfigManager(logr.Discard(), cfg))

	counter := &fakeCounter{}
	hist := &fakeHistogram{}

	// Create the struct under test and inject our fake instruments.
	return &admissionMetrics{
		requestsMetric: counter,
		durationMetric: hist,
		logger:         logr.Discard(),
	}, counter, hist
}

func TestRecordRequest_NoMetrics_DoesNotPanic(t *testing.T) {
	cfg := kyvernoconfig.NewDefaultMetricsConfiguration()
	SetManager(NewMetricsConfigManager(logr.Discard(), cfg))

	// The struct under test has no instruments, so it should not panic.
	m := &admissionMetrics{
		logger: logr.Discard(),
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RecordRequest panicked: %v", r)
		}
	}()

	m.RecordRequest(
		context.Background(),
		true,
		"default",
		admissionv1.Create,
		"Pod",
		time.Now(),
	)
}

func TestRecordRequest_NamespaceFiltered(t *testing.T) {
	cfg := kyvernoconfig.NewDefaultMetricsConfiguration()
	// Fix: Configure the real config to only allow the "default" namespace
	// by creating a fake configmap and loading it.
	cm := &corev1.ConfigMap{
		Data: map[string]string{
			"namespaces": `{"include":["default"]}`,
		},
	}
	cfg.Load(cm)

	m, counter, hist := setupAdmissionMetrics(t, cfg)

	// This request is for "kube-system", which should be filtered out.
	m.RecordRequest(
		context.Background(),
		true,
		"kube-system",
		admissionv1.Create,
		"Pod",
		time.Now(),
	)

	if counter.calls != 0 {
		t.Fatalf("expected no counter calls, got %d", counter.calls)
	}
	if hist.calls != 0 {
		t.Fatalf("expected no histogram calls, got %d", hist.calls)
	}
}

func TestRecordRequest_MetricsInvoked(t *testing.T) {
	cfg := kyvernoconfig.NewDefaultMetricsConfiguration()
	// Configure to allow all namespaces (default behavior).
	m, counter, hist := setupAdmissionMetrics(t, cfg)

	m.RecordRequest(
		context.Background(),
		true,
		"default",
		admissionv1.Create,
		"Pod",
		time.Now().Add(-200*time.Millisecond),
	)

	if counter.calls != 1 {
		t.Fatalf("expected 1 counter call, got %d", counter.calls)
	}
	if hist.calls != 1 {
		t.Fatalf("expected 1 histogram call, got %d", hist.calls)
	}
}

func TestRecordRequest_AttributesCorrect(t *testing.T) {
	cfg := kyvernoconfig.NewDefaultMetricsConfiguration()
	m, counter, _ := setupAdmissionMetrics(t, cfg)

	m.RecordRequest(
		context.Background(),
		false,
		"default",
		admissionv1.Update,
		"Deployment",
		time.Now(),
	)

	// Collect attributes by key
	attrMap := map[string]attribute.Value{}
	for _, a := range counter.attributes {
		attrMap[string(a.Key)] = a.Value
	}

	// String attributes
	if v := attrMap["resource_kind"]; v.AsString() != "Deployment" {
		t.Fatalf("expected resource_kind=Deployment, got %v", v)
	}
	if v := attrMap["resource_namespace"]; v.AsString() != "default" {
		t.Fatalf("expected resource_namespace=default, got %v", v)
	}
	if v := attrMap["resource_request_operation"]; v.AsString() != "update" {
		t.Fatalf("expected resource_request_operation=update, got %v", v)
	}

	// Boolean attribute (correct usage)
	if v := attrMap["request_allowed"]; v.AsBool() != false {
		t.Fatalf("expected request_allowed=false, got %v", v)
	}
}

func TestRecordRequest_DurationRecorded(t *testing.T) {
	cfg := kyvernoconfig.NewDefaultMetricsConfiguration()
	m, _, hist := setupAdmissionMetrics(t, cfg)

	start := time.Now().Add(-500 * time.Millisecond)
	m.RecordRequest(
		context.Background(),
		true,
		"default",
		admissionv1.Create,
		"Pod",
		start,
	)

	if hist.lastValue <= 0 {
		t.Fatalf("expected positive duration, got %f", hist.lastValue)
	}
}
