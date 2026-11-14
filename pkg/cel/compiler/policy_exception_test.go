package compiler_test

import (
	"fmt"
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/compiler"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestCompilePolicyExceptionMatchConditions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		expression string
		wantErrors bool
	}{
		{
			name:       "object",
			expression: "object.metadata.name == 'pod'",
		},
		{
			name:       "old object",
			expression: "oldObject.metadata.name == 'old-pod'",
		},
		{
			name:       "request object",
			expression: "request.object.metadata.name == 'pod'",
		},
		{
			name:       "namespace object",
			expression: "namespaceObject.metadata.name == 'default'",
		},
		{
			name:       "undeclared function",
			expression: "object.metadata.name.endsWithTYPO('pod')",
			wantErrors: true,
		},
		{
			name:       "undeclared variable",
			expression: "totally.unbound.variable == 'x'",
			wantErrors: true,
		},
		{
			name:       "non boolean result",
			expression: "object.metadata.name",
			wantErrors: true,
		},
		{
			name: "resource.Get",
			expression: `resource.Get(
				"v1",
				"namespaces",
				"",
				object.metadata.namespace
			).metadata.labels["team"] == "infra"`,
		},
		{
			name:       "resource invalid method",
			expression: `resource.DoesNotExist("v1", "namespaces", "", "default") == true`,
			wantErrors: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errs := compileConditions(t, []admissionregistrationv1.MatchCondition{{
				Name:       "test-condition",
				Expression: tt.expression,
			}})
			if tt.wantErrors {
				assert.NotEmpty(t, errs)
				assert.Equal(t, "spec.matchConditions[0].expression", errs[0].Field)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

func TestCompilePolicyExceptionMatchConditionsStructure(t *testing.T) {
	t.Parallel()
	t.Run("duplicate names", func(t *testing.T) {
		t.Parallel()
		errs := compileConditions(t, []admissionregistrationv1.MatchCondition{
			{Name: "duplicate", Expression: "true"},
			{Name: "duplicate", Expression: "true"},
		})
		assert.NotEmpty(t, errs)
		assert.Equal(t, "spec.matchConditions[1].name", errs[0].Field)
	})
	t.Run("empty expression", func(t *testing.T) {
		t.Parallel()
		errs := compileConditions(t, []admissionregistrationv1.MatchCondition{{
			Name: "empty",
		}})
		assert.NotEmpty(t, errs)
		assert.Equal(t, "spec.matchConditions[0].expression", errs[0].Field)
	})
	t.Run("invalid name", func(t *testing.T) {
		t.Parallel()
		errs := compileConditions(t, []admissionregistrationv1.MatchCondition{{
			Name:       "not a qualified name",
			Expression: "true",
		}})
		assert.NotEmpty(t, errs)
		assert.Equal(t, "spec.matchConditions[0].name", errs[0].Field)
	})
	t.Run("too many conditions", func(t *testing.T) {
		t.Parallel()
		conditions := make([]admissionregistrationv1.MatchCondition, 65)
		for i := range conditions {
			conditions[i] = admissionregistrationv1.MatchCondition{
				Name:       fmt.Sprintf("condition-%d", i),
				Expression: "true",
			}
		}
		errs := compileConditions(t, conditions)
		assert.NotEmpty(t, errs)
		assert.Equal(t, "spec.matchConditions", errs[0].Field)
	})
}

func compileConditions(t *testing.T, conditions []admissionregistrationv1.MatchCondition) field.ErrorList {
	t.Helper()
	env, err := vpolcompiler.NewExceptionEnv("")
	require.NoError(t, err)
	return compiler.CompilePolicyExceptionMatchConditions(env, conditions)
}
