package engine

import (
	"context"
	"fmt"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	"github.com/stretchr/testify/assert"
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

// buildNonMatchingPolicies returns n compiled ValidatingPolicies whose
// matchConstraints target "configmaps", never the "pods" resource the
// no-match test below submits - so a real matching.Matcher rejects every
// policy before evaluation, and the hoisted `request` CEL activation value
// (built lazily, see vpol/engine/engine.go and prepareK8sData) is never
// actually needed.
func buildNonMatchingPolicies(t testing.TB, n int) Provider {
	t.Helper()
	policies := make([]policiesv1beta1.ValidatingPolicyLike, 0, n)
	for i := 0; i < n; i++ {
		policies = append(policies, &policiesv1beta1.ValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("no-match-policy-%d", i)},
			Spec: policiesv1beta1.ValidatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"configmaps"},
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
	require.NoError(t, err)
	return provider
}

// buildNoMatchEngineAndRequest builds the Provider/Engine once (compiling
// CEL policies has its own, unrelated allocation cost that must not be
// included in the measurement below) plus a Pod CREATE request of roughly
// containerCount containers, for a request that a real matching.Matcher
// rejects for every registered policy.
func buildNoMatchEngineAndRequest(t testing.TB, containerCount int) (Engine, EngineRequest) {
	t.Helper()
	provider := buildNonMatchingPolicies(t, 16)
	noopNsResolver := func(string) *corev1.Namespace { return nil }
	eng := NewEngine(provider, noopNsResolver, matching.NewMatcher())
	pod := benchPod(containerCount)
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
	return eng, req
}

// TestHandle_NoMatch_SkipsRequestMapBuild is the CI-assertable regression
// guard for the lazy request-map build: a request that matches zero
// policies must not pay the cost of building the `request` CEL activation
// value at all, regardless of the admitted object's size. If the build were
// still eager (as it was when this hoist first landed), allocations would
// scale with the object size - the raw-variant builder marshals/unmarshals
// the whole admitted object once per Handle() call. Comparing a tiny object
// against a realistically large (~50KB) one and asserting the allocation
// counts are close proves the build is skipped, not just cheap.
//
// Engine/Provider construction (which compiles CEL policies - its own,
// unrelated and much larger allocation cost) happens once outside the
// AllocsPerRun-measured closure, so only the per-call Handle() cost is
// measured.
func TestHandle_NoMatch_SkipsRequestMapBuild(t *testing.T) {
	smallEng, smallReq := buildNoMatchEngineAndRequest(t, 1)
	largeEng, largeReq := buildNoMatchEngineAndRequest(t, 20)

	smallAllocs := testing.AllocsPerRun(50, func() {
		if _, err := smallEng.Handle(context.Background(), smallReq, nil); err != nil {
			t.Fatalf("Handle failed: %v", err)
		}
	})
	largeAllocs := testing.AllocsPerRun(50, func() {
		if _, err := largeEng.Handle(context.Background(), largeReq, nil); err != nil {
			t.Fatalf("Handle failed: %v", err)
		}
	})

	t.Logf("no-match allocs/op: small object=%.1f, large object=%.1f", smallAllocs, largeAllocs)
	// A generous absolute margin (not a ratio, since both counts are small
	// to begin with) - if the request map were built eagerly, the ~50KB
	// object's JSON marshal/unmarshal would add allocations proportional to
	// its size, blowing well past this margin.
	assert.InDelta(t, smallAllocs, largeAllocs, 30,
		"a request matching zero policies must not build the request map: allocs must stay ~flat regardless of admitted object size")
}
