package mpol

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	fakekyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/managedfields"

	"github.com/kyverno/kyverno/pkg/cel/matching"
)

// gateBenchTypeConverter satisfies mpolcompiler.TypeConverterManager without
// hitting an API server, matching the fakeTypeConverter used by the mpol
// engine's own unit tests (pkg/cel/policies/mpol/engine/engine_test.go).
type gateBenchTypeConverter struct{}

func (gateBenchTypeConverter) GetTypeConverter(gvk schema.GroupVersionKind) managedfields.TypeConverter {
	return managedfields.NewDeducedTypeConverter()
}

// gateBenchHandlerDeployment returns a ~50KB (measured: ~50013 bytes with
// containerCount=83) Deployment payload, padded via container env values so
// the benchmark's object-unmarshal and patch-generation cost is
// representative rather than a tiny fixture; matches #17508's fixture size
// for cross-PR comparability.
func gateBenchHandlerDeployment(containerCount int) []byte {
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

// buildGateMutateHandlerPolicy returns a single compiled, clustered
// MutatingPolicy (no TargetMatchConstraints) that adds an "env" label via an
// ApplyConfiguration mutation, for use as the CEL admission gate's mpol
// handler benchmark load (#17509). It must have no namespace and no target
// match constraints so it survives the MutateClustered predicate
// (ClusteredPolicy + NoTargetMatchConstraintPolicy).
func buildGateMutateHandlerPolicy(b *testing.B) mpolengine.Provider {
	b.Helper()
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gate-bench-mpol",
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
	provider, err := mpolengine.NewProvider(
		mpolcompiler.NewCompiler(),
		[]policiesv1beta1.MutatingPolicyLike{mpol},
		nil,
		libs.NewFakeContextProvider(),
	)
	require.NoError(b, err)
	return provider
}

// BenchmarkMpolHandlerMutate measures the full mpol webhook MutateClustered
// path -- RequestFromAdmission, engine Handle, and JSON-patch generation via
// admissionResponse -- using a real in-process engine (not a mock, so the
// patch-generation cost the benchmark exists to cover is actually
// exercised), reportsConfig/urGenerator mocks from handler_test.go (same
// package), and an event.NewFake sink (no cluster).
//
// The handler fires its audit and update-request work in unwaited
// goroutines (`go func() { ... }()`, unlike vpol's wait.Group), so some of
// that async allocation can land after the timer stops or spill across
// iterations; see the wider allocs/op headroom for this benchmark in
// scripts/bench/thresholds.txt.
func BenchmarkMpolHandlerMutate(b *testing.B) {
	provider := buildGateMutateHandlerPolicy(b)
	eng := mpolengine.NewEngine(
		provider,
		func(ns string) *corev1.Namespace {
			return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		},
		matching.NewMatcher(),
		&gateBenchTypeConverter{},
		&libs.FakeContextProvider{},
	)

	h := New(
		nil,
		eng,
		fakekyvernoclient.NewSimpleClientset(),
		&mockReportsConfig{},
		&mockURGenerator{},
		"",
		event.NewFake(),
	)

	logger := logging.WithName("BenchmarkMpolHandlerMutate")
	requestObject := gateBenchHandlerDeployment(83) // ~50KB, see gateBenchHandlerDeployment's doc comment
	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("bench-uid"),
			Operation: admissionv1.Create,
			Kind:      metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			Resource:  metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			Namespace: "default",
			Name:      "nginx",
			Object: runtime.RawExtension{
				Raw: requestObject,
			},
		},
	}

	// MutateClustered derives its policy list from httprouter params, so a
	// bare context.Background() would resolve to an empty list and take the
	// early-return path (no engine call, no patch generation). Wire the
	// benchmark policy's name explicitly, matching the router's real
	// behavior in production.
	ctx := context.WithValue(b.Context(), httprouter.ParamsKey, httprouter.Params{
		{Key: "policies", Value: "/gate-bench-mpol"},
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.MutateClustered(ctx, logger, request, "", time.Now())
	}
}
