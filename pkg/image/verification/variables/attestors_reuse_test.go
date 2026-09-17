package variables

import (
	"fmt"
	"testing"

	"github.com/google/cel-go/cel"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestCompiledAttestorConcurrentEvaluation(t *testing.T) {
	env, err := cel.NewEnv(cel.Variable("value", cel.StringType))
	require.NoError(t, err)
	definition := policiesv1beta1.Attestor{Name: "dynamic", Notary: &policiesv1beta1.Notary{Certs: &policiesv1beta1.StringOrExpression{Expression: "value"}}}
	compiled, errs := CompileAttestors(field.NewPath("attestors"), []policiesv1beta1.Attestor{definition}, env)
	require.Empty(t, errs)
	for i := range 32 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			expected := fmt.Sprint(i)
			for range 10 {
				result, err := compiled[0].Evaluate(map[string]any{"value": expected})
				require.NoError(t, err)
				require.Equal(t, expected, result.Notary.Certs.Value)
				require.Empty(t, definition.Notary.Certs.Value)
				require.Empty(t, compiled[0].val.Notary.Certs.Value)
			}
		})
	}
}
