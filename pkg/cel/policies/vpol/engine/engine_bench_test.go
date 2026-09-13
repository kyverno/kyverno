package engine

import (
	"context"
	"fmt"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
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

// benchPod builds a Pod-shaped unstructured object with roughly containerCount
// containers, each carrying a modest amount of padding, to approximate a
// realistically sized (~50KB) admitted object without depending on an
// external fixture file.
func benchPod(containerCount int) *unstructured.Unstructured {
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

// buildAlwaysTruePolicies returns n compiled ValidatingPolicies, each with a
// trivially-true validation expression, for use as benchmark load - the goal
// is to exercise the per-policy loop and object-preparation cost, not CEL
// evaluation cost itself.
func buildAlwaysTruePolicies(b *testing.B, n int) Provider {
	b.Helper()
	policies := make([]policiesv1beta1.ValidatingPolicyLike, 0, n)
	for i := 0; i < n; i++ {
		policies = append(policies, &policiesv1beta1.ValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("bench-policy-%d", i)},
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

// BenchmarkHandle exercises the real per-policy loop in Handle with N
// matched policies and a realistically sized admitted object. Success
// criterion (recorded in the PR): allocs/op growth from N=1 to N=64 must be
// far below 64x - the object-preparation component (formerly O(N) via a
// repeated ConvertObjectToUnstructured(request) re-parse of the object
// bytes) is now O(1) per Handle() call, so remaining growth should track CEL
// evaluation cost only, not object-prep cost.
func BenchmarkHandle(b *testing.B) {
	for _, n := range []int{1, 16, 64} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			provider := buildAlwaysTruePolicies(b, n)
			noopNsResolver := func(string) *corev1.Namespace { return nil }
			eng := NewEngine(provider, noopNsResolver, nil)
			pod := benchPod(20)
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
