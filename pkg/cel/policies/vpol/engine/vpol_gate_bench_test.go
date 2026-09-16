package engine

import (
	"context"
	"fmt"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// gateBenchPod builds a Pod-shaped unstructured object with roughly
// containerCount containers, each carrying a modest amount of padding, to
// approximate a realistically sized admitted object without depending on an
// external fixture file. containerCount=83 (used below) yields a ~50KB
// (49983-byte) object, matching #17508's fixture size for cross-PR
// comparability.
func gateBenchPod(containerCount int) *unstructured.Unstructured {
	padding := make([]byte, 512)
	for i := range padding {
		padding[i] = byte('a' + i%26)
	}
	env := map[string]any{"BENCH_PADDING": string(padding)}
	containers := make([]any, 0, containerCount)
	for i := 0; i < containerCount; i++ {
		containers = append(containers, map[string]any{
			"name":  fmt.Sprintf("container-%d", i),
			"image": "nginx:1.21",
			"env": []any{
				map[string]any{"name": "BENCH_PADDING", "value": env["BENCH_PADDING"]},
			},
		})
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":      "bench-pod",
			"namespace": "default",
		},
		"spec": map[string]any{
			"containers": containers,
		},
	}}
}

// buildGateAlwaysTruePolicies returns n compiled ValidatingPolicies, each
// with a trivially-true validation expression, for use as benchmark load -
// the goal is to exercise the per-policy loop, not CEL evaluation cost
// itself.
func buildGateAlwaysTruePolicies(b *testing.B, n int) Provider {
	b.Helper()
	policies := make([]policiesv1beta1.ValidatingPolicyLike, 0, n)
	for i := 0; i < n; i++ {
		policies = append(policies, &policiesv1beta1.ValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("gate-bench-policy-%d", i)},
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
		})
	}
	provider, err := NewProvider(compiler.NewCompiler(), policies, nil)
	require.NoError(b, err)
	return provider
}

// BenchmarkEngineHandleVpol measures the vpol engine's admission-hot-path
// Handle call with N matched policies and a realistically sized admitted
// object, for the CEL admission gate (#17509). It uses a distinct name from
// #17572's BenchmarkHandle so the two can coexist regardless of merge order.
//
// A real matching.Matcher is wired in (not nil) so the benchmark exercises
// the same per-policy MatchConstraints evaluation production uses
// (cmd/kyverno/main.go wires matching.NewMatcher()); Handle skips
// MatchConstraints evaluation entirely when the matcher is nil, which would
// let a per-policy matcher regression escape this gate. Unlike
// BenchmarkVpolHandlerValidate, the engine here is intentionally left
// unwrapped by vpolengine.NewMetricWrapper: this benchmark isolates the
// engine core (matching + CEL evaluation), while the webhook handler
// benchmark already covers the full production wiring, metrics included.
func BenchmarkEngineHandleVpol(b *testing.B) {
	for _, n := range []int{1, 16, 64} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			provider := buildGateAlwaysTruePolicies(b, n)
			noopNsResolver := func(string) *corev1.Namespace { return nil }
			eng := NewEngine(provider, noopNsResolver, matching.NewMatcher())
			pod := gateBenchPod(83) // ~50KB, see gateBenchPod's doc comment
			req := celengine.Request(
				nil,
				schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
				schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
				"",
				"bench-pod",
				"default",
				admissionv1.Create,
				authenticationv1.UserInfo{},
				pod,
				nil,
				false,
				nil,
			)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := eng.Handle(context.Background(), req, nil); err != nil {
					b.Fatalf("Handle failed: %v", err)
				}
			}
		})
	}
}
