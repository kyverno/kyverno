package engine

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/util/json"
	apiserverauthorizationv1 "k8s.io/apiserver/pkg/apis/authorization/v1"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// ruleResult accumulates per-effect outcomes across all applicable policies.
// It is a set: the order in which rules are added never changes the decision.
type ruleResult struct {
	definite  map[policiesv1alpha1.AuthorizingRuleEffect][]string
	residuals map[policiesv1alpha1.AuthorizingRuleEffect][]authorizationv1.Condition
	errs      []string
}

func newRuleResult() *ruleResult {
	return &ruleResult{
		definite:  map[policiesv1alpha1.AuthorizingRuleEffect][]string{},
		residuals: map[policiesv1alpha1.AuthorizingRuleEffect][]authorizationv1.Condition{},
	}
}

// addError records a rule evaluation error with the upstream effect-specific
// semantics: Deny and NoOpinion rules apply on error (true-or-error), so an
// erroring Deny can never be ignored; Allow rules are dropped.
func (r *ruleResult) addError(effect policiesv1alpha1.AuthorizingRuleEffect, id string, err error) {
	r.errs = append(r.errs, fmt.Sprintf("%s: %v", id, err))
	if effect != policiesv1alpha1.AuthorizingRuleEffectAllow {
		r.definite[effect] = append(r.definite[effect], id)
	}
}

func (r *ruleResult) addDefinite(effect policiesv1alpha1.AuthorizingRuleEffect, id string) {
	r.definite[effect] = append(r.definite[effect], id)
}

func (r *ruleResult) addResidual(effect policiesv1alpha1.AuthorizingRuleEffect, cond authorizationv1.Condition) {
	r.residuals[effect] = append(r.residuals[effect], cond)
}

// decide folds the rule results with the precedence Deny > NoOpinion > Allow.
// Only concrete (definite) outcomes are folded; an unresolved higher-precedence
// effect keeps the decision conditional, so a satisfied Allow never bypasses a
// residual Deny or NoOpinion.
func decide(spec *authorizationv1.SubjectAccessReviewSpec, conditionsAware bool, res *ruleResult) authorizationv1.ConditionsAwareDecision {
	errs := slices.Clone(res.errs)
	slices.Sort(errs)
	evalErr := truncate(strings.Join(errs, "; "), maxEvaluationErrorBytes)
	definite := func(effect policiesv1alpha1.AuthorizingRuleEffect) []string {
		ids := slices.Clone(res.definite[effect])
		slices.Sort(ids)
		return slices.Compact(ids)
	}
	residual := func(effect policiesv1alpha1.AuthorizingRuleEffect) []authorizationv1.Condition {
		conds := slices.Clone(res.residuals[effect])
		slices.SortFunc(conds, func(a, b authorizationv1.Condition) int { return strings.Compare(a.ID, b.ID) })
		return conds
	}
	dDeny, dNoOp, dAllow := definite(policiesv1alpha1.AuthorizingRuleEffectDeny), definite(policiesv1alpha1.AuthorizingRuleEffectNoOpinion), definite(policiesv1alpha1.AuthorizingRuleEffectAllow)
	rDeny, rNoOp, rAllow := residual(policiesv1alpha1.AuthorizingRuleEffectDeny), residual(policiesv1alpha1.AuthorizingRuleEffectNoOpinion), residual(policiesv1alpha1.AuthorizingRuleEffectAllow)

	if len(dDeny) > 0 {
		return deny("denied by "+joinIDs(dDeny), evalErr)
	}
	if len(rDeny)+len(rNoOp)+len(rAllow) == 0 {
		switch {
		case len(dNoOp) > 0:
			return noOpinion("no opinion from "+joinIDs(dNoOp), evalErr)
		case len(dAllow) > 0:
			return allow("allowed by "+joinIDs(dAllow), evalErr)
		default:
			return noOpinion("no applicable rule", evalErr)
		}
	}
	// Residuals are present. Simplifications that are safe without conditions:
	// with no residual Deny, a definite NoOpinion already wins over any Allow.
	if len(rDeny) == 0 && len(dNoOp) > 0 {
		return noOpinion("no opinion from "+joinIDs(dNoOp), evalErr)
	}
	// Only Allow residuals remain and an Allow is already satisfied.
	if len(rDeny) == 0 && len(rNoOp) == 0 && len(dAllow) > 0 {
		return allow("allowed by "+joinIDs(dAllow), evalErr)
	}
	// Nothing can allow and nothing can deny.
	if len(rDeny) == 0 && len(rAllow) == 0 && len(dAllow) == 0 {
		return noOpinion("only NoOpinion conditions remain", evalErr)
	}
	failClosed := func(reason string) authorizationv1.ConditionsAwareDecision {
		if len(rDeny) > 0 {
			return deny("failed closed: "+reason, evalErr)
		}
		return noOpinion("failed closed: "+reason, evalErr)
	}
	if !conditionsAware {
		return failClosed("caller does not handle conditional decisions")
	}
	if spec.ResourceAttributes == nil || !conditionalVerbs.Has(spec.ResourceAttributes.Verb) {
		return failClosed("request cannot carry a conditional decision")
	}
	noOp := rNoOp
	if len(dNoOp) > 0 {
		noOp = append(noOp, constantCondition(definiteNoOpinionID, dNoOp))
	}
	allowConds := rAllow
	if len(dAllow) > 0 {
		allowConds = append(allowConds, constantCondition(definiteAllowID, dAllow))
	}
	if len(rDeny)+len(noOp)+len(allowConds) > MaxConditionsPerMap {
		return failClosed(fmt.Sprintf("more than %d conditions", MaxConditionsPerMap))
	}
	// Build through the upstream constructor so IDs, sizes, and duplicates are
	// validated exactly as kube-apiserver does.
	d := apiserverauthorizationv1.ToAuthorizerConditionsAwareDecision(authorizationv1.ConditionsAwareDecision{
		Type: authorizationv1.ConditionsAwareDecisionTypeConditionsMap,
		ConditionsMap: &authorizationv1.ConditionsMap{
			DenyConditions:      rDeny,
			NoOpinionConditions: noOp,
			AllowConditions:     allowConds,
		},
	}, func(err error) authorizer.ConditionsAwareDecision {
		return authorizer.ConditionsAwareDecisionDeny("failed closed", err)
	})
	out := apiserverauthorizationv1.FromAuthorizerConditionsAwareDecision(d)
	if out.Type != authorizationv1.ConditionsAwareDecisionTypeConditionsMap && evalErr != "" {
		setEvaluationError(&out, evalErr)
	}
	return out
}

const maxEvaluationErrorBytes = 4096

// truncate shortens s to at most n bytes plus a "..." marker. It cuts on a
// rune boundary and drops invalid UTF-8 first: JSON encoding replaces a stray
// byte with U+FFFD (3 bytes), which would make the value on the wire longer
// than the byte limits kube-apiserver enforces.
func truncate(s string, n int) string {
	s = strings.ToValidUTF8(s, "")
	if len(s) <= n {
		return s
	}
	return s[:runeBoundary(s, n)] + "..."
}

// runeBoundary returns the largest k <= n such that s[:k] ends on a rune boundary.
func runeBoundary(s string, n int) int {
	if n <= 0 {
		return 0
	}
	if n >= len(s) {
		return len(s)
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return n
}

func joinIDs(ids []string) string {
	return truncate(strings.Join(ids, ", "), 1024)
}

func constantCondition(id string, rules []string) authorizationv1.Condition {
	desc := truncate("definite effect of "+strings.Join(rules, ", "), authorizer.MaxConditionDescriptionBytes-3)
	return authorizationv1.Condition{ID: id, Condition: "true", Type: ConditionType, Description: desc}
}

// RequestValue converts a SubjectAccessReviewSpec into the CEL `request`
// value. All scalar fields are always present so expressions do not fail on
// omitted (empty) JSON fields. Optional object fields (resourceAttributes,
// nonResourceAttributes, and the label and field selectors) are omitted when
// the SubjectAccessReview omits them, so `has()` reports their absence as it
// does in kube-apiserver matchConditions.
func RequestValue(spec *authorizationv1.SubjectAccessReviewSpec) map[string]any {
	groups := make([]any, 0, len(spec.Groups))
	for _, g := range spec.Groups {
		groups = append(groups, g)
	}
	extra := map[string]any{}
	for k, v := range spec.Extra {
		vals := make([]any, 0, len(v))
		for _, s := range v {
			vals = append(vals, s)
		}
		extra[k] = vals
	}
	out := map[string]any{
		"user":   spec.User,
		"groups": groups,
		"uid":    spec.UID,
		"extra":  extra,
	}
	if ra := spec.ResourceAttributes; ra != nil {
		attrs := map[string]any{
			"namespace":   ra.Namespace,
			"verb":        ra.Verb,
			"group":       ra.Group,
			"version":     ra.Version,
			"resource":    ra.Resource,
			"subresource": ra.Subresource,
			"name":        ra.Name,
		}
		if ra.FieldSelector != nil {
			attrs["fieldSelector"] = toJSONValue(ra.FieldSelector)
		}
		if ra.LabelSelector != nil {
			attrs["labelSelector"] = toJSONValue(ra.LabelSelector)
		}
		out["resourceAttributes"] = attrs
	}
	if nra := spec.NonResourceAttributes; nra != nil {
		out["nonResourceAttributes"] = map[string]any{"path": nra.Path, "verb": nra.Verb}
	}
	return out
}

func toJSONValue(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func conditionID(policy, rule string) string { return policy + "/" + rule }

func fromParts(d authorizer.Decision, reason string, err error) authorizationv1.ConditionsAwareDecision {
	return apiserverauthorizationv1.FromAuthorizerConditionsAwareDecision(authorizer.ConditionsAwareDecisionFromParts(d, reason, err))
}

func unconditional(reason, evalErr string) *authorizationv1.UnconditionalDecision {
	return &authorizationv1.UnconditionalDecision{Reason: reason, EvaluationError: evalErr}
}

func allow(reason, evalErr string) authorizationv1.ConditionsAwareDecision {
	return authorizationv1.ConditionsAwareDecision{Type: authorizationv1.ConditionsAwareDecisionTypeAllow, Allow: unconditional(reason, evalErr)}
}

func deny(reason, evalErr string) authorizationv1.ConditionsAwareDecision {
	return authorizationv1.ConditionsAwareDecision{Type: authorizationv1.ConditionsAwareDecisionTypeDeny, Deny: unconditional(reason, evalErr)}
}

func noOpinion(reason, evalErr string) authorizationv1.ConditionsAwareDecision {
	return authorizationv1.ConditionsAwareDecision{Type: authorizationv1.ConditionsAwareDecisionTypeNoOpinion, NoOpinion: unconditional(reason, evalErr)}
}

func setEvaluationError(d *authorizationv1.ConditionsAwareDecision, evalErr string) {
	for _, u := range []*authorizationv1.UnconditionalDecision{d.Allow, d.Deny, d.NoOpinion} {
		if u != nil && u.EvaluationError == "" {
			u.EvaluationError = evalErr
		}
	}
}
