package engine

import (
	"context"
	"errors"
	"fmt"

	"cel.dev/cel-go/common/types"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	admissionv1 "k8s.io/api/admission/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/util/json"
	apiserverauthorizationv1 "k8s.io/apiserver/pkg/apis/authorization/v1"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// EvaluateConditions evaluates only the decision snapshot it is given. It
// never reads the current policies, so any replica answers the same way and a
// policy update or deletion cannot change an in-flight decision. Revocation of
// a cached decision is therefore bounded by the kube-apiserver decision cache
// TTL, not by Kyverno.
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
	failure := func(reason string, err error) authorizationv1.ConditionsAwareDecision {
		return fromParts(d.FailureDecision(), "failed closed: "+reason, err)
	}
	if admission == nil {
		return failure("missing admission request", errors.New("request.admissionRequest is required"))
	}
	if size := len(admission.Object.Raw) + len(admission.OldObject.Raw); size > e.limits.MaxObjectBytes {
		return failure("admission objects too large", fmt.Errorf("object and oldObject are %d bytes, more than the %d byte limit", size, e.limits.MaxObjectBytes))
	}
	release, err := e.review.acquire(ctx)
	if err != nil {
		return failure("no evaluation slot", err)
	}
	defer release()
	object, err := decodeRaw(admission.Object.Raw, e.limits.MaxValueDepth)
	if err != nil {
		return failure("cannot decode object", err)
	}
	oldObject, err := decodeRaw(admission.OldObject.Raw, e.limits.MaxValueDepth)
	if err != nil {
		return failure("cannot decode oldObject", err)
	}
	ev := &evaluation{limits: e.limits}
	activation := map[string]any{celcompiler.ObjectKey: object, celcompiler.OldObjectKey: oldObject}
	decisionOut, reason, err := d.ConditionsMap().Evaluate(ctx, nil, func(ctx context.Context, cond authorizer.Condition, _ authorizer.ConditionsData) (bool, error) {
		if cond.GetType() != ConditionType {
			return false, fmt.Errorf("unsupported condition type %q (supported: %s, environment %s)", cond.GetType(), ConditionType, ConditionsEnvVersion)
		}
		if err := ev.stopped(ctx); err != nil {
			return false, err
		}
		prog, err := e.review.program(cond.GetCondition())
		if err != nil {
			return false, err
		}
		out, details, err := prog.ContextEval(ctx, activation)
		ev.spend(details)
		if err != nil {
			return false, err
		}
		b, ok := out.(types.Bool)
		if !ok {
			return false, fmt.Errorf("condition returned non-boolean %v", out)
		}
		return bool(b), nil
	})
	logger.V(4).Info("evaluated conditions", "decision", decisionOut, "reason", reason, "error", err, "cost", ev.spent.Load())
	return fromParts(decisionOut, reason, err)
}

func decodeRaw(raw []byte, maxDepth int) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if depth(out, 0, maxDepth) > maxDepth {
		return nil, fmt.Errorf("value nesting exceeds %d levels", maxDepth)
	}
	return out, nil
}

// depth returns the nesting depth of a decoded JSON value, stopping early once it exceeds limit.
func depth(v any, cur, limit int) int {
	if cur > limit {
		return cur
	}
	max := cur
	switch t := v.(type) {
	case map[string]any:
		for _, c := range t {
			if d := depth(c, cur+1, limit); d > max {
				max = d
			}
		}
	case []any:
		for _, c := range t {
			if d := depth(c, cur+1, limit); d > max {
				max = d
			}
		}
	}
	return max
}
