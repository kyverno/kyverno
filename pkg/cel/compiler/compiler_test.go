package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestCompileMatchConditions(t *testing.T) {
	tests := []struct {
		name            string
		matchConditions []admissionregistrationv1.MatchCondition
		wantProgs       int
		wantErrs        field.ErrorList
	}{{
		name:            "nil",
		matchConditions: nil,
		wantProgs:       0,
		wantErrs:        nil,
	}, {
		name:            "empty",
		matchConditions: []admissionregistrationv1.MatchCondition{},
		wantProgs:       0,
		wantErrs:        nil,
	}, {
		name: "not bool",
		matchConditions: []admissionregistrationv1.MatchCondition{{
			Name:       "test",
			Expression: `"foo"`,
		}},
		wantProgs: 0,
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "[0].expression",
			BadValue: `"foo"`,
			Detail:   "output is expected to be of type bool",
		}},
	}, {
		name: "not valid",
		matchConditions: []admissionregistrationv1.MatchCondition{{
			Name:       "test",
			Expression: `foo()`,
		}},
		wantProgs: 0,
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "[0].expression",
			BadValue: `foo()`,
			Detail:   "ERROR: <input>:1:4: undeclared reference to 'foo' (in container '')\n | foo()\n | ...^",
		}},
	}, {
		name: "single",
		matchConditions: []admissionregistrationv1.MatchCondition{{
			Name:       "test",
			Expression: `"foo" == "bar"`,
		}},
		wantProgs: 1,
		wantErrs:  nil,
	}, {
		name: "multiple",
		matchConditions: []admissionregistrationv1.MatchCondition{{
			Name:       "test",
			Expression: `"foo" == "bar"`,
		}, {
			Name:       "test-2",
			Expression: `"foo" == "baz"`,
		}},
		wantProgs: 2,
		wantErrs:  nil,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := NewEnv()
			assert.NoError(t, err)
			got, errs := CompileMatchConditions(nil, tt.matchConditions, env)
			assert.Equal(t, tt.wantErrs, errs)
			assert.Equal(t, tt.wantProgs, len(got))
		})
	}
}

func TestCompileValidation(t *testing.T) {
	tests := []struct {
		name            string
		rule            admissionregistrationv1.Validation
		wantMessage     string
		wantMessageExpr bool
		wantProg        bool
		wantErrs        field.ErrorList
	}{{
		name: "empty",
		rule: admissionregistrationv1.Validation{},
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "expression",
			BadValue: "",
			Detail:   "ERROR: <input>:1:0: Syntax error: mismatched input '<EOF>' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}",
		}},
	}, {
		name: "valid",
		rule: admissionregistrationv1.Validation{
			Expression: "true",
		},
		wantProg: true,
		wantErrs: nil,
	}, {
		name: "invalid",
		rule: admissionregistrationv1.Validation{
			Expression: "foo()",
		},
		wantProg: false,
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "expression",
			BadValue: "foo()",
			Detail:   "ERROR: <input>:1:4: undeclared reference to 'foo' (in container '')\n | foo()\n | ...^",
		}},
	}, {
		name: "with message",
		rule: admissionregistrationv1.Validation{
			Message:    "test",
			Expression: "true",
		},
		wantMessage: "test",
		wantProg:    true,
		wantErrs:    nil,
	}, {
		name: "with message expression",
		rule: admissionregistrationv1.Validation{
			MessageExpression: `"test"`,
			Expression:        "true",
		},
		wantMessageExpr: true,
		wantProg:        true,
		wantErrs:        nil,
	}, {
		name: "with invalid message expression",
		rule: admissionregistrationv1.Validation{
			MessageExpression: "true",
			Expression:        "true",
		},
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "messageExpression",
			BadValue: "true",
			Detail:   "output is expected to be of type string",
		}},
	}, {
		name: "with invalid message expression",
		rule: admissionregistrationv1.Validation{
			MessageExpression: "foo()",
			Expression:        "true",
		},
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "messageExpression",
			BadValue: "foo()",
			Detail:   "ERROR: <input>:1:4: undeclared reference to 'foo' (in container '')\n | foo()\n | ...^",
		}},
	},
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := NewEnv()
			assert.NoError(t, err)
			got, errs := CompileValidation(nil, tt.rule, env)
			assert.Equal(t, tt.wantErrs, errs)
			assert.Equal(t, tt.wantMessage, got.Message)
			assert.Equal(t, tt.wantMessageExpr, got.MessageExpression != nil)
			assert.Equal(t, tt.wantProg, got.Program != nil)
		})
	}
}
