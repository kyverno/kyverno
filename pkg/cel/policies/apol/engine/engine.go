// Package engine evaluates AuthorizingPolicy resources for the Kubernetes
// conditional authorization protocol (KEP-5681, Kubernetes v1.38 alpha).
//
// STUB NOTICE (kyverno/kyverno#18616): this is a minimal, correct evaluator
// written by the protocol work so the SubjectAccessReview and
// AuthorizationConditionsReview handlers can be tested end to end. The
// evaluator sub-issue (#18621) owns this package and is expected to replace or
// extend it. Known limitations:
//   - Variables cannot reference object or oldObject (compile error).
//   - A residual that cannot be rendered as a self-contained CEL expression
//     over object and oldObject (for example, because a known value cannot be
//     inlined as a literal) is treated as an evaluation error of its rule.
//   - Residual programs are compiled on every conditions review (no cache).
package engine

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	"github.com/kyverno/kyverno/pkg/logging"
	admissionv1 "k8s.io/api/admission/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/sets"
	apiserverauthorizationv1 "k8s.io/apiserver/pkg/apis/authorization/v1"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/cel/lazy"
)

var logger = logging.WithName("apol-engine")

const (
	// ConditionType marks conditions Kyverno authored and evaluates itself.
	// The kube-apiserver built-in type `authorizer.kubernetes.io/cel` is not
	// used because Kyverno residuals are evaluated with Kyverno's own CEL
	// environment (`object`, `oldObject`), and kube-apiserver v0.38.0-beta.0
	// has no built-in evaluator for that type yet.
	ConditionType = "policies.kyverno.io/cel"
	// MaxConditionsPerMap mirrors the upstream ConditionsMap limit (total over all effects).
	MaxConditionsPerMap = authorizer.MaxConditionsPerMap
	// definite effect condition IDs (constant `true` conditions).
	definiteNoOpinionID = "policies.kyverno.io/definite-noopinion"
	definiteAllowID     = "policies.kyverno.io/definite-allow"
)

// conditionalVerbs are the verbs that can carry a conditional decision.
var conditionalVerbs = sets.New("create", "update", "patch", "delete")

// Provider returns the current compiled policy set. Order of the result is not significant.
type Provider interface {
	Fetch(ctx context.Context) ([]CompiledPolicy, error)
}

// ProviderFunc adapts a function to a Provider.
type ProviderFunc func(context.Context) ([]CompiledPolicy, error)

func (f ProviderFunc) Fetch(ctx context.Context) ([]CompiledPolicy, error) { return f(ctx) }

// CompiledPolicy is one AuthorizingPolicy after compilation.
type CompiledPolicy interface {
	Policy() *policiesv1alpha1.AuthorizingPolicy
	// Applies evaluates matchSubjects and matchResources (no CEL).
	Applies(spec *authorizationv1.SubjectAccessReviewSpec) bool
	// CompileError is non-nil when the policy failed to compile; rules then follow the error semantics.
	CompileError() error
}

// Engine is the evaluator boundary used by the SAR and conditions-review handlers.
type Engine interface {
	// Authorize computes the authorization-time decision. conditionsAware is true only when
	// spec.authorizationOptions.handledDecisionTypes contains ConditionsMap (and Union). The
	// returned decision type is Allow, Deny, NoOpinion, or ConditionsMap (never Union).
	Authorize(ctx context.Context, spec *authorizationv1.SubjectAccessReviewSpec, conditionsAware bool) authorizationv1.ConditionsAwareDecision
	// EvaluateConditions evaluates a decision snapshot previously returned by Authorize against
	// admission data. It ignores current policy state. The returned type is Allow, Deny, or NoOpinion.
	EvaluateConditions(ctx context.Context, decision *authorizationv1.ConditionsAwareDecision, admission *admissionv1.AdmissionRequest) authorizationv1.ConditionsAwareDecision
}

// NewCompiledPolicy compiles a policy and wraps the result.
func NewCompiledPolicy(c apolcompiler.Compiler, policy *policiesv1alpha1.AuthorizingPolicy) CompiledPolicy {
	compiled, errs := c.Compile(policy)
	cp := &compiledPolicy{policy: policy, compiled: compiled}
	if len(errs) > 0 {
		cp.err = errs.ToAggregate()
	}
	return cp
}

type compiledPolicy struct {
	policy   *policiesv1alpha1.AuthorizingPolicy
	compiled *apolcompiler.Policy
	err      error
}

func (p *compiledPolicy) Policy() *policiesv1alpha1.AuthorizingPolicy { return p.policy }
func (p *compiledPolicy) CompileError() error                         { return p.err }

func (p *compiledPolicy) Applies(spec *authorizationv1.SubjectAccessReviewSpec) bool {
	if spec == nil {
		return false
	}
	if !p.policy.Spec.MatchesSubject(policiesv1alpha1.AuthorizationSubject{User: spec.User, Groups: spec.Groups}) {
		return false
	}
	switch {
	case spec.ResourceAttributes != nil:
		ra := spec.ResourceAttributes
		return p.policy.Spec.MatchResources.MatchesResource(policiesv1alpha1.AuthorizationResourceAttributes{
			APIGroup:    ra.Group,
			APIVersion:  ra.Version,
			Resource:    ra.Resource,
			Subresource: ra.Subresource,
			Namespace:   ra.Namespace,
			Name:        ra.Name,
			Verb:        ra.Verb,
		})
	case spec.NonResourceAttributes != nil:
		nra := spec.NonResourceAttributes
		return p.policy.Spec.MatchResources.MatchesNonResource(policiesv1alpha1.AuthorizationNonResourceAttributes{
			Path: nra.Path,
			Verb: nra.Verb,
		})
	}
	return false
}

type engine struct {
	provider      Provider
	conditionsEnv *cel.Env
	// celMu serializes type-checking and residual computation. cel.dev/cel-go
	// v0.33.2 has a data race in checker.(*Env).Catalog when a shared env
	// reports an undeclared reference concurrently (seen with go test -race).
	celMu sync.Mutex
}

func (e *engine) compileCondition(expr string) (*cel.Ast, error) {
	e.celMu.Lock()
	defer e.celMu.Unlock()
	ast, iss := e.conditionsEnv.Compile(expr)
	return ast, iss.Err()
}

// NewEngine returns an Engine backed by the given provider.
func NewEngine(provider Provider) (Engine, error) {
	env, err := apolcompiler.NewConditionsEnv()
	if err != nil {
		return nil, err
	}
	return &engine{provider: provider, conditionsEnv: env}, nil
}

// ruleResult accumulates per-effect outcomes across all applicable policies.
type ruleResult struct {
	definite  map[policiesv1alpha1.AuthorizingRuleEffect][]string
	residuals map[policiesv1alpha1.AuthorizingRuleEffect][]authorizationv1.Condition
	errs      []string
}

func (r *ruleResult) addError(effect policiesv1alpha1.AuthorizingRuleEffect, id string, err error) {
	r.errs = append(r.errs, fmt.Sprintf("%s: %v", id, err))
	// Deny and NoOpinion rules apply on error (true-or-error); Allow rules are dropped.
	if effect != policiesv1alpha1.AuthorizingRuleEffectAllow {
		r.definite[effect] = append(r.definite[effect], id)
	}
}

func (e *engine) Authorize(ctx context.Context, spec *authorizationv1.SubjectAccessReviewSpec, conditionsAware bool) authorizationv1.ConditionsAwareDecision {
	policies, err := e.provider.Fetch(ctx)
	if err != nil {
		// Without policies we cannot know whether a Deny applies; fail closed.
		return deny("failed closed: cannot fetch authorizing policies", err.Error())
	}
	request := RequestValue(spec)
	res := &ruleResult{
		definite:  map[policiesv1alpha1.AuthorizingRuleEffect][]string{},
		residuals: map[policiesv1alpha1.AuthorizingRuleEffect][]authorizationv1.Condition{},
	}
	for _, p := range policies {
		if !p.Applies(spec) {
			continue
		}
		pol := p.Policy()
		if cerr := p.CompileError(); cerr != nil {
			for _, rule := range pol.Spec.Rules {
				res.addError(rule.Effect, conditionID(pol.Name, rule.Name), fmt.Errorf("policy failed to compile: %w", cerr))
			}
			continue
		}
		cp, ok := p.(*compiledPolicy)
		if !ok || cp.compiled == nil {
			for _, rule := range pol.Spec.Rules {
				res.addError(rule.Effect, conditionID(pol.Name, rule.Name), errors.New("policy is not compiled"))
			}
			continue
		}
		e.evaluatePolicy(ctx, cp.compiled, request, res)
	}
	return decide(spec, conditionsAware, res)
}

func (e *engine) evaluatePolicy(ctx context.Context, pol *apolcompiler.Policy, request map[string]any, res *ruleResult) {
	vars := map[string]any{celcompiler.RequestKey: request}
	lazyVars := lazy.NewMapValue(celcompiler.VariablesType)
	vars[celcompiler.VariablesKey] = lazyVars
	activation, err := cel.PartialVars(vars,
		cel.AttributePattern(celcompiler.ObjectKey),
		cel.AttributePattern(celcompiler.OldObjectKey),
	)
	if err != nil {
		for _, rule := range pol.Rules {
			res.addError(rule.Effect, conditionID(pol.Policy.Name, rule.Name), err)
		}
		return
	}
	for _, v := range pol.Variables {
		lazyVars.Append(v.Name, func(*lazy.MapValue) ref.Val {
			out, _, err := v.Program.ContextEval(ctx, activation)
			if err != nil {
				return types.WrapErr(err)
			}
			return out
		})
	}
	for _, rule := range pol.Rules {
		id := conditionID(pol.Policy.Name, rule.Name)
		out, details, err := rule.Program.ContextEval(ctx, activation)
		switch {
		case err != nil:
			res.addError(rule.Effect, id, err)
		case types.IsUnknown(out):
			e.celMu.Lock()
			residual, err := pol.Env.ResidualAst(rule.AST, details)
			e.celMu.Unlock()
			if err != nil {
				res.addError(rule.Effect, id, fmt.Errorf("computing residual: %w", err))
				continue
			}
			expr, err := cel.AstToString(residual)
			if err != nil {
				res.addError(rule.Effect, id, fmt.Errorf("rendering residual: %w", err))
				continue
			}
			// The residual must be self-contained over object and oldObject.
			if _, err := e.compileCondition(expr); err != nil {
				res.addError(rule.Effect, id, fmt.Errorf("residual %q is not self-contained: %w", expr, err))
				continue
			}
			res.residuals[rule.Effect] = append(res.residuals[rule.Effect], authorizationv1.Condition{
				ID:          id,
				Condition:   expr,
				Type:        ConditionType,
				Description: rule.Description,
			})
		case out == types.True:
			res.definite[rule.Effect] = append(res.definite[rule.Effect], id)
		case out == types.False:
		default:
			res.addError(rule.Effect, id, fmt.Errorf("expression returned non-boolean %v", out))
		}
	}
}

// decide implements the cross-policy precedence Deny > NoOpinion > Allow from the contract.
func decide(spec *authorizationv1.SubjectAccessReviewSpec, conditionsAware bool, res *ruleResult) authorizationv1.ConditionsAwareDecision {
	slices.Sort(res.errs)
	evalErr := strings.Join(res.errs, "; ")
	definite := func(effect policiesv1alpha1.AuthorizingRuleEffect) []string {
		ids := slices.Clone(res.definite[effect])
		slices.Sort(ids)
		return ids
	}
	residual := func(effect policiesv1alpha1.AuthorizingRuleEffect) []authorizationv1.Condition {
		conds := slices.Clone(res.residuals[effect])
		slices.SortFunc(conds, func(a, b authorizationv1.Condition) int { return strings.Compare(a.ID, b.ID) })
		return conds
	}
	dDeny, dNoOp, dAllow := definite(policiesv1alpha1.AuthorizingRuleEffectDeny), definite(policiesv1alpha1.AuthorizingRuleEffectNoOpinion), definite(policiesv1alpha1.AuthorizingRuleEffectAllow)
	rDeny, rNoOp, rAllow := residual(policiesv1alpha1.AuthorizingRuleEffectDeny), residual(policiesv1alpha1.AuthorizingRuleEffectNoOpinion), residual(policiesv1alpha1.AuthorizingRuleEffectAllow)

	if len(dDeny) > 0 {
		return deny("denied by "+strings.Join(dDeny, ", "), evalErr)
	}
	if len(rDeny)+len(rNoOp)+len(rAllow) == 0 {
		switch {
		case len(dNoOp) > 0:
			return noOpinion("no opinion from "+strings.Join(dNoOp, ", "), evalErr)
		case len(dAllow) > 0:
			return allow("allowed by "+strings.Join(dAllow, ", "), evalErr)
		default:
			return noOpinion("", evalErr)
		}
	}
	// Residuals are present. Simplifications that do not need conditions:
	if len(rDeny) == 0 && len(dNoOp) > 0 {
		return noOpinion("no opinion from "+strings.Join(dNoOp, ", "), evalErr)
	}
	if len(rDeny) == 0 && len(rNoOp) == 0 && len(dAllow) > 0 {
		return allow("allowed by "+strings.Join(dAllow, ", "), evalErr)
	}
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
	// Build through the upstream constructor so IDs, sizes, and duplicates are validated exactly as kube-apiserver does.
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

func constantCondition(id string, rules []string) authorizationv1.Condition {
	desc := "definite effect of " + strings.Join(rules, ", ")
	if len(desc) > authorizer.MaxConditionDescriptionBytes {
		desc = desc[:authorizer.MaxConditionDescriptionBytes]
	}
	return authorizationv1.Condition{ID: id, Condition: "true", Type: ConditionType, Description: desc}
}

// EvaluateConditions evaluates only the decision snapshot it is given.
func (e *engine) EvaluateConditions(ctx context.Context, decision *authorizationv1.ConditionsAwareDecision, admission *admissionv1.AdmissionRequest) authorizationv1.ConditionsAwareDecision {
	if decision == nil {
		return deny("failed closed: missing decision", "request.decision is required")
	}
	var parseErr error
	d := apiserverauthorizationv1.ToAuthorizerConditionsAwareDecision(*decision, func(err error) authorizer.ConditionsAwareDecision {
		parseErr = err
		return authorizer.ConditionsAwareDecisionDeny("failed closed", err)
	})
	if parseErr != nil {
		return deny("failed closed: malformed decision", parseErr.Error())
	}
	if !d.IsConditionsMap() {
		// Kyverno only ever authors ConditionsMap decisions; anything else
		// (unconditional, Union, or a map that failed validation) fails closed.
		reason := "failed closed: decision is not a ConditionsMap authored by Kyverno"
		errMsg := fmt.Sprintf("unexpected decision type %q", decision.Type)
		if err := d.Error(); err != nil {
			errMsg = err.Error()
		}
		return fromParts(d.FailureDecision(), reason, errors.New(errMsg))
	}
	if admission == nil {
		return fromParts(d.FailureDecision(), "failed closed: missing admission request", errors.New("request.admissionRequest is required"))
	}
	object, err := decodeRaw(admission.Object.Raw)
	if err != nil {
		return fromParts(d.FailureDecision(), "failed closed: cannot decode object", err)
	}
	oldObject, err := decodeRaw(admission.OldObject.Raw)
	if err != nil {
		return fromParts(d.FailureDecision(), "failed closed: cannot decode oldObject", err)
	}
	activation := map[string]any{celcompiler.ObjectKey: object, celcompiler.OldObjectKey: oldObject}
	decisionOut, reason, err := d.ConditionsMap().Evaluate(ctx, nil, func(ctx context.Context, cond authorizer.Condition, _ authorizer.ConditionsData) (bool, error) {
		if cond.GetType() != ConditionType {
			return false, fmt.Errorf("unsupported condition type %q", cond.GetType())
		}
		ast, err := e.compileCondition(cond.GetCondition())
		if err != nil {
			return false, err
		}
		if !ast.OutputType().IsExactType(types.BoolType) && !ast.OutputType().IsExactType(types.DynType) {
			return false, fmt.Errorf("condition must evaluate to bool, got %v", ast.OutputType())
		}
		prog, err := e.conditionsEnv.Program(ast, apolcompiler.ProgramOptions(false)...)
		if err != nil {
			return false, err
		}
		out, _, err := prog.ContextEval(ctx, activation)
		if err != nil {
			return false, err
		}
		b, ok := out.(types.Bool)
		if !ok {
			return false, fmt.Errorf("condition returned non-boolean %v", out)
		}
		return bool(b), nil
	})
	logger.V(4).Info("evaluated conditions", "decision", decisionOut, "reason", reason, "error", err)
	return fromParts(decisionOut, reason, err)
}

func decodeRaw(raw []byte) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RequestValue converts a SubjectAccessReviewSpec into the CEL `request`
// value. All scalar fields are always present so expressions do not fail on
// omitted (empty) JSON fields.
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
		"user":                  spec.User,
		"groups":                groups,
		"uid":                   spec.UID,
		"extra":                 extra,
		"resourceAttributes":    nil,
		"nonResourceAttributes": nil,
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
		for key, sel := range map[string]any{"fieldSelector": ra.FieldSelector, "labelSelector": ra.LabelSelector} {
			attrs[key] = toJSONValue(sel)
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
