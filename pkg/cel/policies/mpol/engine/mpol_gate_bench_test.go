package engine

import (
	"context"
	"fmt"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// gateBenchDeployment returns a ~50KB (measured: ~50013 bytes with
// containerCount=83) Deployment payload, padded via container env values so
// the mpol engine's object-prep and patch-generation cost is measured
// against a realistically sized object rather than a tiny fixture; matches
// #17508's fixture size for cross-PR comparability.
func gateBenchDeployment(containerCount int) []byte {
	padding := make([]byte, 512)
	for i := range padding {
		padding[i] = byte('a' + i%26)
	}
	containers := ""
	for i := 0; i < containerCount; i++ {
		if i > 0 {
			containers += ","
		}
		containers += fmt.Sprintf(`{"name":"container-%d","image":"nginx:1.21","env":[{"name":"BENCH_PADDING","value":%q}]}`, i, string(padding))
	}
	return []byte(fmt.Sprintf(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
			"name": "nginx",
			"namespace": "default"
		},
		"spec": {
			"template": {
				"spec": {
					"containers": [%s]
				}
			}
		}
	}`, containers))
}

// buildGateMutatePolicy returns a single compiled MutatingPolicy that adds
// an "env" label via an ApplyConfiguration mutation, for use as the CEL
// admission gate's mpol engine benchmark load (#17509).
func buildGateMutatePolicy(b *testing.B) Provider {
	b.Helper()
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gate-bench-add-label",
		},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
							},
						},
					},
				},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{
				{
					PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
						Expression: `Object{metadata: Object.metadata{labels: {"env": "bench"}}}`,
					},
				},
			},
		},
	}

	provider, err := NewProvider(
		compiler.NewCompiler(),
		[]policiesv1beta1.MutatingPolicyLike{mpol},
		nil,
		libs.NewFakeContextProvider(),
	)
	require.NoError(b, err)
	return provider
}

// BenchmarkEngineHandleMpol measures the mpol engine's admission-hot-path
// Handle call for a single mutation policy, built in-process using the
// existing mpol engine unit-test harness (fakeTypeConverter,
// libs.FakeContextProvider). Unlike BenchmarkMpolHandlerMutate, the engine
// here is intentionally left unwrapped by mpolengine.NewMetricWrapper: this
// benchmark isolates the engine core (matching + CEL evaluation + patch
// building), while the webhook handler benchmark already covers the full
// production wiring, metrics included.
func BenchmarkEngineHandleMpol(b *testing.B) {
	provider := buildGateMutatePolicy(b)
	eng := NewEngine(
		provider,
		func(ns string) *corev1.Namespace {
			return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		},
		matching.NewMatcher(),
		&fakeTypeConverter{},
		&libs.FakeContextProvider{},
	)

	dryRun := true
	requestObject := gateBenchDeployment(83) // ~50KB, see gateBenchDeployment's doc comment
	req := engine.EngineRequest{
		Request: admissionv1.AdmissionRequest{
			Kind:      metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			Resource:  metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			Namespace: "default",
			Name:      "nginx",
			Operation: admissionv1.Create,
			// OldObject is intentionally left empty: a real CREATE has no old
			// object, and ExtractResources unconditionally parses non-empty
			// OldObject.Raw, so setting it here would measure parse cost
			// production never incurs on this operation.
			Object: runtime.RawExtension{
				Raw: requestObject,
			},
			DryRun: &dryRun,
		},
	}
	predicate := func(policiesv1beta1.MutatingPolicyLike) bool { return true }

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := eng.Handle(context.Background(), req, predicate); err != nil {
			b.Fatalf("Handle failed: %v", err)
		}
	}
}
