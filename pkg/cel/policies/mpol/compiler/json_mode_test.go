package compiler

import (
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/require"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The Kubernetes compiler must refuse JSON mode policies so that any missed
// gate upstream (provider, reconciler, scanner, background) fails loudly
// instead of evaluating a JSON policy against Kubernetes admission requests.
func TestCompile_RejectsJSONMode(t *testing.T) {
	mutation := admissionregistrationv1alpha1.Mutation{
		PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
		JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: `[JSONPatch{op:"add",path:"/a",value:1}]`},
	}
	for _, mode := range []string{"", policieskyvernoio.EvaluationModeKubernetes, policieskyvernoio.EvaluationModeJSON} {
		t.Run(mode, func(t *testing.T) {
			policy := &v1beta1.MutatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "p"},
				Spec:       v1beta1.MutatingPolicySpec{Mutations: []admissionregistrationv1alpha1.Mutation{mutation}},
			}
			if mode != "" {
				policy.Spec.EvaluationConfiguration = &v1beta1.MutatingPolicyEvaluationConfiguration{Mode: mode}
			}
			compiled, errs := NewCompiler().Compile(policy, nil)
			if mode == policieskyvernoio.EvaluationModeJSON {
				require.Nil(t, compiled)
				require.Len(t, errs, 1)
				require.Equal(t, "spec.evaluation.mode", errs[0].Field)
				jsonCompiled, jsonErrs := CompileJSON(policy, nil)
				require.Empty(t, jsonErrs)
				require.NotNil(t, jsonCompiled)
				return
			}
			require.Empty(t, errs)
			require.NotNil(t, compiled)
			_, jsonErrs := CompileJSON(policy, nil)
			require.NotEmpty(t, jsonErrs)
		})
	}
}
