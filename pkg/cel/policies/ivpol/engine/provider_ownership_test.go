package engine

import (
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	iveval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type capturingCompiler struct {
	iveval.Compiler
	exceptions [][]string
}

func (c *capturingCompiler) Compile(policy policiesv1beta1.ImageValidatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (iveval.CompiledPolicy, field.ErrorList) {
	names := []string{}
	for _, polex := range exceptions {
		names = append(names, polex.Name)
	}
	c.exceptions = append(c.exceptions, names)
	return c.Compiler.Compile(policy, exceptions)
}

// TestNewProvider_SkipsExceptionsOwnedByOtherControllers verifies that NewProvider compiles a policy
// and its autogen variants with the exceptions Kyverno owns, and without those owned by another
// controller, such as Kyverno Authz.
func TestNewProvider_SkipsExceptionsOwnedByOtherControllers(t *testing.T) {
	exception := func(name string, mode policiesv1beta1.EvaluationMode) *policiesv1beta1.PolicyException {
		return &policiesv1beta1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: policiesv1beta1.PolicyExceptionSpec{
				EvaluationMode: mode,
				PolicyRefs:     []policiesv1beta1.PolicyRef{{Name: "alpha", Kind: policieskyvernoio.ImageValidatingPolicyKind}},
			},
		}
	}
	policy := autogennablePolicy("alpha")
	policy.TypeMeta = metav1.TypeMeta{Kind: policieskyvernoio.ImageValidatingPolicyKind}
	c := &capturingCompiler{Compiler: iveval.NewCompiler(nil)}
	exceptions := []*policiesv1beta1.PolicyException{
		exception("no-mode", ""),
		exception("kubernetes", "Kubernetes"),
		exception("envoy", "Envoy"),
		exception("http", "HTTP"),
	}

	_, err := NewProvider(c, []policiesv1beta1.ImageValidatingPolicyLike{policy}, exceptions)
	require.NoError(t, err)
	require.NotEmpty(t, c.exceptions)
	for _, names := range c.exceptions {
		assert.Equal(t, []string{"no-mode", "kubernetes"}, names)
	}
}
