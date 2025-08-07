package compiler

import (
	"testing"

	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func TestCompile(t *testing.T) {
	t.Run("should_compile_successfully_when_valid_match_condition_provided", func(t *testing.T) {
		pol := &v1alpha1.GeneratingPolicy{
			Spec: v1alpha1.GeneratingPolicySpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{
						Name:       "is-namespace",
						Expression: "object.metadata.namespace == 'isolated'",
					},
				},
			},
		}
		comp := NewCompiler()

		res, errs := comp.Compile(pol, nil)
		assert.NotNil(t, res)
		assert.Nil(t, errs)
	})

	t.Run("should_fail_when_match_condition_is_empty", func(t *testing.T) {
		pol := &v1alpha1.GeneratingPolicy{
			Spec: v1alpha1.GeneratingPolicySpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{
						Name:       "",
						Expression: "",
					},
				},
			},
		}
		comp := NewCompiler()

		res, errs := comp.Compile(pol, nil)
		assert.Nil(t, res)
		assert.NotNil(t, errs)
	})

	t.Run("should_fail_when_variable_expression_is_invalid", func(t *testing.T) {
		pol := &v1alpha1.GeneratingPolicy{
			Spec: v1alpha1.GeneratingPolicySpec{
				Variables: []admissionregistrationv1.Variable{
					{
						Name:       "invalid-name",
						Expression: "invalid-expression",
					},
				},
			},
		}
		comp := NewCompiler()
		res, errs := comp.Compile(pol, nil)
		assert.Nil(t, res)
		assert.NotNil(t, errs)
	})

	t.Run("should_fail_when_generation_expression_is_invalid", func(t *testing.T) {
		pol := &v1alpha1.GeneratingPolicy{
			Spec: v1alpha1.GeneratingPolicySpec{
				Generation: []v1alpha1.Generation{
					{
						Expression: "invalid-generation",
					},
				},
			},
		}
		comp := NewCompiler()
		res, errs := comp.Compile(pol, nil)
		assert.Nil(t, res)
		assert.NotNil(t, errs)
	})

	t.Run("should_fail_when_match_condition_in_policy_exception_is_invalid", func(t *testing.T) {
		pol := &v1alpha1.GeneratingPolicy{
			Spec: v1alpha1.GeneratingPolicySpec{},
		}
		polexs := []*v1alpha1.PolicyException{
			{
				Spec: v1alpha1.PolicyExceptionSpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "valid-exec",
							Expression: "object.metadata.namespace ==",
						},
					},
				},
			},
		}
		comp := NewCompiler()
		res, errs := comp.Compile(pol, polexs)
		assert.Nil(t, res)
		assert.NotNil(t, errs)
	})

	t.Run("should_compile_successfully_with_valid_policy_exception_conditions", func(t *testing.T) {
		pol := &v1alpha1.GeneratingPolicy{
			Spec: v1alpha1.GeneratingPolicySpec{},
		}
		polexs := []*v1alpha1.PolicyException{
			{
				Spec: v1alpha1.PolicyExceptionSpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "valid-exec",
							Expression: "object.metadata.namespace == 'default'",
						},
					},
				},
			},
		}
		comp := NewCompiler()
		res, errs := comp.Compile(pol, polexs)
		assert.NotNil(t, res)
		assert.Nil(t, errs)
	})
}
