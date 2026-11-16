package vpol

import (
	"fmt"
	"sync"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	fakekyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// gateBenchMetricsOnce forces the process-global metrics manager on for the
// vpol handler benchmark, guarded so it only runs once per test binary and
// never from a regular test. This makes vpolengine.NewMetricWrapper's inner
// metrics non-nil (it silently returns the bare engine otherwise), matching
// production (cmd/kyverno/main.go wires vpolengine.NewMetricWrapper around
// every vpol engine). Because this only runs inside a Benchmark's setup,
// `go test` without `-bench` never executes it and unit-test behavior in
// this package is unchanged.
var gateBenchMetricsOnce sync.Once

func setupGateBenchMetrics() {
	gateBenchMetricsOnce.Do(func() {
		metrics.SetManager(metrics.NewFakeMetricsConfig())
	})
}

// gateBenchHandlerPod is a ~50KB (measured: ~50072 bytes) Pod payload
// (padding via env values) so the benchmark's object-unmarshal and
// admission-review cost is representative, not a tiny fixture; matches
// #17508's fixture size for cross-PR comparability.
func gateBenchHandlerPod() []byte {
	padding := make([]byte, 512)
	for i := range padding {
		padding[i] = byte('a' + i%26)
	}
	containers := ""
	for i := 0; i < 83; i++ {
		if i > 0 {
			containers += ","
		}
		containers += fmt.Sprintf(`{"name":"container-%d","image":"nginx:1.21","env":[{"name":"BENCH_PADDING","value":%q}]}`, i, string(padding))
	}
	return []byte(fmt.Sprintf(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "bench-pod",
			"namespace": "default",
			"uid": "00000000-0000-0000-0000-000000000001"
		},
		"spec": {
			"containers": [%s]
		}
	}`, containers))
}

// buildGateValidatePolicy returns a single compiled always-true
// ValidatingPolicy for use as the CEL admission gate's vpol handler
// benchmark load (#17509).
func buildGateValidatePolicy(b *testing.B) vpolengine.Provider {
	b.Helper()
	pol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gate-bench-validate"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "request.object.metadata.name == request.object.metadata.name"},
			},
		},
	}
	provider, err := vpolengine.NewProvider(vpolcompiler.NewCompiler(), []policiesv1beta1.ValidatingPolicyLike{pol}, nil)
	require.NoError(b, err)
	return provider
}

// BenchmarkVpolHandlerValidate measures the full vpol webhook Validate path
// -- RequestFromAdmission, engine Handle, the async audit fan-out (which
// runs under a wait.Group with a deferred Wait, so its allocations land
// inside the benchmark loop deterministically), and admissionResponse --
// with admissionReports disabled (so no report is built or persisted) and
// an event.NewFake sink (no cluster).
//
// A real matching.Matcher is wired in (not nil), matching production
// (cmd/kyverno/main.go wires matching.NewMatcher()); the vpol engine skips
// MatchConstraints evaluation entirely when its matcher is nil, which would
// let a per-policy matcher regression escape this gate. The engine is also
// wrapped with vpolengine.NewMetricWrapper (fake metrics manager forced on
// via setupGateBenchMetrics), matching production wiring exactly, so the
// per-request RecordDuration/RecordResult allocations this benchmark claims
// to cover the "full production handler path" for are actually gated.
func BenchmarkVpolHandlerValidate(b *testing.B) {
	setupGateBenchMetrics()
	provider := buildGateValidatePolicy(b)
	noopNsResolver := func(string) *corev1.Namespace { return nil }
	eng := vpolengine.NewMetricWrapper(vpolengine.NewEngine(provider, noopNsResolver, matching.NewMatcher()), metrics.AdmissionRequest)

	h := New(
		eng,
		libs.NewFakeContextProvider(),
		fakekyvernoclient.NewSimpleClientset(),
		false, // admissionReports
		event.NewFake(),
	)

	logger := logging.WithName("BenchmarkVpolHandlerValidate")
	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("bench-uid"),
			Operation: admissionv1.Create,
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Namespace: "default",
			Name:      "bench-pod",
			Object: runtime.RawExtension{
				Raw: gateBenchHandlerPod(),
			},
		},
	}

	ctx := b.Context()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.ValidateClustered(ctx, logger, request, "", time.Now())
	}
}
