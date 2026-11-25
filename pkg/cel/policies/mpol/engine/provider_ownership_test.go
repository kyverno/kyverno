package engine

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type capturingCompiler struct {
	compiler.Compiler
	calls      int
	exceptions []string
}

func (c *capturingCompiler) Compile(policy policiesv1beta1.MutatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*compiler.Policy, field.ErrorList) {
	c.calls++
	for _, polex := range exceptions {
		c.exceptions = append(c.exceptions, polex.Name)
	}
	return c.Compiler.Compile(policy, exceptions)
}

// TestNewProvider_SkipsExceptionsOwnedByOtherControllers verifies that NewProvider compiles a policy
// with the exceptions Kyverno owns, and without those owned by another controller, such as Kyverno Authz.
func TestNewProvider_SkipsExceptionsOwnedByOtherControllers(t *testing.T) {
	exception := func(mode policiesv1beta1.EvaluationMode) *policiesv1beta1.PolicyException {
		return &policiesv1beta1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{Name: "polex-" + string(mode)},
			Spec: policiesv1beta1.PolicyExceptionSpec{
				EvaluationMode: mode,
				PolicyRefs:     []policiesv1beta1.PolicyRef{{Name: "p", Kind: "MutatingPolicy"}},
			},
		}
	}
	tests := []struct {
		name       string
		exceptions []*policiesv1beta1.PolicyException
		want       []string
	}{
		{name: "owned only", exceptions: []*policiesv1beta1.PolicyException{exception(""), exception("Kubernetes")}, want: []string{"polex-", "polex-Kubernetes"}},
		{name: "JSON is kept", exceptions: []*policiesv1beta1.PolicyException{exception("JSON")}, want: []string{"polex-JSON"}},
		{name: "foreign only", exceptions: []*policiesv1beta1.PolicyException{exception("Envoy"), exception("HTTP")}},
		{name: "mixed", exceptions: []*policiesv1beta1.PolicyException{exception(""), exception("Envoy"), exception("JSON"), exception("HTTP")}, want: []string{"polex-", "polex-JSON"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &policiesv1beta1.MutatingPolicy{
				TypeMeta:   metav1.TypeMeta{Kind: "MutatingPolicy"},
				ObjectMeta: metav1.ObjectMeta{Name: "p"},
				Spec: policiesv1beta1.MutatingPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{},
				},
			}
			c := &capturingCompiler{Compiler: compiler.NewCompiler()}
			_, err := NewProvider(c, []policiesv1beta1.MutatingPolicyLike{policy}, tt.exceptions, libs.NewFakeContextProvider())
			require.NoError(t, err)
			require.Equal(t, 1, c.calls, "the policy must be compiled")
			assert.ElementsMatch(t, tt.want, c.exceptions)
		})
	}
}
