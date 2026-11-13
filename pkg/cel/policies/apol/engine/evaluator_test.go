package engine

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"
	"time"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apiserver/pkg/cel/lazy"
)

const (
	typeAllow = authorizationv1.ConditionsAwareDecisionTypeAllow
	typeDeny  = authorizationv1.ConditionsAwareDecisionTypeDeny
	typeNoOp  = authorizationv1.ConditionsAwareDecisionTypeNoOpinion
	typeMap   = authorizationv1.ConditionsAwareDecisionTypeConditionsMap
)

func compileAll(t testing.TB, pols ...*policiesv1alpha1.AuthorizingPolicy) []CompiledPolicy {
	t.Helper()
	c := apolcompiler.NewCompiler()
	out := make([]CompiledPolicy, 0, len(pols))
	for _, p := range pols {
		out = append(out, NewCompiledPolicy(c, p))
	}
	return out
}

func engineWith(t testing.TB, provider Provider, opts ...Option) Engine {
	t.Helper()
	e, err := NewEngine(provider, opts...)
	require.NoError(t, err)
	return e
}

func admission(object, oldObject string) *admissionv1.AdmissionRequest {
	req := &admissionv1.AdmissionRequest{}
	if object != "" {
		req.Object = runtime.RawExtension{Raw: []byte(object)}
	}
	if oldObject != "" {
		req.OldObject = runtime.RawExtension{Raw: []byte(oldObject)}
	}
	return req
}

// phased runs the two-phase protocol: SubjectAccessReview, then (if
// conditional) the conditions review with the admission data.
func phased(t testing.TB, e Engine, spec *authorizationv1.SubjectAccessReviewSpec, req *admissionv1.AdmissionRequest) authorizationv1.ConditionsAwareDecision {
	t.Helper()
	d := e.Authorize(context.Background(), spec, true)
	if d.Type != typeMap {
		return d
	}
	return e.EvaluateConditions(context.Background(), &d, req)
}

// fullEvaluate is the reference complete-request evaluator: every rule of
// every applicable policy is evaluated with request, variables, object, and
// oldObject all known, then folded with the upstream ConditionsMap semantics
// (Deny true-or-error > NoOpinion true-or-error > Allow true > NoOpinion).
func fullEvaluate(t testing.TB, compiled []CompiledPolicy, spec *authorizationv1.SubjectAccessReviewSpec, req *admissionv1.AdmissionRequest) authorizationv1.ConditionsAwareDecisionType {
	t.Helper()
	var object, oldObject any
	if req != nil {
		var err error
		object, err = decodeRaw(req.Object.Raw, 64)
		require.NoError(t, err)
		oldObject, err = decodeRaw(req.OldObject.Raw, 64)
		require.NoError(t, err)
	}
	hit := map[policiesv1alpha1.AuthorizingRuleEffect]bool{}
	for _, p := range compiled {
		if !p.Applies(spec) {
			continue
		}
		if p.CompileError() != nil {
			for _, r := range p.Policy().Spec.Rules {
				if r.Effect != allowE {
					hit[r.Effect] = true
				}
			}
			continue
		}
		cp := p.(*compiledPolicy).compiled
		vars := lazy.NewMapValue(celcompiler.VariablesType)
		activation := map[string]any{
			celcompiler.RequestKey:   RequestValue(spec),
			celcompiler.VariablesKey: vars,
			celcompiler.ObjectKey:    object,
			celcompiler.OldObjectKey: oldObject,
		}
		for _, v := range cp.Variables {
			vars.Append(v.Name, func(*lazy.MapValue) ref.Val {
				out, _, err := v.Program.Eval(activation)
				if err != nil {
					return types.WrapErr(err)
				}
				return out
			})
		}
		for _, r := range cp.Rules {
			out, _, err := r.Program.Eval(activation)
			switch {
			case err != nil || types.IsError(out):
				if r.Effect != allowE {
					hit[r.Effect] = true
				}
			case out == types.True:
				hit[r.Effect] = true
			}
		}
	}
	switch {
	case hit[denyE]:
		return typeDeny
	case hit[noOpE]:
		return typeNoOp
	case hit[allowE]:
		return typeAllow
	}
	return typeNoOp
}

// rulePool mixes known, unknown, erroring, comprehension, and variable rules.
var rulePool = []string{
	"true",
	"false",
	"request.user == 'alice'",
	"'dev' in request.groups",
	"object.metadata.name == 'x'",
	"object.metadata.name == request.user",
	"has(object.metadata.labels) && object.metadata.labels.owner == request.user",
	"object.spec.replicas > 2",
	"oldObject != null && oldObject.metadata.name != object.metadata.name",
	"object.spec.containers.all(c, c.name.startsWith(request.user))",
	"object.metadata.labels.exists(k, k == request.resourceAttributes.namespace)",
	"request.extra['missing'][0] == 'x'",
	"request.missing.x == 1 || object.metadata.name == 'x'",
	"variables.team == 'a' && object.metadata.name.startsWith(variables.prefix)",
	"variables.prefixes.exists(p, object.metadata.name.startsWith(p))",
}

var poolVariables = []admissionregistrationv1.Variable{
	{Name: "team", Expression: "request.resourceAttributes.namespace == 'team-a' ? 'a' : 'b'"},
	{Name: "prefix", Expression: "request.user + '-'"},
	{Name: "prefixes", Expression: "[string(request.user), 'shared']"},
}

var effects = []policiesv1alpha1.AuthorizingRuleEffect{allowE, denyE, noOpE}

func TestPoolPoliciesCompile(t *testing.T) {
	t.Parallel()
	for i, expr := range rulePool {
		require.NoError(t, compileAll(t, policy("p", poolVariables, rule("r", allowE, expr)))[0].CompileError(), "rule %d", i)
	}
}

func randomPolicies(rng *rand.Rand) []*policiesv1alpha1.AuthorizingPolicy {
	n := 1 + rng.Intn(4)
	out := make([]*policiesv1alpha1.AuthorizingPolicy, 0, n)
	for i := 0; i < n; i++ {
		m := 1 + rng.Intn(4)
		var rules []policiesv1alpha1.AuthorizingRule
		for j := 0; j < m; j++ {
			rules = append(rules, rule(fmt.Sprintf("r%d", j), effects[rng.Intn(len(effects))], rulePool[rng.Intn(len(rulePool))]))
		}
		out = append(out, policy(fmt.Sprintf("p%d", i), poolVariables, rules...))
	}
	return out
}

var admissionPool = []*admissionv1.AdmissionRequest{
	admission(`{"metadata":{"name":"x","labels":{"owner":"alice","team-a":"1"}},"spec":{"replicas":3,"containers":[{"name":"alice-c"}]}}`, ""),
	admission(`{"metadata":{"name":"alice-1","labels":{"owner":"bob"}},"spec":{"replicas":1,"containers":[{"name":"bob"}]}}`, `{"metadata":{"name":"alice-1"}}`),
	admission(`{"metadata":{"name":"y"}}`, `{"metadata":{"name":"z"}}`),
	admission(`{"metadata":{"name":"shared-1","labels":{}},"spec":{"containers":[]}}`, ""),
	admission("", `{"metadata":{"name":"x"}}`),
}

// canonical renders a decision independently of policy and rule names.
func canonical(d authorizationv1.ConditionsAwareDecision, rename func(string) string) string {
	if d.Type != typeMap {
		return string(d.Type)
	}
	list := func(conds []authorizationv1.Condition) string {
		var s []string
		for _, c := range conds {
			s = append(s, rename(c.ID)+"="+c.Condition)
		}
		slices.Sort(s)
		return strings.Join(s, "|")
	}
	m := d.ConditionsMap
	return fmt.Sprintf("map deny[%s] noop[%s] allow[%s]", list(m.DenyConditions), list(m.NoOpinionConditions), list(m.AllowConditions))
}

// TestPermutationInvariance is a property test: permuting policies, rules,
// provider insertion order, snapshot generations, and policy names never
// changes the decision.
func TestPermutationInvariance(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(18621)) //nolint:gosec
	specs := []*authorizationv1.SubjectAccessReviewSpec{sar("create", "dev"), sar("update", "dev", "ops"), sar("get", "dev"), sar("delete", "dev")}
	for iter := 0; iter < 60; iter++ {
		pols := randomPolicies(rng)
		base := engineWith(t, StaticProvider(compileAll(t, pols...)...))
		type variant struct {
			engine Engine
			rename func(string) string
		}
		var variants []variant
		for perm := 0; perm < 4; perm++ {
			// Shuffle policies and the rules inside each policy, and rename policies.
			shuffled := make([]*policiesv1alpha1.AuthorizingPolicy, 0, len(pols))
			names := map[string]string{}
			for _, i := range rng.Perm(len(pols)) {
				p := pols[i].DeepCopy()
				newName := fmt.Sprintf("q%d-%d", perm, rng.Intn(1_000_000))
				names[newName] = p.Name
				p.Name = newName
				rng.Shuffle(len(p.Spec.Rules), func(a, b int) { p.Spec.Rules[a], p.Spec.Rules[b] = p.Spec.Rules[b], p.Spec.Rules[a] })
				shuffled = append(shuffled, p)
			}
			snapshot := NewSnapshot(uint64(perm+100), compileAll(t, shuffled...)...)
			variants = append(variants, variant{
				engine: engineWith(t, ProviderFunc(func(context.Context) (*Snapshot, error) { return snapshot, nil })),
				rename: func(id string) string {
					policyName, ruleName, ok := strings.Cut(id, "/")
					if orig, found := names[policyName]; ok && found {
						return orig + "/" + ruleName
					}
					return id
				},
			})
		}
		for _, spec := range specs {
			for _, aware := range []bool{true, false} {
				want := base.Authorize(context.Background(), spec, aware)
				for perm, v := range variants {
					got := v.engine.Authorize(context.Background(), spec, aware)
					require.Equal(t, canonical(want, func(s string) string { return s }), canonical(got, v.rename), "iteration %d permutation %d spec %s aware %v", iter, perm, spec.ResourceAttributes.Verb, aware)
					if want.Type == typeMap {
						for _, req := range admissionPool {
							require.Equal(t, base.EvaluateConditions(context.Background(), &want, req).Type, v.engine.EvaluateConditions(context.Background(), &got, req).Type)
						}
					}
				}
			}
		}
	}
}

// TestPhasedMatchesFullEvaluation checks that partial evaluation followed by
// the conditions review gives the same decision as evaluating each request
// with all inputs known, including error cases.
func TestPhasedMatchesFullEvaluation(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(15778)) //nolint:gosec
	specs := []*authorizationv1.SubjectAccessReviewSpec{sar("create", "dev"), sar("update", "dev", "ops"), sar("delete", "dev")}
	specs[1].User = "bob"
	for iter := 0; iter < 100; iter++ {
		pols := randomPolicies(rng)
		compiled := compileAll(t, pols...)
		e := engineWith(t, StaticProvider(compiled...))
		for _, spec := range specs {
			for i, req := range admissionPool {
				want := fullEvaluate(t, compiled, spec, req)
				got := phased(t, e, spec, req)
				require.Equal(t, want, got.Type, "iteration %d admission %d verb %s user %s: %+v", iter, i, spec.ResourceAttributes.Verb, spec.User, got)
			}
		}
	}
}

func TestPhasedMatchesFullEvaluationTable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		pols []*policiesv1alpha1.AuthorizingPolicy
	}{
		{"satisfied allow with residual deny", []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil, rule("a", allowE, "true"), rule("d", denyE, "object.metadata.name == 'x'"))}},
		{"satisfied noopinion with residual deny", []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil, rule("n", noOpE, "true"), rule("d", denyE, "object.metadata.name == 'x'"), rule("a", allowE, "true"))}},
		{"satisfied allow with residual noopinion", []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil, rule("a", allowE, "true"), rule("n", noOpE, "object.metadata.name == 'x'"))}},
		{"residual allow and residual deny across policies", []*policiesv1alpha1.AuthorizingPolicy{
			policy("a", nil, rule("a", allowE, "object.metadata.name.startsWith(request.user)")),
			policy("b", nil, rule("d", denyE, "has(object.metadata.labels) && 'protected' in object.metadata.labels")),
		}},
		{"comprehension capturing request", []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil, rule("a", allowE, "object.spec.containers.all(c, c.name.startsWith(request.user))"))}},
		{"erroring known part with unknown part", []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil, rule("a", allowE, "request.missing.x == 1 || object.metadata.name == 'x'"), rule("d", denyE, "request.missing.x == 1 && object.metadata.name == 'y'"))}},
		{"variables captured", []*policiesv1alpha1.AuthorizingPolicy{policy("p", poolVariables, rule("a", allowE, "variables.prefixes.exists(p, object.metadata.name.startsWith(p))"), rule("d", denyE, "object.metadata.name == variables.prefix + 'blocked'"))}},
		{"oldObject transitions", []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil, rule("a", allowE, "true"), rule("d", denyE, "oldObject != null && oldObject.metadata.name != object.metadata.name"))}},
	}
	reqs := append(slices.Clone(admissionPool),
		admission(`{"metadata":{"name":"alice-blocked"}}`, ""),
		admission(`{"metadata":{"name":"x","labels":{"protected":""}}}`, ""),
	)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			compiled := compileAll(t, tt.pols...)
			e := engineWith(t, StaticProvider(compiled...))
			for i, req := range reqs {
				spec := sar("update", "dev")
				require.Equal(t, fullEvaluate(t, compiled, spec, req), phased(t, e, spec, req).Type, "admission %d", i)
			}
		})
	}
}

func TestSatisfiedEffectsStayConditional(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		rules []policiesv1alpha1.AuthorizingRule
		check func(t *testing.T, m *authorizationv1.ConditionsMap)
		// wantX is the conditions-review decision for an object named x.
		wantX authorizationv1.ConditionsAwareDecisionType
	}{{
		wantX: typeDeny,
		name:  "satisfied allow and residual deny",
		rules: []policiesv1alpha1.AuthorizingRule{rule("a", allowE, "true"), rule("d", denyE, "object.metadata.name == 'x'")},
		check: func(t *testing.T, m *authorizationv1.ConditionsMap) {
			require.Len(t, m.DenyConditions, 1)
			require.Equal(t, `object.metadata.name == "x"`, m.DenyConditions[0].Condition)
			require.Equal(t, definiteAllowID, m.AllowConditions[0].ID)
		},
	}, {
		name:  "satisfied allow and residual noopinion",
		wantX: typeNoOp,
		rules: []policiesv1alpha1.AuthorizingRule{rule("a", allowE, "true"), rule("n", noOpE, "object.metadata.name == 'x'")},
		check: func(t *testing.T, m *authorizationv1.ConditionsMap) {
			require.Len(t, m.NoOpinionConditions, 1)
			require.Len(t, m.AllowConditions, 1)
		},
	}, {
		name:  "satisfied noopinion and residual deny",
		wantX: typeDeny,
		rules: []policiesv1alpha1.AuthorizingRule{rule("n", noOpE, "true"), rule("d", denyE, "object.metadata.name == 'x'"), rule("a", allowE, "object.metadata.name == 'y'")},
		check: func(t *testing.T, m *authorizationv1.ConditionsMap) {
			require.Len(t, m.DenyConditions, 1)
			require.Equal(t, definiteNoOpinionID, m.NoOpinionConditions[0].ID)
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newEngine(t, policy("p", nil, tt.rules...))
			d := e.Authorize(context.Background(), sar("create", "dev"), true)
			require.Equal(t, typeMap, d.Type, "%+v", d)
			tt.check(t, d.ConditionsMap)
			require.Equal(t, tt.wantX, e.EvaluateConditions(context.Background(), &d, admission(`{"metadata":{"name":"x"}}`, "")).Type)
			// For any other object the satisfied effect applies.
			other := e.EvaluateConditions(context.Background(), &d, admission(`{"metadata":{"name":"z"}}`, "")).Type
			require.NotEqual(t, typeDeny, other)
		})
	}
}

// TestFieldStates distinguishes unknown (authorization time), absent, null,
// empty, and malformed object fields.
func TestFieldStates(t *testing.T) {
	t.Parallel()
	pols := []*policiesv1alpha1.AuthorizingPolicy{
		policy("eq", nil, rule("a", allowE, "object.spec.x == 'a'")),
		policy("eqdeny", nil, rule("d", denyE, "object.spec.x == 'a'"), rule("a", allowE, "true")),
		policy("has", nil, rule("a", allowE, "has(object.spec.x)")),
		policy("null", nil, rule("a", allowE, "object.spec.x == null")),
		policy("empty", nil, rule("a", allowE, "object.spec.x == ''")),
	}
	byName := map[string]*policiesv1alpha1.AuthorizingPolicy{}
	for _, p := range pols {
		byName[p.Name] = p
	}
	// At authorization time every object field is unknown: never absent or null.
	for _, p := range pols {
		d := newEngine(t, p).Authorize(context.Background(), sar("create", "dev"), true)
		require.Equal(t, typeMap, d.Type, "policy %s must stay conditional: %+v", p.Name, d)
	}
	tests := []struct {
		state  string
		object string
		want   map[string]authorizationv1.ConditionsAwareDecisionType
	}{
		{"present", `{"spec":{"x":"a"}}`, map[string]authorizationv1.ConditionsAwareDecisionType{"eq": typeAllow, "eqdeny": typeDeny, "has": typeAllow, "null": typeNoOp, "empty": typeNoOp}},
		// absent: selecting a missing key is an error -> Allow dropped, Deny applies.
		{"absent", `{"spec":{}}`, map[string]authorizationv1.ConditionsAwareDecisionType{"eq": typeNoOp, "eqdeny": typeDeny, "has": typeNoOp, "null": typeNoOp, "empty": typeNoOp}},
		{"null", `{"spec":{"x":null}}`, map[string]authorizationv1.ConditionsAwareDecisionType{"eq": typeNoOp, "eqdeny": typeAllow, "has": typeAllow, "null": typeAllow, "empty": typeNoOp}},
		{"empty", `{"spec":{"x":""}}`, map[string]authorizationv1.ConditionsAwareDecisionType{"eq": typeNoOp, "eqdeny": typeAllow, "has": typeAllow, "null": typeNoOp, "empty": typeAllow}},
		// object itself is null (for example a DELETE): every select errors.
		{"null object", ``, map[string]authorizationv1.ConditionsAwareDecisionType{"eq": typeNoOp, "eqdeny": typeDeny, "has": typeNoOp, "null": typeNoOp, "empty": typeNoOp}},
		// malformed JSON fails closed to the decision's failure decision.
		{"malformed", `{"spec":`, map[string]authorizationv1.ConditionsAwareDecisionType{"eq": typeNoOp, "eqdeny": typeDeny, "has": typeNoOp, "null": typeNoOp, "empty": typeNoOp}},
		// wrong type: comparing a number with a string is false, not an error.
		{"wrong type", `{"spec":{"x":1}}`, map[string]authorizationv1.ConditionsAwareDecisionType{"eq": typeNoOp, "eqdeny": typeAllow, "has": typeAllow, "null": typeNoOp, "empty": typeNoOp}},
	}
	for _, tt := range tests {
		for name, want := range tt.want {
			e := newEngine(t, byName[name])
			got := phased(t, e, sar("create", "dev"), admission(tt.object, ""))
			require.Equal(t, want, got.Type, "state %s policy %s: %+v", tt.state, name, got)
		}
	}
}

func TestResidualsAreSelfContained(t *testing.T) {
	t.Parallel()
	e := newEngine(t, policy("p", poolVariables,
		rule("comprehension", allowE, "object.spec.containers.all(c, c.name.startsWith(request.user))"),
		rule("vars", allowE, "variables.prefixes.exists(p, object.metadata.name.startsWith(p))"),
		rule("pruned", denyE, "object.metadata.name == request.user + '-blocked'"),
	))
	d := e.Authorize(context.Background(), sar("create", "dev"), true)
	require.Equal(t, typeMap, d.Type, "%+v", d)
	env, err := apolcompiler.NewConditionsEnv()
	require.NoError(t, err)
	for _, c := range append(slices.Clone(d.ConditionsMap.AllowConditions), d.ConditionsMap.DenyConditions...) {
		_, iss := env.Compile(c.Condition)
		require.NoError(t, iss.Err(), c.Condition)
		t.Logf("%s: %s", c.ID, c.Condition)
	}
	pruned := d.ConditionsMap.DenyConditions[0].Condition
	require.Equal(t, `object.metadata.name == "alice-blocked"`, pruned)
	// The same decision evaluates identically on another replica that has no policies at all.
	other := engineWith(t, StaticProvider())
	req := admission(`{"metadata":{"name":"alice-x"},"spec":{"containers":[{"name":"alice-a"}]}}`, "")
	require.Equal(t, typeAllow, e.EvaluateConditions(context.Background(), &d, req).Type)
	require.Equal(t, typeAllow, other.EvaluateConditions(context.Background(), &d, req).Type)
}

func TestCallbackIgnoresPolicyUpdates(t *testing.T) {
	t.Parallel()
	current := NewSnapshot(1, compileAll(t, policy("p", nil, rule("a", allowE, "object.metadata.name == 'x'")))...)
	e := engineWith(t, ProviderFunc(func(context.Context) (*Snapshot, error) { return current, nil }))
	d := e.Authorize(context.Background(), sar("create", "dev"), true)
	require.Equal(t, typeMap, d.Type)
	// The policy is deleted and replaced with a Deny-everything policy.
	current = NewSnapshot(2, compileAll(t, policy("p", nil, rule("d", denyE, "true")))...)
	require.Equal(t, typeAllow, e.EvaluateConditions(context.Background(), &d, admission(`{"metadata":{"name":"x"}}`, "")).Type)
	require.Equal(t, typeDeny, e.Authorize(context.Background(), sar("create", "dev"), true).Type)
}

func TestMissingSnapshotFailsClosed(t *testing.T) {
	t.Parallel()
	for name, p := range map[string]Provider{
		"not ready": ProviderFunc(func(context.Context) (*Snapshot, error) { return nil, ErrNoSnapshot }),
		"nil":       ProviderFunc(func(context.Context) (*Snapshot, error) { return nil, nil }),
		"error":     ProviderFunc(func(context.Context) (*Snapshot, error) { return nil, errors.New("boom") }),
	} {
		d := engineWith(t, p).Authorize(context.Background(), sar("get", "dev"), true)
		require.Equal(t, typeDeny, d.Type, name)
		require.NotEmpty(t, d.Deny.EvaluationError, name)
	}
	d := engineWith(t, StaticProvider()).Authorize(context.Background(), nil, true)
	require.Equal(t, typeDeny, d.Type)
}

func TestInvalidCompilation(t *testing.T) {
	t.Parallel()
	bad := policy("bad", nil, rule("a", allowE, "this is not cel"))
	badDeny := policy("bad-deny", nil, rule("d", denyE, "object.nope("), rule("a", allowE, "true"))
	notBool := policy("not-bool", nil, rule("a", allowE, "'string'"))
	good := policy("good", nil, rule("a", allowE, "true"))
	for _, p := range []*policiesv1alpha1.AuthorizingPolicy{bad, badDeny, notBool} {
		require.Error(t, compileAll(t, p)[0].CompileError(), p.Name)
	}
	// An invalid Allow-only policy is excluded (its Allow never grants) but does not hide valid policies.
	require.Equal(t, typeNoOp, newEngine(t, bad).Authorize(context.Background(), sar("get", "dev"), true).Type)
	require.Equal(t, typeAllow, newEngine(t, bad, good).Authorize(context.Background(), sar("get", "dev"), true).Type)
	// An invalid policy never silently exempts: its Deny rules apply.
	d := newEngine(t, badDeny, good).Authorize(context.Background(), sar("get", "dev"), true)
	require.Equal(t, typeDeny, d.Type)
	require.Contains(t, d.Deny.EvaluationError, "failed to compile")
}

// nestedLoops returns an expression costing more than the per-call limit.
func nestedLoops() string {
	list := "[1,2,3,4,5,6,7,8,9,10]"
	expr := "true"
	for _, v := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		expr = fmt.Sprintf("%s.all(%s, %s)", list, v, expr)
	}
	return "request.user == 'alice' && " + expr
}

func TestCostExhaustion(t *testing.T) {
	t.Parallel()
	expensive := nestedLoops()
	t.Run("per expression limit", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, typeNoOp, newEngine(t, policy("p", nil, rule("a", allowE, expensive))).Authorize(context.Background(), sar("get", "dev"), true).Type)
		d := newEngine(t, policy("p", nil, rule("a", allowE, "true"), rule("d", denyE, expensive))).Authorize(context.Background(), sar("get", "dev"), true)
		require.Equal(t, typeDeny, d.Type)
		require.Contains(t, d.Deny.EvaluationError, "cost")
	})
	t.Run("total budget", func(t *testing.T) {
		t.Parallel()
		var rules []policiesv1alpha1.AuthorizingRule
		for i := 0; i < 10; i++ {
			rules = append(rules, rule(fmt.Sprintf("a%d", i), allowE, "request.groups.exists(g, g == 'dev')"))
		}
		rules = append(rules, rule("d", denyE, "request.groups.exists(g, g == 'nope')"))
		e := engineWith(t, StaticProvider(compileAll(t, policy("p", nil, rules...))...), WithLimits(Limits{TotalCostLimit: 5}))
		d := e.Authorize(context.Background(), sar("get", "dev"), true)
		require.Equal(t, typeDeny, d.Type, "%+v", d)
		require.Contains(t, d.Deny.EvaluationError, "budget")
		// Without the tiny budget the Deny rule is false and the Allow rules win.
		require.Equal(t, typeAllow, newEngine(t, policy("p", nil, rules...)).Authorize(context.Background(), sar("get", "dev"), true).Type)
	})
	t.Run("conditions review budget", func(t *testing.T) {
		t.Parallel()
		e := engineWith(t, StaticProvider(compileAll(t, policy("p", nil,
			rule("a", allowE, "true"),
			rule("d1", denyE, "object.spec.containers.exists(c, c.name == 'evil')"),
			rule("d2", denyE, "object.spec.containers.exists(c, c.name == 'evil2')"),
		))...), WithLimits(Limits{TotalCostLimit: 3}))
		d := e.Authorize(context.Background(), sar("create", "dev"), true)
		require.Equal(t, typeMap, d.Type)
		got := e.EvaluateConditions(context.Background(), &d, admission(`{"spec":{"containers":[{"name":"a"},{"name":"b"},{"name":"c"},{"name":"d"}]}}`, ""))
		require.Equal(t, typeDeny, got.Type, "%+v", got)
	})
}

func TestCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := newEngine(t, policy("p", nil, rule("a", allowE, "true")))
	d := e.Authorize(ctx, sar("get", "dev"), true)
	require.Equal(t, typeDeny, d.Type)
	require.Contains(t, d.Deny.EvaluationError, "canceled")

	// Cancellation during evaluation turns the remaining rules into errors.
	pol := compileAll(t, policy("p", nil, rule("a", allowE, "true"), rule("d", denyE, "false")))[0]
	eng := e.(*engine)
	res := newRuleResult()
	eng.evaluatePolicy(ctx, &evaluation{limits: eng.limits, request: RequestValue(sar("get", "dev"))}, pol, res)
	require.Equal(t, typeDeny, decide(sar("get", "dev"), true, res).Type)

	// Conditions review: a canceled callback fails closed.
	live := newEngine(t, policy("p", nil, rule("a", allowE, "true"), rule("d", denyE, "object.metadata.name == 'x'")))
	md := live.Authorize(context.Background(), sar("create", "dev"), true)
	require.Equal(t, typeMap, md.Type)
	require.Equal(t, typeDeny, live.EvaluateConditions(ctx, &md, admission(`{"metadata":{"name":"y"}}`, "")).Type)
}

func TestConcurrencyLimit(t *testing.T) {
	t.Parallel()
	e := engineWith(t, StaticProvider(compileAll(t, policy("p", nil, rule("a", allowE, "true")))...), WithLimits(Limits{MaxConcurrent: 1}))
	eng := e.(*engine)
	release, err := eng.acquire(context.Background())
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	d := e.Authorize(ctx, sar("get", "dev"), true)
	require.Equal(t, typeDeny, d.Type)
	release()
	require.Equal(t, typeAllow, e.Authorize(context.Background(), sar("get", "dev"), true).Type)
}

func TestInputBounds(t *testing.T) {
	t.Parallel()
	t.Run("request size", func(t *testing.T) {
		t.Parallel()
		pols := compileAll(t, policy("p", nil, rule("a", allowE, "true"), rule("d", denyE, "false")))
		e := engineWith(t, StaticProvider(pols...), WithLimits(Limits{MaxRequestBytes: 512}))
		spec := sar("get", "dev")
		for i := 0; i < 100; i++ {
			spec.Groups = append(spec.Groups, fmt.Sprintf("group-%d", i))
		}
		d := e.Authorize(context.Background(), spec, true)
		require.Equal(t, typeDeny, d.Type)
		require.Contains(t, d.Deny.EvaluationError, "byte limit")
	})
	t.Run("residual size", func(t *testing.T) {
		t.Parallel()
		pols := compileAll(t, policy("p", nil, rule("a", allowE, "true"), rule("d", denyE, "object.spec.containers.all(c, c.name.startsWith(request.user))")))
		e := engineWith(t, StaticProvider(pols...), WithLimits(Limits{MaxConditionBytes: 40}))
		d := e.Authorize(context.Background(), sar("create", "dev"), true)
		require.Equal(t, typeDeny, d.Type, "an oversized Deny residual is an error, so the Deny applies")
	})
	t.Run("object size and depth", func(t *testing.T) {
		t.Parallel()
		pols := compileAll(t, policy("p", nil, rule("a", allowE, "true"), rule("d", denyE, "object.metadata.name == 'x'")))
		e := engineWith(t, StaticProvider(pols...), WithLimits(Limits{MaxObjectBytes: 64, MaxValueDepth: 4}))
		d := e.Authorize(context.Background(), sar("create", "dev"), true)
		require.Equal(t, typeMap, d.Type)
		big := `{"metadata":{"name":"` + strings.Repeat("y", 100) + `"}}`
		require.Equal(t, typeDeny, e.EvaluateConditions(context.Background(), &d, admission(big, "")).Type)
		deep := `{"metadata":{"name":"y"},"a":{"b":{"c":{"d":{"e":{}}}}}}`
		require.Equal(t, typeDeny, e.EvaluateConditions(context.Background(), &d, admission(deep, "")).Type)
		require.Equal(t, typeAllow, e.EvaluateConditions(context.Background(), &d, admission(`{"metadata":{"name":"y"}}`, "")).Type)
	})
}

func TestUncapturableVariableIsAnError(t *testing.T) {
	t.Parallel()
	vars := []admissionregistrationv1.Variable{{Name: "bad", Expression: "request.extra['missing'][0]"}}
	pols := compileAll(t, policy("p", vars, rule("a", allowE, "true"), rule("d", denyE, "object.metadata.name == variables.bad")))
	d := engineWith(t, StaticProvider(pols...)).Authorize(context.Background(), sar("create", "dev"), true)
	require.Equal(t, typeDeny, d.Type)
}

func TestLiteralBuilder(t *testing.T) {
	t.Parallel()
	env, err := cel.NewEnv()
	require.NoError(t, err)
	val := types.DefaultTypeAdapter.NativeToValue(map[string]any{
		"s": "quote\" and \\ and \n and é", "i": int64(-3), "u": uint64(4), "f": 1.5, "b": true, "n": nil,
		"l": []any{"a", int64(1), nil, map[string]any{}}, "m": map[string]any{"k": []any{}},
		"bytes": []byte{0, 1}, "dur": 3 * time.Second, "ts": time.Unix(0, 0).UTC(),
	})
	src, err := newLiteralBuilder(8).render(val)
	require.NoError(t, err)
	ast, iss := env.Compile(src)
	require.NoError(t, iss.Err(), src)
	prog, err := env.Program(ast)
	require.NoError(t, err)
	out, _, err := prog.Eval(cel.NoVars())
	require.NoError(t, err)
	require.Equal(t, types.True, out.Equal(val), src)
	_, err = newLiteralBuilder(1).render(types.DefaultTypeAdapter.NativeToValue(map[string]any{"a": map[string]any{"b": map[string]any{}}}))
	require.Error(t, err)
}

func TestSnapshotIsImmutableAndSorted(t *testing.T) {
	t.Parallel()
	pols := compileAll(t, policy("b", nil, rule("a", allowE, "true")), policy("a", nil, rule("a", allowE, "true")))
	s := NewSnapshot(7, pols...)
	pols[0] = nil
	require.Equal(t, uint64(7), s.Generation())
	require.Equal(t, 2, s.Len())
	got := s.Policies()
	require.Equal(t, "a", got[0].Policy().Name)
	got[0] = nil
	require.NotNil(t, s.Policies()[0])
}

func TestDecisionIsDeterministicJSON(t *testing.T) {
	t.Parallel()
	pols := []*policiesv1alpha1.AuthorizingPolicy{
		policy("z", nil, rule("a", allowE, "object.metadata.name == 'x'")),
		policy("a", nil, rule("d", denyE, "request.extra['missing'][0] == 'x'"), rule("n", noOpE, "request.extra['missing'][1] == 'x'")),
	}
	first, err := json.Marshal(newEngine(t, pols...).Authorize(context.Background(), sar("create", "dev"), true))
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(newEngine(t, pols[1], pols[0]).Authorize(context.Background(), sar("create", "dev"), true))
		require.NoError(t, err)
		require.JSONEq(t, string(first), string(got))
	}
}

func BenchmarkAuthorize(b *testing.B) {
	var pols []*policiesv1alpha1.AuthorizingPolicy
	for i := 0; i < 30; i++ {
		pols = append(pols, policy(fmt.Sprintf("p%02d", i), poolVariables,
			rule("known", allowE, "request.user == 'alice'"),
			rule("residual", denyE, "has(object.metadata.labels) && object.metadata.labels.owner != request.user"),
			rule("comprehension", noOpE, "object.spec.containers.exists(c, c.image.startsWith(variables.prefix))"),
		))
	}
	e := engineWith(b, StaticProvider(compileAll(b, pols...)...))
	spec := sar("create", "dev")
	d := e.Authorize(context.Background(), spec, true)
	require.Equal(b, typeMap, d.Type)
	req := admission(`{"metadata":{"labels":{"owner":"alice"}},"spec":{"containers":[{"image":"nginx"}]}}`, "")
	b.Run("authorize", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			e.Authorize(context.Background(), spec, true)
		}
	})
	b.Run("conditions", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			e.EvaluateConditions(context.Background(), &d, req)
		}
	})
}

// TestPruningDoesNotMutateCompiledPolicy guards against the pruner mutating
// the shared macro-call map of a compiled AST across requests.
func TestPruningDoesNotMutateCompiledPolicy(t *testing.T) {
	t.Parallel()
	compiled := compileAll(t, policy("p", poolVariables,
		rule("a", allowE, "object.spec.containers.all(c, c.name.startsWith(request.user)) && variables.prefixes.exists(p, p == 'shared')"),
	))
	ast := compiled[0].(*compiledPolicy).compiled.Rules[0].AST.NativeRep()
	before := len(ast.SourceInfo().MacroCalls())
	e := engineWith(t, StaticProvider(compiled...))
	first := e.Authorize(context.Background(), sar("create", "dev"), true)
	require.Equal(t, typeMap, first.Type)
	done := make(chan authorizationv1.ConditionsAwareDecision, 8)
	for i := 0; i < cap(done); i++ {
		go func() { done <- e.Authorize(context.Background(), sar("create", "dev"), true) }()
	}
	for i := 0; i < cap(done); i++ {
		require.Equal(t, first, <-done)
	}
	require.Equal(t, before, len(ast.SourceInfo().MacroCalls()))
}
