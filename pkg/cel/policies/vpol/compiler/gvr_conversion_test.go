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

// gvrStubContext returns the GVR a real RESTMapper returns for core/v1 Pod.
type gvrStubContext struct {
	*libs.FakeContextProvider
}

func (s gvrStubContext) ToGVR(apiVersion, kind string) (*schema.GroupVersionResource, error) {
	return &schema.GroupVersionResource{Version: "v1", Resource: "pods"}, nil
}

// TestResourceToGVR_RealVpolEnv reproduces #17744 through the real ValidatingPolicy
// compiler, with the same variables as the vpol context/resource/get conformance test.
// Since cel-go v0.31.0 (#17067) NativeToValue only converts registered native types,
// and the kyverno/sdk resource lib hands it a bare *schema.GroupVersionResource.
func TestResourceToGVR_RealVpolEnv(t *testing.T) {
	// The fix belongs in kyverno/sdk; drop this skip once go.mod picks it up.
	t.Skip("#17744: resource.ToGVR fails until kyverno/sdk returns a CEL-convertible GVR")

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
