package engine

import (
	"context"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies/v1alpha1"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	"github.com/kyverno/kyverno/pkg/logging"
	"k8s.io/apiserver/pkg/cel/lazy"
)

var engineLogger = logging.WithName("apol-engine")

// AuthorizationDecision is the outcome of evaluating an AuthorizingPolicy against a request.
type AuthorizationDecision struct {
	// Effect is the final authorization effect: Allow, Deny, or NoOpinion.
	Effect policiesv1alpha1.AuthorizingRuleEffect
	// ConditionSet carries an unevaluated condition set for native conditional SAR responses.
	ConditionSet []Condition
	// ConditionResults carries any returned conditions (used when Effect == Conditional).
	ConditionResults []ConditionResult
	// Reason is optional human-readable text explaining the decision.
	Reason string
	// EvaluationError carries engine evaluation failure details when available.
	EvaluationError string
}

// Condition is an unevaluated condition entry in a conditional decision.
type Condition struct {
	ID          string
	Condition   string
	Effect      policiesv1alpha1.AuthorizingConditionEffect
	Description string
}

// ConditionResult maps a condition ID to its evaluated effect.
type ConditionResult struct {
	ID     string
	Effect policiesv1alpha1.AuthorizingConditionEffect
}

// Evaluator evaluates a compiled AuthorizingPolicy against a CEL activation.
type Evaluator interface {
	Evaluate(ctx context.Context, request map[string]any) (AuthorizationDecision, error)
}

// Engine evaluates a compiled AuthorizingPolicy.
type Engine interface {
	HandleSAR(ctx context.Context, request map[string]any) (AuthorizationDecision, error)
	HandleConditionsReview(ctx context.Context, request map[string]any) (AuthorizationDecision, error)
}

type engine struct {
	policy *apolcompiler.Policy
}

// NewEngine constructs an Engine for the given compiled policy.
func NewEngine(policy *apolcompiler.Policy) Engine {
	return &engine{policy: policy}
}

// HandleSAR evaluates authorization phase (SubjectAccessReview) for the compiled policy.
func (e *engine) HandleSAR(ctx context.Context, request map[string]any) (AuthorizationDecision, error) {
	return e.evaluate(ctx, request, false)
}

// HandleConditionsReview evaluates the conditions phase for the compiled policy.
func (e *engine) HandleConditionsReview(ctx context.Context, request map[string]any) (AuthorizationDecision, error) {
	return e.evaluate(ctx, request, true)
}

func (e *engine) evaluate(ctx context.Context, request map[string]any, conditionsPhase bool) (AuthorizationDecision, error) {
	pol := e.policy

	log := engineLogger.WithValues("policy", pol.Name, "conditionsPhase", conditionsPhase)
	log.V(3).Info("starting policy evaluation", "ruleCount", len(pol.Rules))

	activation := map[string]any{
		celcompiler.RequestKey: request,
	}
	appendVariables(ctx, activation, pol.Variables)

	log.V(4).Info("evaluating policy-level match conditions", "count", len(pol.MatchConditions))
	matched, err := evalMatchConditions(pol.MatchConditions, activation)
	if err != nil {
		log.Error(err, "policy-level match condition error")
		return noOpinionWithError(fmt.Sprintf("match condition error: %v", err), err), nil
	}
	if !matched {
		log.V(3).Info("policy-level match conditions not satisfied, skipping policy")
		return noOpinion("policy match conditions not satisfied"), nil
	}
	log.V(4).Info("policy-level match conditions satisfied")

	for _, rule := range pol.Rules {
		ruleLog := log.WithValues("rule", rule.Name, "effect", rule.Effect)
		ruleLog.V(4).Info("evaluating rule")

		ruleMatched, err := evalMatchConditions(rule.MatchConditions, activation)
		if err != nil {
			ruleLog.Error(err, "rule match condition error")
			return failDecision(pol.FailurePolicy, fmt.Sprintf("rule %q match condition error: %v", rule.Name, err), err), nil
		}
		if !ruleMatched {
			ruleLog.V(4).Info("rule match conditions not satisfied, skipping rule")
			continue
		}
		ruleLog.V(4).Info("rule match conditions satisfied")

		if rule.Expression != nil {
			ruleLog.V(4).Info("evaluating rule expression")
			out, _, err := rule.Expression.ContextEval(ctx, activation)
			if err != nil {
				ruleLog.Error(err, "rule expression evaluation error")
				return failDecision(pol.FailurePolicy, fmt.Sprintf("rule %q expression error: %v", rule.Name, err), err), nil
			}
			if !isTruthy(out) {
				ruleLog.V(4).Info("rule expression returned false, skipping rule")
				continue
			}
			ruleLog.V(3).Info("rule expression returned true")
		}

		ruleLog.V(2).Info("rule matched")

		if rule.Effect != policiesv1alpha1.AuthorizingRuleEffectConditional {
			decision := AuthorizationDecision{
				Effect: rule.Effect,
				Reason: fmt.Sprintf("matched rule %q", rule.Name),
			}
			ruleLog.V(2).Info("returning decisive decision", "effect", rule.Effect)
			return decision, nil
		}

		if !conditionsPhase {
			conditionSet := make([]Condition, 0, len(rule.Conditions))
			for _, cond := range rule.Conditions {
				conditionSet = append(conditionSet, Condition{
					ID:          cond.ID,
					Condition:   cond.Condition,
					Effect:      cond.Effect,
					Description: cond.Description,
				})
			}
			decision := AuthorizationDecision{
				Effect:       policiesv1alpha1.AuthorizingRuleEffectConditional,
				ConditionSet: conditionSet,
				Reason:       fmt.Sprintf("matched rule %q", rule.Name),
			}
			ruleLog.V(2).Info("returning conditional decision with unevaluated conditions", "conditionCount", len(conditionSet))
			return decision, nil
		}

		ruleLog.V(3).Info("evaluating conditions phase for conditional rule", "conditionCount", len(rule.Conditions))
		condResults, effect, reason, evalErr := evalConditions(ctx, rule.Conditions, activation, pol.FailurePolicy)
		if effect == policiesv1alpha1.AuthorizingRuleEffectAllow ||
			effect == policiesv1alpha1.AuthorizingRuleEffectDeny ||
			effect == policiesv1alpha1.AuthorizingRuleEffectNoOpinion {
			decision := AuthorizationDecision{
				Effect:           effect,
				ConditionResults: condResults,
				Reason:           reason,
				EvaluationError:  evalErr,
			}
			ruleLog.V(2).Info("returning conditional phase decision", "effect", effect)
			return decision, nil
		}

		return AuthorizationDecision{
			Effect:           policiesv1alpha1.AuthorizingRuleEffectConditional,
			ConditionResults: condResults,
			Reason:           fmt.Sprintf("matched rule %q", rule.Name),
		}, nil
	}

	return noOpinion("no matching rule"), nil
}

// evalMatchConditions returns true if all programs evaluate to true.
func evalMatchConditions(progs []cel.Program, activation map[string]any) (bool, error) {
	log := engineLogger.WithValues("conditionCount", len(progs))
	log.V(4).Info("evaluating match conditions")

	for i, prog := range progs {
		condLog := log.WithValues("index", i)
		condLog.V(4).Info("evaluating match condition")

		out, _, err := prog.Eval(activation)
		if err != nil {
			condLog.Error(err, "match condition evaluation error")
			return false, err
		}
		if !isTruthy(out) {
			condLog.V(4).Info("match condition returned false, conditions not satisfied")
			return false, nil
		}
		condLog.V(4).Info("match condition satisfied")
	}
	log.V(4).Info("all match conditions satisfied")
	return true, nil
}

// evalConditions evaluates all CompiledCondition programs and returns the concrete
// decision for the condition set according to KEP-5681 semantics.
func evalConditions(
	ctx context.Context,
	conds []apolcompiler.CompiledCondition,
	activation map[string]any,
	failurePolicy policiesv1alpha1.AuthorizingFailurePolicyType,
) ([]ConditionResult, policiesv1alpha1.AuthorizingRuleEffect, string, string) {
	log := engineLogger.WithValues("conditionCount", len(conds))
	log.V(4).Info("evaluating conditions")

	results := make([]ConditionResult, 0, len(conds))
	allowTrue := false
	for _, cond := range conds {
		condLog := log.WithValues("conditionID", cond.ID, "effect", cond.Effect)
		condLog.V(4).Info("evaluating condition")

		out, _, err := cond.Expression.ContextEval(ctx, activation)
		if err != nil {
			condLog.Error(err, "condition evaluation error")
			switch cond.Effect {
			case policiesv1alpha1.AuthorizingConditionEffectDeny:
				if failurePolicy == policiesv1alpha1.AuthorizingFailurePolicyNoOpinion {
					condLog.V(3).Info("deny condition failed with no-opinion failure policy")
					return results, policiesv1alpha1.AuthorizingRuleEffectNoOpinion, fmt.Sprintf("condition %q error (deny ignored): %v", cond.ID, err), err.Error()
				}
				condLog.V(3).Info("deny condition failed, applying fail-closed deny decision")
				return results, policiesv1alpha1.AuthorizingRuleEffectDeny, fmt.Sprintf("condition %q error (deny fail-closed): %v", cond.ID, err), err.Error()
			case policiesv1alpha1.AuthorizingConditionEffectNoOpinion:
				condLog.V(3).Info("no-opinion condition failed")
				return results, policiesv1alpha1.AuthorizingRuleEffectNoOpinion, fmt.Sprintf("condition %q error (no-opinion)", cond.ID), err.Error()
			default:
				// Allow-condition errors are ignored per KEP semantics.
				condLog.V(4).Info("allow condition evaluation failed, ignoring error per KEP semantics")
				continue
			}
		}

		condLog.V(4).Info("condition evaluation result", "result", isTruthy(out))

		if isTruthy(out) {
			results = append(results, ConditionResult{ID: cond.ID, Effect: cond.Effect})
			switch cond.Effect {
			case policiesv1alpha1.AuthorizingConditionEffectDeny:
				condLog.V(2).Info("condition denied the request")
				return results, policiesv1alpha1.AuthorizingRuleEffectDeny, fmt.Sprintf("condition %q denied", cond.ID), ""
			case policiesv1alpha1.AuthorizingConditionEffectNoOpinion:
				condLog.V(3).Info("condition returned no-opinion")
				return results, policiesv1alpha1.AuthorizingRuleEffectNoOpinion, fmt.Sprintf("condition %q returned no-opinion", cond.ID), ""
			case policiesv1alpha1.AuthorizingConditionEffectAllow:
				condLog.V(4).Info("allow condition matched")
				allowTrue = true
			}
		}
	}
	if allowTrue {
		log.V(2).Info("at least one allow condition matched")
		return results, policiesv1alpha1.AuthorizingRuleEffectAllow, "allow condition matched", ""
	}
	log.V(3).Info("no condition matched allow")
	return results, policiesv1alpha1.AuthorizingRuleEffectNoOpinion, "no condition matched allow", ""
}

// isTruthy returns true if the CEL value is exactly the true boolean.
func isTruthy(v ref.Val) bool {
	return v == types.True
}

func noOpinion(msg string) AuthorizationDecision {
	return AuthorizationDecision{
		Effect: policiesv1alpha1.AuthorizingRuleEffectNoOpinion,
		Reason: msg,
	}
}

func noOpinionWithError(msg string, evalErr error) AuthorizationDecision {
	decision := noOpinion(msg)
	if evalErr != nil {
		decision.EvaluationError = evalErr.Error()
	}
	return decision
}

func failDecision(policy policiesv1alpha1.AuthorizingFailurePolicyType, msg string, evalErr error) AuthorizationDecision {
	if policy == policiesv1alpha1.AuthorizingFailurePolicyDeny {
		decision := AuthorizationDecision{
			Effect: policiesv1alpha1.AuthorizingRuleEffectDeny,
			Reason: msg,
		}
		if evalErr != nil {
			decision.EvaluationError = evalErr.Error()
		}
		return decision
	}
	return noOpinionWithError(msg, evalErr)
}

func appendVariables(ctx context.Context, activation map[string]any, vars map[string]cel.Program) {
	lazyVars := lazy.NewMapValue(celcompiler.VariablesType)
	activation[celcompiler.VariablesKey] = lazyVars

	for name, variable := range vars {
		lazyVars.Append(name, func(*lazy.MapValue) ref.Val {
			out, _, err := variable.ContextEval(ctx, activation)
			if out != nil {
				return out
			}
			if err != nil {
				return types.WrapErr(err)
			}
			return nil
		})
	}
}
