package engine

import (
	"context"
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func policy(name string, vars []admissionregistrationv1.Variable, rules ...policiesv1alpha1.AuthorizingRule) *policiesv1alpha1.AuthorizingPolicy {
	return &policiesv1alpha1.AuthorizingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1alpha1.AuthorizingPolicySpec{
			MatchSubjects: []policiesv1alpha1.SubjectSelectorTerm{{
				MatchExpressions: []policiesv1alpha1.SubjectSelectorRequirement{{Key: "groups", Operator: "In", Values: []string{"dev"}}},
			}},
			MatchResources: policiesv1alpha1.AuthorizingMatchResources{
				ResourceRules: []policiesv1alpha1.AuthorizingResourceRule{{
					APIGroups: []string{""}, APIVersions: []string{"*"}, Resources: []string{"configmaps"}, Verbs: []string{"*"},
				}},
			},
			Variables: vars,
			Rules:     rules,
		},
	}
}

func rule(name string, effect policiesv1alpha1.AuthorizingRuleEffect, expr string) policiesv1alpha1.AuthorizingRule {
	return policiesv1alpha1.AuthorizingRule{Name: name, Effect: effect, Expression: expr}
}

func newEngine(t *testing.T, pols ...*policiesv1alpha1.AuthorizingPolicy) Engine {
	t.Helper()
	c := apolcompiler.NewCompiler()
	var compiled []CompiledPolicy
	for _, p := range pols {
		compiled = append(compiled, NewCompiledPolicy(c, p))
	}
	e, err := NewEngine(ProviderFunc(func(context.Context) ([]CompiledPolicy, error) { return compiled, nil }))
	require.NoError(t, err)
	return e
}

func sar(verb string, groups ...string) *authorizationv1.SubjectAccessReviewSpec {
	return &authorizationv1.SubjectAccessReviewSpec{
		User:   "alice",
		Groups: groups,
		ResourceAttributes: &authorizationv1.ResourceAttributes{
			Namespace: "team-a", Verb: verb, Version: "v1", Resource: "configmaps", Name: "cm",
		},
	}
}

const (
	allowE = policiesv1alpha1.AuthorizingRuleEffectAllow
	denyE  = policiesv1alpha1.AuthorizingRuleEffectDeny
	noOpE  = policiesv1alpha1.AuthorizingRuleEffectNoOpinion
)

func TestAuthorize(t *testing.T) {
	t.Parallel()
	labelled := "has(object.metadata.labels) && 'owner' in object.metadata.labels && object.metadata.labels.owner == request.user"
	tests := []struct {
		name     string
		pols     []*policiesv1alpha1.AuthorizingPolicy
		spec     *authorizationv1.SubjectAccessReviewSpec
		aware    bool
		wantType authorizationv1.ConditionsAwareDecisionType
		check    func(t *testing.T, d authorizationv1.ConditionsAwareDecision)
	}{{
		name:     "no policy applies",
		pols:     []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil, rule("a", allowE, "true"))},
		spec:     sar("get", "other"),
		wantType: authorizationv1.ConditionsAwareDecisionTypeNoOpinion,
	}, {
		name:     "definite allow",
		pols:     []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil, rule("a", allowE, "request.resourceAttributes.verb == 'get'"))},
		spec:     sar("get", "dev"),
		wantType: authorizationv1.ConditionsAwareDecisionTypeAllow,
	}, {
		name: "deny beats allow across policies",
		pols: []*policiesv1alpha1.AuthorizingPolicy{
			policy("a", nil, rule("a", allowE, "true")),
			policy("b", nil, rule("d", denyE, "request.user == 'alice'")),
		},
		spec:     sar("get", "dev"),
		wantType: authorizationv1.ConditionsAwareDecisionTypeDeny,
	}, {
		name: "noopinion beats allow",
		pols: []*policiesv1alpha1.AuthorizingPolicy{
			policy("a", nil, rule("a", allowE, "true"), rule("n", noOpE, "true")),
		},
		spec:     sar("get", "dev"),
		wantType: authorizationv1.ConditionsAwareDecisionTypeNoOpinion,
	}, {
		name:     "variables are known",
		pols:     []*policiesv1alpha1.AuthorizingPolicy{policy("p", []admissionregistrationv1.Variable{{Name: "isDev", Expression: "'dev' in request.groups"}}, rule("a", allowE, "variables.isDev"))},
		spec:     sar("get", "dev"),
		wantType: authorizationv1.ConditionsAwareDecisionTypeAllow,
	}, {
		name:     "residual allow for aware caller",
		pols:     []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil, rule("owner", allowE, labelled))},
		spec:     sar("update", "dev"),
		aware:    true,
		wantType: authorizationv1.ConditionsAwareDecisionTypeConditionsMap,
		check: func(t *testing.T, d authorizationv1.ConditionsAwareDecision) {
			require.Len(t, d.ConditionsMap.AllowConditions, 1)
			c := d.ConditionsMap.AllowConditions[0]
			require.Equal(t, "p/owner", c.ID)
			require.Equal(t, ConditionType, c.Type)
			require.NotContains(t, c.Condition, "request")
			require.Contains(t, c.Condition, `"alice"`)
			t.Log(c.Condition)
		},
	}, {
		name:     "residual allow for unaware caller fails closed to NoOpinion",
		pols:     []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil, rule("owner", allowE, labelled))},
		spec:     sar("update", "dev"),
		wantType: authorizationv1.ConditionsAwareDecisionTypeNoOpinion,
	}, {
		name:     "residual deny for unaware caller fails closed to Deny",
		pols:     []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil, rule("d", denyE, "object.metadata.name == 'x'"))},
		spec:     sar("update", "dev"),
		wantType: authorizationv1.ConditionsAwareDecisionTypeDeny,
	}, {
		name:     "residual deny on read verb fails closed",
		pols:     []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil, rule("d", denyE, "object.metadata.name == 'x'"))},
		spec:     sar("get", "dev"),
		aware:    true,
		wantType: authorizationv1.ConditionsAwareDecisionTypeDeny,
	}, {
		name: "residual deny with definite allow gives map",
		pols: []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil,
			rule("d", denyE, "object.metadata.name == 'x'"),
			rule("a", allowE, "true"),
		)},
		spec:     sar("create", "dev"),
		aware:    true,
		wantType: authorizationv1.ConditionsAwareDecisionTypeConditionsMap,
		check: func(t *testing.T, d authorizationv1.ConditionsAwareDecision) {
			require.Len(t, d.ConditionsMap.DenyConditions, 1)
			require.Len(t, d.ConditionsMap.AllowConditions, 1)
			require.Equal(t, definiteAllowID, d.ConditionsMap.AllowConditions[0].ID)
			require.Equal(t, "true", d.ConditionsMap.AllowConditions[0].Condition)
		},
	}, {
		name: "definite allow and only allow residuals simplifies to Allow",
		pols: []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil,
			rule("r", allowE, "object.metadata.name == 'x'"),
			rule("a", allowE, "true"),
		)},
		spec:     sar("create", "dev"),
		aware:    true,
		wantType: authorizationv1.ConditionsAwareDecisionTypeAllow,
	}, {
		name:     "deny rule error applies",
		pols:     []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil, rule("d", denyE, "request.extra['missing'][0] == 'x'"))},
		spec:     sar("get", "dev"),
		wantType: authorizationv1.ConditionsAwareDecisionTypeDeny,
		check: func(t *testing.T, d authorizationv1.ConditionsAwareDecision) {
			require.NotEmpty(t, d.Deny.EvaluationError)
		},
	}, {
		name:     "allow rule error is dropped",
		pols:     []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil, rule("a", allowE, "request.extra['missing'][0] == 'x'"))},
		spec:     sar("get", "dev"),
		wantType: authorizationv1.ConditionsAwareDecisionTypeNoOpinion,
		check: func(t *testing.T, d authorizationv1.ConditionsAwareDecision) {
			require.NotEmpty(t, d.NoOpinion.EvaluationError)
		},
	}, {
		name: "compile error makes deny rules apply",
		pols: []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil,
			rule("d", denyE, "this is not cel"),
			rule("a", allowE, "true"),
		)},
		spec:     sar("get", "dev"),
		wantType: authorizationv1.ConditionsAwareDecisionTypeDeny,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newEngine(t, tt.pols...)
			d := e.Authorize(context.Background(), tt.spec, tt.aware)
			require.Equal(t, tt.wantType, d.Type, "%+v", d)
			if tt.check != nil {
				tt.check(t, d)
			}
		})
	}
}

func TestAuthorizeIsOrderIndependent(t *testing.T) {
	t.Parallel()
	a := policy("a", nil, rule("r1", allowE, "object.metadata.name == 'x'"))
	b := policy("b", nil, rule("r2", denyE, "object.metadata.name == 'y'"))
	d1 := newEngine(t, a, b).Authorize(context.Background(), sar("create", "dev"), true)
	d2 := newEngine(t, b, a).Authorize(context.Background(), sar("create", "dev"), true)
	require.Equal(t, d1, d2)
}

func TestVariablesCannotReferenceObject(t *testing.T) {
	t.Parallel()
	_, errs := apolcompiler.NewCompiler().Compile(policy("p", []admissionregistrationv1.Variable{{Name: "n", Expression: "object.metadata.name"}}, rule("a", allowE, "true")))
	require.NotEmpty(t, errs)
}

func TestEvaluateConditions(t *testing.T) {
	t.Parallel()
	labelled := "has(object.metadata.labels) && 'owner' in object.metadata.labels && object.metadata.labels.owner == request.user"
	e := newEngine(t, policy("p", nil,
		rule("owner", allowE, labelled),
		rule("protected", denyE, "has(object.metadata.labels) && 'protected' in object.metadata.labels"),
	))
	d := e.Authorize(context.Background(), sar("create", "dev"), true)
	require.Equal(t, authorizationv1.ConditionsAwareDecisionTypeConditionsMap, d.Type)
	obj := func(labels string) *admissionv1.AdmissionRequest {
		return &admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm","labels":` + labels + `}}`)}}
	}
	tests := []struct {
		name   string
		req    *admissionv1.AdmissionRequest
		wanted authorizationv1.ConditionsAwareDecisionType
	}{
		{"owner allows", obj(`{"owner":"alice"}`), authorizationv1.ConditionsAwareDecisionTypeAllow},
		{"other owner", obj(`{"owner":"bob"}`), authorizationv1.ConditionsAwareDecisionTypeNoOpinion},
		{"protected denies", obj(`{"owner":"alice","protected":"true"}`), authorizationv1.ConditionsAwareDecisionTypeDeny},
		{"undecodable object fails closed", &admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: []byte(`{`)}}, authorizationv1.ConditionsAwareDecisionTypeDeny},
		{"missing admission request fails closed", nil, authorizationv1.ConditionsAwareDecisionTypeDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := e.EvaluateConditions(context.Background(), &d, tt.req)
			require.Equal(t, tt.wanted, got.Type, "%+v", got)
		})
	}
}

func TestEvaluateConditionsRejectsMalformed(t *testing.T) {
	t.Parallel()
	e := newEngine(t)
	req := &admissionv1.AdmissionRequest{}
	cond := func(id, typ, expr string) authorizationv1.Condition {
		return authorizationv1.Condition{ID: id, Type: typ, Condition: expr}
	}
	tests := []struct {
		name     string
		decision *authorizationv1.ConditionsAwareDecision
		wanted   authorizationv1.ConditionsAwareDecisionType
	}{
		{"nil decision", nil, authorizationv1.ConditionsAwareDecisionTypeDeny},
		{"unknown type", &authorizationv1.ConditionsAwareDecision{Type: "Bogus"}, authorizationv1.ConditionsAwareDecisionTypeDeny},
		{"unconditional", &authorizationv1.ConditionsAwareDecision{Type: authorizationv1.ConditionsAwareDecisionTypeAllow, Allow: &authorizationv1.UnconditionalDecision{}}, authorizationv1.ConditionsAwareDecisionTypeNoOpinion},
		{"union", &authorizationv1.ConditionsAwareDecision{Type: authorizationv1.ConditionsAwareDecisionTypeUnion, Union: []authorizationv1.NamedConditionsAwareDecision{{
			AuthorizerName: "x", Decision: authorizationv1.ConditionsAwareDecision{Type: authorizationv1.ConditionsAwareDecisionTypeConditionsMap, ConditionsMap: &authorizationv1.ConditionsMap{DenyConditions: []authorizationv1.Condition{cond("p/d", ConditionType, "true")}}},
		}}}, authorizationv1.ConditionsAwareDecisionTypeDeny},
		{"duplicate ids", &authorizationv1.ConditionsAwareDecision{Type: authorizationv1.ConditionsAwareDecisionTypeConditionsMap, ConditionsMap: &authorizationv1.ConditionsMap{
			DenyConditions:  []authorizationv1.Condition{cond("p/x", ConditionType, "false")},
			AllowConditions: []authorizationv1.Condition{cond("p/x", ConditionType, "true")},
		}}, authorizationv1.ConditionsAwareDecisionTypeDeny},
		{"invalid id", &authorizationv1.ConditionsAwareDecision{Type: authorizationv1.ConditionsAwareDecisionTypeConditionsMap, ConditionsMap: &authorizationv1.ConditionsMap{
			AllowConditions: []authorizationv1.Condition{cond("not a key!", ConditionType, "true")},
		}}, authorizationv1.ConditionsAwareDecisionTypeNoOpinion},
		{"unknown condition type on deny errors to Deny", &authorizationv1.ConditionsAwareDecision{Type: authorizationv1.ConditionsAwareDecisionTypeConditionsMap, ConditionsMap: &authorizationv1.ConditionsMap{
			DenyConditions:  []authorizationv1.Condition{cond("p/d", "example.com/other", "false")},
			AllowConditions: []authorizationv1.Condition{cond("p/a", ConditionType, "true")},
		}}, authorizationv1.ConditionsAwareDecisionTypeDeny},
		{"unknown condition type on allow is ignored", &authorizationv1.ConditionsAwareDecision{Type: authorizationv1.ConditionsAwareDecisionTypeConditionsMap, ConditionsMap: &authorizationv1.ConditionsMap{
			AllowConditions: []authorizationv1.Condition{cond("p/a", "example.com/other", "true")},
		}}, authorizationv1.ConditionsAwareDecisionTypeNoOpinion},
		{"residual referencing request errors", &authorizationv1.ConditionsAwareDecision{Type: authorizationv1.ConditionsAwareDecisionTypeConditionsMap, ConditionsMap: &authorizationv1.ConditionsMap{
			DenyConditions: []authorizationv1.Condition{cond("p/d", ConditionType, "request.user == 'x'")},
		}}, authorizationv1.ConditionsAwareDecisionTypeDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := e.EvaluateConditions(context.Background(), tt.decision, req)
			require.Equal(t, tt.wanted, got.Type, "%+v", got)
		})
	}
}
