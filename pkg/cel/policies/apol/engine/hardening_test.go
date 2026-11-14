package engine

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/require"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// roundTrip returns the decision as kube-apiserver receives it on the wire.
func roundTrip(t *testing.T, d authorizationv1.ConditionsAwareDecision) authorizationv1.ConditionsAwareDecision {
	t.Helper()
	raw, err := json.Marshal(d)
	require.NoError(t, err)
	var out authorizationv1.ConditionsAwareDecision
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func TestTruncateKeepsValidUTF8(t *testing.T) {
	t.Parallel()
	s := strings.Repeat("é", 1000)
	got := truncate(s, authorizer.MaxConditionDescriptionBytes-3)
	require.True(t, utf8.ValidString(got))
	require.LessOrEqual(t, len(got), authorizer.MaxConditionDescriptionBytes)
	require.Equal(t, "ab", truncate("a\xffb", 10), "invalid UTF-8 is dropped")
	require.Equal(t, "abc", truncate("abc", 3))
}

// A multi-byte description that fits the CRD's character limit must not grow
// past the upstream byte limit on the wire, or the conditional decision fails closed.
func TestMultiByteDescriptionSurvivesTheWire(t *testing.T) {
	t.Parallel()
	r := rule("a", allowE, "object.metadata.name == 'ok'")
	r.Description = strings.Repeat("é", 1000)
	e := newEngine(t, policy("p", nil, r))
	d := roundTrip(t, e.Authorize(context.Background(), sar("create", "dev"), true))
	require.Equal(t, typeMap, d.Type, "%+v", d)
	for _, c := range d.ConditionsMap.AllowConditions {
		require.LessOrEqual(t, len(c.Description), authorizer.MaxConditionDescriptionBytes)
	}
	got := e.EvaluateConditions(context.Background(), &d, admission(`{"metadata":{"name":"ok"}}`, ""))
	require.Equal(t, typeAllow, got.Type, "%+v", got)
}

// `request` mirrors the SubjectAccessReview spec: has() reports absent
// optional fields as absent, so the standard guard pattern does not error.
func TestRequestHasGuards(t *testing.T) {
	t.Parallel()
	selDeny := rule("d", denyE, "has(request.resourceAttributes.labelSelector) && request.resourceAttributes.labelSelector.requirements.exists(r, r.key == 'team')")
	nonResDeny := rule("n", denyE, "has(request.nonResourceAttributes) && request.nonResourceAttributes.path == '/x'")
	e := newEngine(t, policy("p", nil, rule("a", allowE, "true"), selDeny, nonResDeny))

	d := e.Authorize(context.Background(), sar("list", "dev"), true)
	require.Equal(t, typeAllow, d.Type, "%+v", d)
	require.Empty(t, d.Allow.EvaluationError)

	withSel := sar("list", "dev")
	withSel.ResourceAttributes.LabelSelector = &authorizationv1.LabelSelectorAttributes{
		Requirements: []metav1.LabelSelectorRequirement{{Key: "team", Operator: metav1.LabelSelectorOpIn, Values: []string{"a"}}},
	}
	require.Equal(t, typeDeny, e.Authorize(context.Background(), withSel, true).Type)

	// A guarded Allow evaluates to false (not an error) when the field is absent.
	guardedAllow := newEngine(t, policy("p", nil, rule("a", allowE, "!has(request.resourceAttributes.fieldSelector)")))
	got := guardedAllow.Authorize(context.Background(), sar("list", "dev"), true)
	require.Equal(t, typeAllow, got.Type, "%+v", got)
}

// Exhausting the total cost budget must not make the decision depend on
// policy names or rule order.
func TestBudgetExhaustionIsOrderIndependent(t *testing.T) {
	t.Parallel()
	heavyExpr := "[" + strings.TrimSuffix(strings.Repeat("1,", 40), ",") + "].all(i, i > 0) && request.user == 'alice'"
	build := func(heavyName, denyName string) Engine {
		return engineWith(t, StaticProvider(compileAll(t,
			policy(heavyName, nil, rule("h", allowE, heavyExpr)),
			policy(denyName, nil, rule("d", denyE, "request.user == 'mallory'")),
		)...), WithLimits(Limits{TotalCostLimit: 50}))
	}
	a := build("a-heavy", "z-deny").Authorize(context.Background(), sar("get", "dev"), true)
	z := build("z-heavy", "a-deny").Authorize(context.Background(), sar("get", "dev"), true)
	require.Equal(t, a.Type, z.Type, "a=%+v z=%+v", a, z)
	require.Equal(t, typeDeny, a.Type, "budget exhaustion fails closed when a Deny rule applies")
	require.Contains(t, a.Deny.EvaluationError, "budget")

	// Without a Deny or NoOpinion rule, exhaustion yields NoOpinion, never Allow.
	onlyAllow := engineWith(t, StaticProvider(compileAll(t,
		policy("a", nil, rule("h", allowE, heavyExpr)),
		policy("b", nil, rule("x", allowE, "true")),
	)...), WithLimits(Limits{TotalCostLimit: 50}))
	require.Equal(t, typeNoOp, onlyAllow.Authorize(context.Background(), sar("get", "dev"), true).Type)
}

// When pruning folds a known sub-expression into one that no longer compiles,
// the residual falls back to the captured source expression and matches full evaluation.
func TestResidualFallsBackWhenPrunedFormDoesNotCompile(t *testing.T) {
	t.Parallel()
	for _, effect := range []policiesv1alpha1.AuthorizingRuleEffect{denyE, noOpE} {
		pols := []*policiesv1alpha1.AuthorizingPolicy{policy("p", nil,
			rule("a", allowE, "true"),
			rule("r", effect, "int(request.uid) > 0 && object.metadata.name == 'x'"),
		)}
		compiled := compileAll(t, pols...)
		e := engineWith(t, StaticProvider(compiled...))
		spec := sar("create", "dev")
		for _, name := range []string{"x", "y"} {
			req := admission(fmt.Sprintf(`{"metadata":{"name":%q}}`, name), "")
			require.Equal(t, fullEvaluate(t, compiled, spec, req), phased(t, e, spec, req).Type, "effect %s object %s", effect, name)
		}
	}
}

// The definite-effect condition IDs cannot collide with a policy named
// policies.kyverno.io, because rule names cannot contain `_`.
func TestDefiniteConditionIDsDoNotCollide(t *testing.T) {
	t.Parallel()
	e := newEngine(t,
		policy("policies.kyverno.io", nil, rule("definite-allow", allowE, "object.metadata.name == 'ok'")),
		policy("q", nil, rule("a", allowE, "true"), rule("d", denyE, "object.metadata.name == 'bad'")),
	)
	d := e.Authorize(context.Background(), sar("create", "dev"), true)
	require.Equal(t, typeMap, d.Type, "%+v", d)
	got := e.EvaluateConditions(context.Background(), &d, admission(`{"metadata":{"name":"fine"}}`, ""))
	require.Equal(t, typeAllow, got.Type, "%+v", got)
}

// Conditions reviews use their own concurrency slots, so they cannot starve
// SubjectAccessReviews (and the reverse).
func TestConditionsReviewsHaveSeparateSlots(t *testing.T) {
	t.Parallel()
	e := engineWith(t, StaticProvider(compileAll(t, policy("p", nil,
		rule("a", allowE, "true"), rule("d", denyE, "object.metadata.name == 'x'")))...), WithLimits(Limits{MaxConcurrent: 1}))
	eng := e.(*engine)
	d := e.Authorize(context.Background(), sar("create", "dev"), true)
	require.Equal(t, typeMap, d.Type)

	release, err := eng.review.acquire(context.Background())
	require.NoError(t, err)
	require.Equal(t, typeMap, e.Authorize(context.Background(), sar("create", "dev"), true).Type, "a held review slot must not block authorization")
	release()

	release, err = eng.authorize.acquire(context.Background())
	require.NoError(t, err)
	defer release()
	require.Equal(t, typeAllow, e.EvaluateConditions(context.Background(), &d, admission(`{"metadata":{"name":"y"}}`, "")).Type)
}
