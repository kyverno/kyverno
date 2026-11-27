package engine

import (
	"context"
	"fmt"
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/require"
	authorizationv1 "k8s.io/api/authorization/v1"
)

func subjectReq(key policiesv1alpha1.SubjectSelectorKey, op policiesv1alpha1.SubjectSelectorOperator, values ...string) policiesv1alpha1.SubjectSelectorRequirement {
	return policiesv1alpha1.SubjectSelectorRequirement{Key: key, Operator: op, Values: values}
}

func term(reqs ...policiesv1alpha1.SubjectSelectorRequirement) policiesv1alpha1.SubjectSelectorTerm {
	return policiesv1alpha1.SubjectSelectorTerm{MatchExpressions: reqs}
}

// denyAllFor returns a policy that unconditionally denies every request it
// selects, so the decision is Deny exactly when the selectors match.
func denyAllFor(terms []policiesv1alpha1.SubjectSelectorTerm, rules ...policiesv1alpha1.AuthorizingResourceRule) *policiesv1alpha1.AuthorizingPolicy {
	p := policy("sel", nil, rule("deny", denyE, "true"))
	p.Spec.MatchSubjects = terms
	if rules != nil {
		p.Spec.MatchResources.ResourceRules = rules
	}
	return p
}

func subject(user string, groups ...string) *authorizationv1.SubjectAccessReviewSpec {
	s := sar("get")
	s.User = user
	s.Groups = groups
	return s
}

// TestSubjectSelectorSemantics checks matchSubjects end to end through Authorize.
// Terms are ORed, requirements within a term are ANDed, and NotIn on groups is a
// set negation: a request is selected only when none of its groups is listed.
func TestSubjectSelectorSemantics(t *testing.T) {
	t.Parallel()
	const (
		user   = policiesv1alpha1.SubjectSelectorKeyUser
		groups = policiesv1alpha1.SubjectSelectorKeyGroups
		sa     = policiesv1alpha1.SubjectSelectorKeyServiceAccount
		in     = policiesv1alpha1.SubjectSelectorOpIn
		notIn  = policiesv1alpha1.SubjectSelectorOpNotIn
		exists = policiesv1alpha1.SubjectSelectorOpExists
		absent = policiesv1alpha1.SubjectSelectorOpDoesNotExist
	)
	terms := func(ts ...policiesv1alpha1.SubjectSelectorTerm) []policiesv1alpha1.SubjectSelectorTerm { return ts }
	tests := []struct {
		name    string
		terms   []policiesv1alpha1.SubjectSelectorTerm
		spec    *authorizationv1.SubjectAccessReviewSpec
		matches bool
	}{
		// Multi-group negation. A per-group "any group is not listed" check would
		// select these requests and deny the admin.
		{"groups NotIn excludes a member of any listed group", terms(term(subjectReq(groups, notIn, "admins"))), subject("alice", "dev", "admins"), false},
		{"groups NotIn excludes when listed group is last", terms(term(subjectReq(groups, notIn, "admins", "ops"))), subject("alice", "dev", "qa", "ops"), false},
		{"groups NotIn selects when no group is listed", terms(term(subjectReq(groups, notIn, "admins", "ops"))), subject("alice", "dev", "qa"), true},
		{"groups NotIn selects a subject with no groups", terms(term(subjectReq(groups, notIn, "admins"))), subject("alice"), true},
		{"groups NotIn with no values never matches", terms(term(subjectReq(groups, notIn))), subject("alice", "dev"), false},
		{"groups In matches any listed group", terms(term(subjectReq(groups, in, "ops", "dev"))), subject("alice", "qa", "dev"), true},
		{"groups In does not match unlisted groups", terms(term(subjectReq(groups, in, "ops"))), subject("alice", "dev"), false},
		{"groups In is case sensitive", terms(term(subjectReq(groups, in, "Dev"))), subject("alice", "dev"), false},
		{"groups In wildcard needs at least one group", terms(term(subjectReq(groups, in, "*"))), subject("alice"), false},
		{"groups In wildcard matches any group", terms(term(subjectReq(groups, in, "*"))), subject("alice", "x"), true},
		{"groups Exists", terms(term(subjectReq(groups, exists))), subject("alice", "x"), true},
		{"groups Exists without groups", terms(term(subjectReq(groups, exists))), subject("alice"), false},
		{"groups DoesNotExist", terms(term(subjectReq(groups, absent))), subject("alice"), true},
		{"groups DoesNotExist with groups", terms(term(subjectReq(groups, absent))), subject("alice", "x"), false},
		{"user In", terms(term(subjectReq(user, in, "bob", "alice"))), subject("alice"), true},
		{"user NotIn excludes listed user", terms(term(subjectReq(user, notIn, "alice"))), subject("alice", "dev"), false},
		{"user NotIn selects other users", terms(term(subjectReq(user, notIn, "alice"))), subject("bob", "dev"), true},
		{"user Exists is not a valid operator and never matches", terms(term(subjectReq(user, exists))), subject("alice"), false},
		{"unknown key never matches", terms(term(subjectReq("uid", in, "*"))), subject("alice", "dev"), false},
		{"serviceAccount In", terms(term(subjectReq(sa, in, "ns/builder"))), subject("system:serviceaccount:ns:builder"), true},
		{"serviceAccount In does not match a user named like it", terms(term(subjectReq(sa, in, "ns/builder"))), subject("ns/builder"), false},
		{"serviceAccount with missing name is not a service account", terms(term(subjectReq(sa, exists))), subject("system:serviceaccount:ns:"), false},
		{"serviceAccount with extra colon is not a service account", terms(term(subjectReq(sa, exists))), subject("system:serviceaccount:ns:a:b"), false},
		{"serviceAccount NotIn selects non service accounts", terms(term(subjectReq(sa, notIn, "ns/builder"))), subject("alice"), true},
		{"serviceAccount NotIn excludes the listed account", terms(term(subjectReq(sa, notIn, "ns/builder"))), subject("system:serviceaccount:ns:builder"), false},
		{"serviceAccount DoesNotExist", terms(term(subjectReq(sa, absent))), subject("alice"), true},
		{"requirements in a term are ANDed", terms(term(subjectReq(groups, in, "dev"), subjectReq(user, notIn, "alice"))), subject("alice", "dev"), false},
		{"requirements in a term all hold", terms(term(subjectReq(groups, in, "dev"), subjectReq(user, notIn, "alice"))), subject("bob", "dev"), true},
		{"terms are ORed", terms(term(subjectReq(user, in, "bob")), term(subjectReq(groups, in, "dev"))), subject("alice", "dev"), true},
		{"an empty term matches nothing", terms(term()), subject("alice", "dev"), false},
		{"no terms match nothing", nil, subject("alice", "dev"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			want := typeNoOp
			if tt.matches {
				want = typeDeny
			}
			for _, aware := range []bool{false, true} {
				d := newEngine(t, denyAllFor(tt.terms)).Authorize(context.Background(), tt.spec, aware)
				require.Equal(t, want, d.Type, "aware=%v %+v", aware, d)
			}
		})
	}
}

// TestResourceSelectorSemantics checks matchResources end to end through Authorize.
func TestResourceSelectorSemantics(t *testing.T) {
	t.Parallel()
	dev := []policiesv1alpha1.SubjectSelectorTerm{term(subjectReq(policiesv1alpha1.SubjectSelectorKeyGroups, policiesv1alpha1.SubjectSelectorOpIn, "dev"))}
	rr := func(mod func(r *policiesv1alpha1.AuthorizingResourceRule)) policiesv1alpha1.AuthorizingResourceRule {
		r := policiesv1alpha1.AuthorizingResourceRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}, Verbs: []string{"*"}}
		mod(&r)
		return r
	}
	res := func(group, resource, sub, ns, name, verb string) *authorizationv1.SubjectAccessReviewSpec {
		return &authorizationv1.SubjectAccessReviewSpec{User: "alice", Groups: []string{"dev"}, ResourceAttributes: &authorizationv1.ResourceAttributes{
			Group: group, Version: "v1", Resource: resource, Subresource: sub, Namespace: ns, Name: name, Verb: verb,
		}}
	}
	nonRes := func(path, verb string) *authorizationv1.SubjectAccessReviewSpec {
		return &authorizationv1.SubjectAccessReviewSpec{User: "alice", Groups: []string{"dev"}, NonResourceAttributes: &authorizationv1.NonResourceAttributes{Path: path, Verb: verb}}
	}
	noop := func(*policiesv1alpha1.AuthorizingResourceRule) {}
	tests := []struct {
		name     string
		rules    []policiesv1alpha1.AuthorizingResourceRule
		nonRules []policiesv1alpha1.AuthorizingNonResourceRule
		spec     *authorizationv1.SubjectAccessReviewSpec
		matches  bool
	}{
		{"exact resource", []policiesv1alpha1.AuthorizingResourceRule{rr(noop)}, nil, res("", "pods", "", "ns", "p", "get"), true},
		{"resource does not match its subresource", []policiesv1alpha1.AuthorizingResourceRule{rr(noop)}, nil, res("", "pods", "exec", "ns", "p", "create"), false},
		{"resource/* matches any subresource", []policiesv1alpha1.AuthorizingResourceRule{rr(func(r *policiesv1alpha1.AuthorizingResourceRule) { r.Resources = []string{"pods/*"} })}, nil, res("", "pods", "exec", "ns", "p", "create"), true},
		{"*/status matches status of any resource", []policiesv1alpha1.AuthorizingResourceRule{rr(func(r *policiesv1alpha1.AuthorizingResourceRule) { r.Resources = []string{"*/status"} })}, nil, res("", "deployments", "status", "ns", "d", "update"), true},
		{"*/status does not match the main resource", []policiesv1alpha1.AuthorizingResourceRule{rr(func(r *policiesv1alpha1.AuthorizingResourceRule) { r.Resources = []string{"*/status"} })}, nil, res("", "deployments", "", "ns", "d", "update"), false},
		{"api group must match", []policiesv1alpha1.AuthorizingResourceRule{rr(noop)}, nil, res("apps", "pods", "", "ns", "p", "get"), false},
		{"verb must match", []policiesv1alpha1.AuthorizingResourceRule{rr(func(r *policiesv1alpha1.AuthorizingResourceRule) { r.Verbs = []string{"create"} })}, nil, res("", "pods", "", "ns", "p", "delete"), false},
		{"namespaces excludes other namespaces", []policiesv1alpha1.AuthorizingResourceRule{rr(func(r *policiesv1alpha1.AuthorizingResourceRule) { r.Namespaces = []string{"team-a"} })}, nil, res("", "pods", "", "team-b", "p", "get"), false},
		{"namespaces excludes cluster-wide requests", []policiesv1alpha1.AuthorizingResourceRule{rr(func(r *policiesv1alpha1.AuthorizingResourceRule) { r.Namespaces = []string{"*"} })}, nil, res("", "pods", "", "", "", "list"), false},
		{"resourceNames excludes unnamed list", []policiesv1alpha1.AuthorizingResourceRule{rr(func(r *policiesv1alpha1.AuthorizingResourceRule) { r.ResourceNames = []string{"p"} })}, nil, res("", "pods", "", "ns", "", "list"), false},
		{"resourceNames matches the named object", []policiesv1alpha1.AuthorizingResourceRule{rr(func(r *policiesv1alpha1.AuthorizingResourceRule) { r.ResourceNames = []string{"p"} })}, nil, res("", "pods", "", "ns", "p", "get"), true},
		{"any resource rule may match", []policiesv1alpha1.AuthorizingResourceRule{
			rr(func(r *policiesv1alpha1.AuthorizingResourceRule) { r.Resources = []string{"secrets"} }), rr(noop),
		}, nil, res("", "pods", "", "ns", "p", "get"), true},
		{"resource rules never match non-resource requests", []policiesv1alpha1.AuthorizingResourceRule{rr(func(r *policiesv1alpha1.AuthorizingResourceRule) {
			r.APIGroups, r.APIVersions, r.Resources = []string{"*"}, []string{"*"}, []string{"*"}
		})}, nil, nonRes("/healthz", "get"), false},
		{"non-resource prefix", nil, []policiesv1alpha1.AuthorizingNonResourceRule{{Paths: []string{"/apis/*"}, Verbs: []string{"get"}}}, nonRes("/apis/apps", "get"), true},
		{"non-resource exact path is not a prefix", nil, []policiesv1alpha1.AuthorizingNonResourceRule{{Paths: []string{"/apis"}, Verbs: []string{"get"}}}, nonRes("/apis/apps", "get"), false},
		{"non-resource rules never match resource requests", nil, []policiesv1alpha1.AuthorizingNonResourceRule{{Paths: []string{"*"}, Verbs: []string{"*"}}}, res("", "pods", "", "ns", "p", "get"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := denyAllFor(dev)
			p.Spec.MatchResources = policiesv1alpha1.AuthorizingMatchResources{ResourceRules: tt.rules, NonResourceRules: tt.nonRules}
			want := typeNoOp
			if tt.matches {
				want = typeDeny
			}
			d := newEngine(t, p).Authorize(context.Background(), tt.spec, true)
			require.Equal(t, want, d.Type, "%+v", d)
		})
	}
}

// TestNoFirstMatch covers cases where the earlier first-match implementation
// (rules evaluated in order, first true rule wins, errors decided by policy
// failurePolicy) returned a different decision. Every rule of every applicable
// policy is evaluated and folded with Deny > NoOpinion > Allow, so rule order
// within a policy and policy order never change the outcome.
func TestNoFirstMatch(t *testing.T) {
	t.Parallel()
	const errExpr = "request.extra['missing'][0] == 'x'"
	tests := []struct {
		name  string
		rules []policiesv1alpha1.AuthorizingRule
		verb  string
		aware bool
		want  authorizationv1.ConditionsAwareDecisionType
	}{
		{"allow listed before deny", []policiesv1alpha1.AuthorizingRule{rule("a", allowE, "true"), rule("d", denyE, "true")}, "get", false, typeDeny},
		{"allow listed before noopinion", []policiesv1alpha1.AuthorizingRule{rule("a", allowE, "true"), rule("n", noOpE, "true")}, "get", false, typeNoOp},
		{"allow listed before erroring deny", []policiesv1alpha1.AuthorizingRule{rule("a", allowE, "true"), rule("d", denyE, errExpr)}, "get", false, typeDeny},
		{"allow listed before erroring noopinion", []policiesv1alpha1.AuthorizingRule{rule("a", allowE, "true"), rule("n", noOpE, errExpr)}, "get", false, typeNoOp},
		{"erroring allow listed before allow", []policiesv1alpha1.AuthorizingRule{rule("e", allowE, errExpr), rule("a", allowE, "true")}, "get", false, typeAllow},
		{"false deny listed before allow", []policiesv1alpha1.AuthorizingRule{rule("d", denyE, "false"), rule("a", allowE, "true")}, "get", false, typeAllow},
		{"satisfied allow before object-dependent deny stays conditional", []policiesv1alpha1.AuthorizingRule{rule("a", allowE, "true"), rule("d", denyE, "object.metadata.name == 'x'")}, "create", true, typeMap},
		{"satisfied allow before object-dependent deny denies unaware callers", []policiesv1alpha1.AuthorizingRule{rule("a", allowE, "true"), rule("d", denyE, "object.metadata.name == 'x'")}, "create", false, typeDeny},
		{"satisfied allow before object-dependent noopinion stays conditional", []policiesv1alpha1.AuthorizingRule{rule("a", allowE, "true"), rule("n", noOpE, "object.metadata.name == 'x'")}, "create", true, typeMap},
		{"satisfied allow before object-dependent noopinion is NoOpinion for unaware callers", []policiesv1alpha1.AuthorizingRule{rule("a", allowE, "true"), rule("n", noOpE, "object.metadata.name == 'x'")}, "create", false, typeNoOp},
		{"definite deny wins over an earlier residual allow", []policiesv1alpha1.AuthorizingRule{rule("r", allowE, "object.metadata.name == 'x'"), rule("d", denyE, "true")}, "create", true, typeDeny},
		{"definite noopinion wins over an earlier residual allow", []policiesv1alpha1.AuthorizingRule{rule("r", allowE, "object.metadata.name == 'x'"), rule("n", noOpE, "true")}, "create", true, typeNoOp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Every rotation of the rule list, and splitting the rules across
			// policies in either order, must give the same decision.
			n := len(tt.rules)
			for shift := range n {
				rotated := append(append([]policiesv1alpha1.AuthorizingRule{}, tt.rules[shift:]...), tt.rules[:shift]...)
				d := newEngine(t, policy("p", nil, rotated...)).Authorize(context.Background(), sar(tt.verb, "dev"), tt.aware)
				require.Equal(t, tt.want, d.Type, "rotation %d: %+v", shift, d)
				var split []*policiesv1alpha1.AuthorizingPolicy
				for i, r := range rotated {
					split = append(split, policy(fmt.Sprintf("p%d", i), nil, r))
				}
				d = newEngine(t, split...).Authorize(context.Background(), sar(tt.verb, "dev"), tt.aware)
				require.Equal(t, tt.want, d.Type, "split rotation %d: %+v", shift, d)
			}
		})
	}
}

// TestPolicyScopeIsolation checks that a Deny in a policy whose selectors do
// not match never affects the decision, and that an applicable Deny cannot be
// bypassed by an Allow in another policy.
func TestPolicyScopeIsolation(t *testing.T) {
	t.Parallel()
	allowDev := policy("allow-dev", nil, rule("a", allowE, "true"))
	denyAdmins := denyAllFor([]policiesv1alpha1.SubjectSelectorTerm{term(subjectReq(policiesv1alpha1.SubjectSelectorKeyGroups, policiesv1alpha1.SubjectSelectorOpIn, "admins"))})
	denyAdmins.Name = "deny-admins"
	e := newEngine(t, allowDev, denyAdmins)
	require.Equal(t, typeAllow, e.Authorize(context.Background(), sar("get", "dev"), false).Type)
	require.Equal(t, typeDeny, e.Authorize(context.Background(), sar("get", "dev", "admins"), false).Type)
	require.Equal(t, typeDeny, e.Authorize(context.Background(), sar("get", "admins"), false).Type)
	require.Equal(t, typeNoOp, e.Authorize(context.Background(), sar("get", "qa"), false).Type)
}

// TestDecisionReasonsNameAllContributingRules checks that the reason lists
// every rule that produced the winning effect, sorted, not only the first one.
func TestDecisionReasonsNameAllContributingRules(t *testing.T) {
	t.Parallel()
	e := newEngine(t,
		policy("b", nil, rule("d2", denyE, "true"), rule("a", allowE, "true")),
		policy("a", nil, rule("d1", denyE, "true")),
	)
	d := e.Authorize(context.Background(), sar("get", "dev"), false)
	require.Equal(t, typeDeny, d.Type)
	require.Equal(t, "denied by a/d1, b/d2", d.Deny.Reason)
}
