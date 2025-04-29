package compiler

import (
	"testing"

	"github.com/google/cel-go/cel"
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
			env, err := NewBaseEnv()
			assert.NoError(t, err)
			got, errs := CompileMatchConditions(nil, env, tt.matchConditions...)
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
		name: "invalid",
		rule: admissionregistrationv1.Validation{
			Expression: `"foo"`,
		},
		wantProg: false,
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "expression",
			BadValue: `"foo"`,
			Detail:   "output is expected to be of type bool",
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
			env, err := NewBaseEnv()
			assert.NoError(t, err)
			got, errs := CompileValidation(nil, env, tt.rule)
			assert.Equal(t, tt.wantErrs, errs)
			assert.Equal(t, tt.wantMessage, got.Message)
			assert.Equal(t, tt.wantMessageExpr, got.MessageExpression != nil)
			assert.Equal(t, tt.wantProg, got.Program != nil)
		})
	}
}

func TestCompileAuditAnnotation(t *testing.T) {
	tests := []struct {
		name            string
		auditAnnotation admissionregistrationv1.AuditAnnotation
		wantProg        bool
		wantErrs        field.ErrorList
	}{{
		name:            "empty",
		auditAnnotation: admissionregistrationv1.AuditAnnotation{},
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "valueExpression",
			BadValue: "",
			Detail:   "ERROR: <input>:1:0: Syntax error: mismatched input '<EOF>' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}",
		}},
	}, {
		name: "bad type",
		auditAnnotation: admissionregistrationv1.AuditAnnotation{
			Key:             "test",
			ValueExpression: "true",
		},
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "valueExpression",
			BadValue: "true",
			Detail:   "output is expected to be either of type string or null_type",
		}},
	}, {
		name: "doesn't compile",
		auditAnnotation: admissionregistrationv1.AuditAnnotation{
			Key:             "test",
			ValueExpression: "foo()",
		},
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "valueExpression",
			BadValue: "foo()",
			Detail:   "ERROR: <input>:1:4: undeclared reference to 'foo' (in container '')\n | foo()\n | ...^",
		}},
	}, {
		name: "valid",
		auditAnnotation: admissionregistrationv1.AuditAnnotation{
			Key:             "test",
			ValueExpression: `"test"`,
		},
		wantProg: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := NewBaseEnv()
			assert.NoError(t, err)
			got, errs := CompileAuditAnnotation(nil, env, tt.auditAnnotation)
			assert.Equal(t, tt.wantErrs, errs)
			assert.Equal(t, tt.wantProg, got != nil)
		})
	}
}

func TestCompileAuditAnnotations(t *testing.T) {
	tests := []struct {
		name             string
		auditAnnotations []admissionregistrationv1.AuditAnnotation
		wantProgs        int
		wantAllErrs      field.ErrorList
	}{{
		name:             "nil",
		auditAnnotations: nil,
		wantProgs:        0,
	}, {
		name:             "empty",
		auditAnnotations: []admissionregistrationv1.AuditAnnotation{},
		wantProgs:        0,
	}, {
		name: "single",
		auditAnnotations: []admissionregistrationv1.AuditAnnotation{{
			Key:             "foo",
			ValueExpression: `"foo"`,
		}},
		wantProgs: 1,
	}, {
		name: "multiple",
		auditAnnotations: []admissionregistrationv1.AuditAnnotation{{
			Key:             "foo",
			ValueExpression: `"foo"`,
		}, {
			Key:             "bar",
			ValueExpression: `"bar"`,
		}},
		wantProgs: 2,
	}, {
		name: "with error",
		auditAnnotations: []admissionregistrationv1.AuditAnnotation{{
			Key:             "foo",
			ValueExpression: `"foo"`,
		}, {
			Key:             "bar",
			ValueExpression: `bar()`,
		}},
		wantProgs: 1,
		wantAllErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "[1].valueExpression",
			BadValue: "bar()",
			Detail:   "ERROR: <input>:1:4: undeclared reference to 'bar' (in container '')\n | bar()\n | ...^",
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := NewBaseEnv()
			assert.NoError(t, err)
			gotProgs, gotAllErrs := CompileAuditAnnotations(nil, env, tt.auditAnnotations...)
			assert.Equal(t, tt.wantAllErrs, gotAllErrs)
			assert.Equal(t, tt.wantProgs, len(gotProgs))
		})
	}
}

func TestCompileVariables(t *testing.T) {
	tests := []struct {
		name        string
		variables   []admissionregistrationv1.Variable
		wantProgs   int
		wantAllErrs field.ErrorList
	}{{
		name:      "nil",
		variables: nil,
		wantProgs: 0,
	}, {
		name:      "empty",
		variables: []admissionregistrationv1.Variable{},
		wantProgs: 0,
	}, {
		name: "single",
		variables: []admissionregistrationv1.Variable{{
			Name:       "foo",
			Expression: `"foo"`,
		}},
		wantProgs: 1,
	}, {
		name: "multiple",
		variables: []admissionregistrationv1.Variable{{
			Name:       "foo",
			Expression: `"foo"`,
		}, {
			Name:       "bar",
			Expression: `"bar"`,
		}},
		wantProgs: 2,
	}, {
		name: "with dependency",
		variables: []admissionregistrationv1.Variable{{
			Name:       "foo",
			Expression: `"foo"`,
		}, {
			Name:       "foobar",
			Expression: `variables.foo + "bar"`,
		}},
		wantProgs: 2,
	}, {
		name: "with dependency error",
		variables: []admissionregistrationv1.Variable{{
			Name:       "foo",
			Expression: `"foo"`,
		}, {
			Name:       "errbar",
			Expression: `variables.err + "bar"`,
		}},
		wantProgs: 1,
		wantAllErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "[1].expression",
			BadValue: `variables.err + "bar"`,
			Detail:   "ERROR: <input>:1:10: undefined field 'err'\n | variables.err + \"bar\"\n | .........^",
		}},
	}, {
		name: "with error",
		variables: []admissionregistrationv1.Variable{{
			Name:       "foo",
			Expression: `"foo"`,
		}, {
			Name:       "bar",
			Expression: `bar()`,
		}},
		wantProgs: 1,
		wantAllErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "[1].expression",
			BadValue: "bar()",
			Detail:   "ERROR: <input>:1:4: undeclared reference to 'bar' (in container '')\n | bar()\n | ...^",
		}},
	},
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := NewBaseEnv()
			assert.NoError(t, err)
			provider := NewVariablesProvider(env.CELTypeProvider())
			env, err = env.Extend(
				cel.Variable(VariablesKey, VariablesType),
				cel.CustomTypeProvider(provider),
			)
			assert.NoError(t, err)
			gotProgs, gotAllErrs := CompileVariables(nil, env, provider, tt.variables...)
			assert.Equal(t, tt.wantAllErrs, gotAllErrs)
			assert.Equal(t, tt.wantProgs, len(gotProgs))
		})
	}
}
