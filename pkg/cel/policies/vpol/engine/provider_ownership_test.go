package engine

import (
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type capturingCompiler struct {
	compiler.Compiler
	exceptions []string
}

func (c *capturingCompiler) Compile(policy policiesv1beta1.ValidatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*compiler.Policy, field.ErrorList) {
	for _, polex := range exceptions {
		c.exceptions = append(c.exceptions, polex.Name)
	}
	return c.Compiler.Compile(policy, exceptions)
}

// TestNewProvider_SkipsExceptionsOwnedByOtherControllers verifies that NewProvider compiles a policy
// with the exceptions Kyverno owns, and without those owned by another controller, such as Kyverno Authz.
func TestNewProvider_SkipsExceptionsOwnedByOtherControllers(t *testing.T) {
	exception := func(name string, mode policiesv1beta1.EvaluationMode) *policiesv1beta1.PolicyException {
		return &policiesv1beta1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: policiesv1beta1.PolicyExceptionSpec{
				EvaluationMode: mode,
				PolicyRefs:     []policiesv1beta1.PolicyRef{{Name: "p", Kind: policieskyvernoio.ValidatingPolicyKind}},
			},
		}
	}
	policy := &policiesv1beta1.ValidatingPolicy{
		TypeMeta:   metav1.TypeMeta{Kind: policieskyvernoio.ValidatingPolicyKind},
		ObjectMeta: metav1.ObjectMeta{Name: "p"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			Validations:      []admissionregistrationv1.Validation{{Expression: "true"}},
		},
	}
	c := &capturingCompiler{Compiler: compiler.NewCompiler()}
	exceptions := []*policiesv1beta1.PolicyException{
		exception("no-mode", ""),
		exception("kubernetes", "Kubernetes"),
		exception("envoy", "Envoy"),
		exception("http", "HTTP"),
	}

	_, err := NewProvider(c, []policiesv1beta1.ValidatingPolicyLike{policy}, exceptions)
	require.NoError(t, err)
	assert.Equal(t, []string{"no-mode", "kubernetes"}, c.exceptions)
}
