package compiler

import (
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func compilePolicy(vars []admissionregistrationv1.Variable, rules ...policiesv1alpha1.AuthorizingRule) (*Policy, []string) {
	p := &policiesv1alpha1.AuthorizingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "p"},
		Spec:       policiesv1alpha1.AuthorizingPolicySpec{Variables: vars, Rules: rules},
	}
	out, errs := NewCompiler().Compile(p)
	var fields []string
	for _, e := range errs {
		fields = append(fields, e.Field)
	}
	return out, fields
}

func allowRule(name, expr string) policiesv1alpha1.AuthorizingRule {
	return policiesv1alpha1.AuthorizingRule{Name: name, Effect: policiesv1alpha1.AuthorizingRuleEffectAllow, Expression: expr}
}

func TestCompile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		vars       []admissionregistrationv1.Variable
		rules      []policiesv1alpha1.AuthorizingRule
		wantFields []string
	}{{
		name:  "request, object, oldObject, and variables",
		vars:  []admissionregistrationv1.Variable{{Name: "isDev", Expression: "'dev' in request.groups"}},
		rules: []policiesv1alpha1.AuthorizingRule{allowRule("a", "variables.isDev && object.metadata.name != oldObject.metadata.name")},
	}, {
		name:  "comprehensions over object",
		rules: []policiesv1alpha1.AuthorizingRule{allowRule("a", "object.spec.containers.all(c, c.image.startsWith('registry.example.com/'))")},
	}, {
		name:  "cel.bind is available",
		rules: []policiesv1alpha1.AuthorizingRule{allowRule("a", "cel.bind(u, request.user, u == 'alice')")},
	}, {
		name:       "syntax error",
		rules:      []policiesv1alpha1.AuthorizingRule{allowRule("a", "this is not cel")},
		wantFields: []string{"spec.rules[0].expression"},
	}, {
		name:       "non-boolean rule",
		rules:      []policiesv1alpha1.AuthorizingRule{allowRule("a", "request.user")},
		wantFields: []string{"spec.rules[0].expression"},
	}, {
		name:       "undeclared identifier",
		rules:      []policiesv1alpha1.AuthorizingRule{allowRule("a", "params.allowed")},
		wantFields: []string{"spec.rules[0].expression"},
	}, {
		name:       "undeclared variable",
		rules:      []policiesv1alpha1.AuthorizingRule{allowRule("a", "variables.missing == true")},
		wantFields: []string{"spec.rules[0].expression"},
	}, {
		name:       "variable reading object",
		vars:       []admissionregistrationv1.Variable{{Name: "n", Expression: "object.metadata.name"}},
		rules:      []policiesv1alpha1.AuthorizingRule{allowRule("a", "true")},
		wantFields: []string{"spec.variables[0].expression"},
	}, {
		name:       "variable reading oldObject",
		vars:       []admissionregistrationv1.Variable{{Name: "n", Expression: "oldObject.metadata.name"}},
		rules:      []policiesv1alpha1.AuthorizingRule{allowRule("a", "true")},
		wantFields: []string{"spec.variables[0].expression"},
	}, {
		// Residual conditions must be pure functions of the decision snapshot,
		// so Kyverno libraries that perform I/O are not registered.
		name:       "http library is not available",
		rules:      []policiesv1alpha1.AuthorizingRule{allowRule("a", "http.Get('https://example.com') != null")},
		wantFields: []string{"spec.rules[0].expression"},
	}, {
		name:       "resource library is not available",
		rules:      []policiesv1alpha1.AuthorizingRule{allowRule("a", "resource.Get('v1', 'configmaps', 'ns', 'cm') != null")},
		wantFields: []string{"spec.rules[0].expression"},
	}, {
		name:       "globalContext library is not available",
		rules:      []policiesv1alpha1.AuthorizingRule{allowRule("a", "globalContext.Get('x', '') != null")},
		wantFields: []string{"spec.rules[0].expression"},
	}, {
		name: "every invalid rule is reported",
		rules: []policiesv1alpha1.AuthorizingRule{
			allowRule("a", "this is not cel"), allowRule("b", "true"), allowRule("c", "1"),
		},
		wantFields: []string{"spec.rules[0].expression", "spec.rules[2].expression"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, fields := compilePolicy(tt.vars, tt.rules...)
			require.Equal(t, tt.wantFields, fields)
			if len(tt.wantFields) > 0 {
				require.Nil(t, out)
				return
			}
			require.NotNil(t, out)
			require.Len(t, out.Rules, len(tt.rules))
			require.Len(t, out.Variables, len(tt.vars))
		})
	}
}

func TestCompilePreservesRuleOrderAndMetadata(t *testing.T) {
	t.Parallel()
	rules := []policiesv1alpha1.AuthorizingRule{
		{Name: "z", Effect: policiesv1alpha1.AuthorizingRuleEffectDeny, Expression: "false", Description: "zed"},
		{Name: "a", Effect: policiesv1alpha1.AuthorizingRuleEffectNoOpinion, Expression: "true"},
	}
	out, fields := compilePolicy(nil, rules...)
	require.Empty(t, fields)
	require.Len(t, out.Rules, 2)
	for i, r := range rules {
		require.Equal(t, r.Name, out.Rules[i].Name)
		require.Equal(t, r.Effect, out.Rules[i].Effect)
		require.Equal(t, r.Description, out.Rules[i].Description)
		require.Equal(t, r.Expression, out.Rules[i].Expression)
		require.NotNil(t, out.Rules[i].AST)
		require.NotNil(t, out.Rules[i].Program)
	}
}

func TestConditionsEnvDeclaresOnlyObjects(t *testing.T) {
	t.Parallel()
	env, err := NewConditionsEnv()
	require.NoError(t, err)
	for _, ok := range []string{"object.metadata.name == 'x'", "oldObject == null", "cel.bind(u, dyn('alice'), object.metadata.labels.owner == u)"} {
		_, issues := env.Compile(ok)
		require.NoError(t, issues.Err(), ok)
	}
	for _, bad := range []string{"request.user == 'alice'", "variables.x", "http.Get('https://example.com') != null"} {
		_, issues := env.Compile(bad)
		require.Error(t, issues.Err(), bad)
	}
}
