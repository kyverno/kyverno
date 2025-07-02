package compiler

import (
	"testing"

	"github.com/google/cel-go/cel"
	policiesv1alpgha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/libs/generator"
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
	}}
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
	}}
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

func TestCompileMatchImageReference(t *testing.T) {
	tests := []struct {
		name      string
		match     policiesv1alpgha1.MatchImageReference
		wantMatch bool
		wantErrs  field.ErrorList
	}{{
		name: "glob",
		match: policiesv1alpgha1.MatchImageReference{
			Glob: "ghcr.io/*",
		},
		wantMatch: true,
	}, {
		name: "cel",
		match: policiesv1alpgha1.MatchImageReference{
			Expression: "true",
		},
		wantMatch: true,
	}, {
		name: "cel error",
		match: policiesv1alpgha1.MatchImageReference{
			Expression: "bar()",
		},
		wantMatch: false,
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "test.expression",
			BadValue: "bar()",
			Detail:   "ERROR: <input>:1:4: undeclared reference to 'bar' (in container '')\n | bar()\n | ...^",
		}},
	}, {
		name: "cel not bool",
		match: policiesv1alpgha1.MatchImageReference{
			Expression: `"bar"`,
		},
		wantMatch: false,
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "test.expression",
			BadValue: `"bar"`,
			Detail:   "output is expected to be of type bool",
		}},
	}, {
		name:      "unknown",
		match:     policiesv1alpgha1.MatchImageReference{},
		wantMatch: false,
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "test",
			BadValue: policiesv1alpgha1.MatchImageReference{},
			Detail:   "either glob or expression must be set",
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := NewMatchImageEnv()
			assert.NoError(t, err)
			got, gotErrs := CompileMatchImageReference(field.NewPath("test"), env, tt.match)
			assert.Equal(t, tt.wantErrs, gotErrs)
			assert.Equal(t, tt.wantMatch, got != nil)
		})
	}
}

func TestCompileMatchImageReferences(t *testing.T) {
	tests := []struct {
		name        string
		matches     []policiesv1alpgha1.MatchImageReference
		wantResults int
		wantAllErrs field.ErrorList
	}{{
		name:        "nil",
		matches:     nil,
		wantResults: 0,
	}, {
		name:        "empty",
		matches:     []policiesv1alpgha1.MatchImageReference{},
		wantResults: 0,
	}, {
		name: "single",
		matches: []policiesv1alpgha1.MatchImageReference{{
			Expression: `true`,
		}},
		wantResults: 1,
	}, {
		name: "multiple",
		matches: []policiesv1alpgha1.MatchImageReference{{
			Expression: `true`,
		}, {
			Expression: `false`,
		}},
		wantResults: 2,
	}, {
		name: "with error",
		matches: []policiesv1alpgha1.MatchImageReference{{
			Expression: `true`,
		}, {
			Expression: `bar()`,
		}},
		wantResults: 1,
		wantAllErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "[1].expression",
			BadValue: "bar()",
			Detail:   "ERROR: <input>:1:4: undeclared reference to 'bar' (in container '')\n | bar()\n | ...^",
		}},
	}, {
		name: "not bool",
		matches: []policiesv1alpgha1.MatchImageReference{{
			Expression: `true`,
		}, {
			Expression: `"bar"`,
		}},
		wantResults: 1,
		wantAllErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "[1].expression",
			BadValue: `"bar"`,
			Detail:   "output is expected to be of type bool",
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := NewMatchImageEnv()
			assert.NoError(t, err)
			gotResults, gotAllErrs := CompileMatchImageReferences(nil, env, tt.matches...)
			assert.Equal(t, tt.wantAllErrs, gotAllErrs)
			assert.Equal(t, tt.wantResults, len(gotResults))
		})
	}
}

func TestCompileGenerations(t *testing.T) {
	tests := []struct {
		name        string
		generations []policiesv1alpgha1.Generation
		wantProgs   int
		wantErrs    field.ErrorList
	}{{
		name:        "empty",
		generations: []policiesv1alpgha1.Generation{},
		wantProgs:   0,
	}, {
		name: "valid",
		generations: []policiesv1alpgha1.Generation{{
			Expression: `
generator.Apply(
	"default",
	[
		{
			"apiVersion": dyn("apps/v1"),
			"kind":       dyn("Deployment"),
			"metadata": dyn({
				"name":      "name",
				"namespace": "namespace",
			}),
		},
	]
)`,
		}},
		wantProgs: 1,
	}, {
		name: "multiple",
		generations: []policiesv1alpgha1.Generation{{
			Expression: `
generator.Apply(
	"default",
	[
		{
			"apiVersion": dyn("apps/v1"),
			"kind":       dyn("Deployment"),
			"metadata": dyn({
				"name":      "name",
				"namespace": "namespace",
			}),
		},
	]
)`,
		}, {
			Expression: `
generator.Apply(
	"default",
	[
		{
			"apiVersion": dyn("v1"),
			"kind":       dyn("ConfigMap"),
			"metadata": dyn({
				"name":      "name",
				"namespace": "namespace",
			}),
		},
	]
)`,
		}},
		wantProgs: 2,
	}, {
		name: "invalid",
		generations: []policiesv1alpgha1.Generation{{
			Expression: `
generator.ApplyAll(
	"default",
	[
		{
			"apiVersion": dyn("apps/v1"),
			"kind":       dyn("Deployment"),
			"metadata": dyn({
				"name":      "name",
				"namespace": "namespace",
			}),
		},
	]
)`,
		}},
		wantProgs: 0,
		wantErrs: field.ErrorList{{
			Type:  field.ErrorTypeInvalid,
			Field: "[0].expression",
			BadValue: `
generator.ApplyAll(
	"default",
	[
		{
			"apiVersion": dyn("apps/v1"),
			"kind":       dyn("Deployment"),
			"metadata": dyn({
				"name":      "name",
				"namespace": "namespace",
			}),
		},
	]
)`,
			Detail: "ERROR: <input>:2:19: undeclared reference to 'ApplyAll' (in container '')\n | generator.ApplyAll(\n | ..................^",
		}},
	}, {
		name: "multiple invalid",
		generations: []policiesv1alpgha1.Generation{{
			Expression: `
generator.Apply(
	"default",
	[
		{
			"apiVersion": dyn("apps/v1"),
			"kind":       dyn("Deployment"),
			"metadata": dyn({
				"name":      "name",
				"namespace": "namespace",
			}),
		},
	]
)`,
		}, {
			Expression: `
generator.Apply(
	[
		{
			"apiVersion": dyn("v1"),
			"kind":       dyn("ConfigMap"),
			"metadata": dyn({
				"name":      "name",
				"namespace": "namespace",
			}),
		},
	]
)`,
		}},
		wantProgs: 1,
		wantErrs: field.ErrorList{{
			Type:  field.ErrorTypeInvalid,
			Field: "[1].expression",
			BadValue: `
generator.Apply(
	[
		{
			"apiVersion": dyn("v1"),
			"kind":       dyn("ConfigMap"),
			"metadata": dyn({
				"name":      "name",
				"namespace": "namespace",
			}),
		},
	]
)`,
			Detail: "ERROR: <input>:2:16: found no matching overload for 'Apply' applied to 'generator.Context.(list(map(string, dyn)))'\n | generator.Apply(\n | ...............^",
		}},
	}, {
		name: "bad type",
		generations: []policiesv1alpgha1.Generation{{
			Expression: `"foo"`,
		}},
		wantProgs: 0,
		wantErrs: field.ErrorList{{
			Type:     field.ErrorTypeInvalid,
			Field:    "[0].expression",
			BadValue: `"foo"`,
			Detail:   "output is expected to be of type bool",
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, err := NewBaseEnv()
			assert.NoError(t, err)
			env, err := base.Extend(
				cel.Variable(GeneratorKey, generator.ContextType),
				generator.Lib(),
			)
			assert.NoError(t, err)
			gotProgs, gotErrs := CompileGenerations(nil, env, tt.generations...)
			assert.Equal(t, tt.wantErrs, gotErrs)
			assert.Equal(t, tt.wantProgs, len(gotProgs))
		})
	}
}
