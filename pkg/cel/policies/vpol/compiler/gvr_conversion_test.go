package compiler

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// gvrStubContext wraps a real FakeContextProvider (used by the CLI and by
// every other in-repo CEL unit test) and only overrides ToGVR, mirroring what
// pkg/cel/libs/context.go's production contextProvider.ToGVR actually returns
// for a real restMapper.RESTMapping lookup on core/v1 Pod: a
// *schema.GroupVersionResource{Version:"v1", Resource:"pods"}.
type gvrStubContext struct {
	*libs.FakeContextProvider
}

func (s gvrStubContext) ToGVR(apiVersion, kind string) (*schema.GroupVersionResource, error) {
	return &schema.GroupVersionResource{Version: "v1", Resource: "pods"}, nil
}

// TestResourceToGVR_RealVpolEnv reproduces kyverno/kyverno#17744 through
// Kyverno's real ValidatingPolicy compiler pipeline (createBaseVpolEnv's
// resource.Lib(...) wiring -- the exact env production admission uses), not
// just the sdk lib in isolation. The policy is a byte-for-byte match of the
// real, already-in-repo conformance fixture
// test/conformance/chainsaw/validating-policies/context/resource/get/policy.yaml
// (`variables.gvr = resource.ToGVR("v1", "Pod")`), the deterministic
// always-failing "vpol context/resource/get" conformance job named in
// ci_e2e_gate_tests_red_2026_09 / issue #17744 (Dreamstick9).
//
// Since the cel-go v0.30.0 -> v0.31.0 bump (#17067), NativeToValue only
// converts native Go types explicitly registered with the CEL env via
// ext.NativeTypes(...); resource.Lib's CompileOptions
// (~/go/pkg/mod/.../sdk/extensions/cel/libs/resource/lib.go) registers only
// the `Context` interface type, never schema.GroupVersionResource, so
// resource.impl.go's `convert_to_gvr_string_string` handing NativeToValue a
// bare *schema.GroupVersionResource fails at evaluation time.
//
// This is entirely inside the external kyverno/sdk module (impl.go's
// NativeToValue call, lib.go's CompileOptions) -- Kyverno's own code never
// touches the value between ToGVR() returning it and NativeToValue rejecting
// it, so there is no in-repo fix available; it must be fixed in kyverno/sdk
// (e.g. registering schema.GroupVersionResource via ext.NativeTypes) and
// consumed here via a kyverno/sdk version bump. Coordinate with Dreamstick9
// (#17744), don't duplicate their fix.
func TestResourceToGVR_RealVpolEnv(t *testing.T) {
	// Skipped, not deleted: this reproduces an upstream kyverno/sdk bug
	// (verified 2026-09-27, error "unsupported conversion to ref.Val:
	// (*schema.GroupVersionResource)/v1, Resource=pods"), not a kyverno/kyverno
	// one -- the broken NativeToValue call is entirely inside
	// extensions/cel/libs/resource/impl.go's convert_to_gvr_string_string, which
	// this repo cannot reach or patch around. Tracked upstream as #17744
	// (Dreamstick9, OPEN). Remove this Skip once kyverno/sdk registers
	// schema.GroupVersionResource as a native CEL type and this repo's go.mod
	// picks up that version; at that point this test should go green unmodified.
	t.Skip("kyverno/kyverno#17744: resource.ToGVR() fails until kyverno/sdk registers schema.GroupVersionResource as a native CEL type (fix owned upstream)")

	fake := libs.NewFakeContextProvider()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "policy-pod",
			Namespace: "test-context-get",
			Labels:    map[string]string{"env": "prod"},
		},
	}
	require.NoError(t, fake.AddResource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}, pod))

	prev := libs.LibraryContext
	libs.LibraryContext = gvrStubContext{FakeContextProvider: fake}
	t.Cleanup(func() { libs.LibraryContext = prev })

	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check-deployment-labels"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
							},
						},
					},
				},
			},
			Variables: []admissionregistrationv1.Variable{
				{Name: "gvr", Expression: `resource.ToGVR("v1", "Pod")`},
				{Name: "pod", Expression: `resource.get(variables.gvr, object.metadata.namespace, "policy-pod")`},
				{Name: "environment", Expression: `has(object.metadata.labels) && 'env' in object.metadata.labels && object.metadata.labels['env'] == variables.pod.metadata.labels.env`},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "variables.environment == true",
					Message:    "Deployment labels must be env=prod",
				},
			},
		},
	}

	compiled, errs := NewCompiler().Compile(policy, nil)
	require.Empty(t, errs, "policy must compile cleanly, same as the real conformance fixture")
	require.NotNil(t, compiled)

	gvrProgram, ok := compiled.variables["gvr"]
	require.True(t, ok, "compiler must have produced a program for the gvr variable")

	out, _, err := gvrProgram.ContextEval(context.Background(), map[string]any{})
	require.NoError(t, err, "resource.ToGVR(\"v1\", \"Pod\") must not fail at evaluation time")
	t.Logf("gvr=%v", out)
}
