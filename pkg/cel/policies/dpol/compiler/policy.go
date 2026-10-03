package compiler

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/kyverno/sdk/extensions/cel/libs/resource"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"go.uber.org/multierr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type Policy struct {
	deletionPropagationPolicy *metav1.DeletionPropagation
	schedule                  string
	conditions                []cel.Program
	variables                 map[string]cel.Program
	exceptions                []compiler.Exception
	// trace is whether this policy was compiled for decision tracing. tracedConditions is
	// index-aligned with conditions and, like tracedVariables, is empty when trace is off.
	trace            bool
	tracedConditions []compiler.TracedProgram
	tracedVariables  map[string]compiler.TracedProgram
}

// Tracing reports whether Evaluate fills EvaluationResult.Trace.
func (p *Policy) Tracing() bool {
	return p.trace
}

func (p *Policy) Evaluate(ctx context.Context, object unstructured.Unstructured, namespace runtime.Object, context libs.Context) (*EvaluationResult, error) {
	vars := lazy.NewMapValue(compiler.VariablesType)
	namespaceVal, err := utils.ObjectToResolveVal(namespace)
	if err != nil {
		return nil, err
	}
	dataNew := map[string]any{
		compiler.NamespaceObjectKey: namespaceVal,
		compiler.ObjectKey:          object.UnstructuredContent(),
		compiler.ResourceKey:        resource.Context{ContextInterface: context},
		compiler.VariablesKey:       vars,
	}
	// conditionTraces and variableTraces are only appended to when p.trace is set, and decision()
	// returns nil otherwise, so with tracing off none of this changes what Evaluate returns.
	var conditionTraces, variableTraces []trace.NamedExpressionTrace
	decision := func(verdict trace.VerdictTrace) *trace.Decision {
		if !p.trace {
			return nil
		}
		return &trace.Decision{Match: conditionTraces, Variables: variableTraces, Verdict: verdict}
	}
	for name, variable := range p.variables {
		vars.Append(name, func(*lazy.MapValue) ref.Val {
			out, details, err := variable.ContextEval(ctx, dataNew)
			if p.trace {
				if t, ok := p.tracedVariables[name]; ok {
					// variables are lazy, so this records them in the order they are first read
					variableTraces = append(variableTraces, trace.NamedExpressionTrace{
						Name:            name,
						ExpressionTrace: buildExpressionTrace(t.AST, out, details, err),
					})
				}
			}
			if out != nil {
				return out
			}
			if err != nil {
				return types.WrapErr(err)
			}
			return nil
		})
	}

	// check if the resource matches an exception
	if len(p.exceptions) > 0 {
		matchedExceptions := make([]*policiesv1beta1.PolicyException, 0)
		for _, polex := range p.exceptions {
			match, err := p.match(ctx, dataNew, polex.MatchConditions, nil)
			if err != nil {
				if !p.trace {
					return nil, err
				}
				return &EvaluationResult{Trace: decision(trace.VerdictTrace{Status: trace.VerdictError, Message: err.Error()})}, err
			}
			if match {
				matchedExceptions = append(matchedExceptions, polex.Exception)
			}
		}
		if len(matchedExceptions) > 0 {
			return &EvaluationResult{
				Exceptions: matchedExceptions,
				Trace:      decision(trace.VerdictTrace{Status: trace.VerdictSkip, Message: exemptMessage(matchedExceptions)}),
			}, nil
		}
	}
	var recordCondition func(int, ref.Val, *cel.EvalDetails, error)
	if p.trace {
		recordCondition = func(i int, out ref.Val, details *cel.EvalDetails, err error) {
			if i >= len(p.tracedConditions) {
				return
			}
			t := p.tracedConditions[i]
			name := t.Name
			if name == "" {
				name = fmt.Sprintf("conditions[%d]", i)
			}
			conditionTraces = append(conditionTraces, trace.NamedExpressionTrace{
				Name:            name,
				ExpressionTrace: buildExpressionTrace(t.AST, out, details, err),
			})
		}
	}
	match, err := p.match(ctx, dataNew, p.conditions, recordCondition)
	if err != nil {
		if !p.trace {
			return nil, err
		}
		// the error stays the second return value so callers handle it exactly as before; the
		// result only carries the conditions traced up to and including the failing ones
		return &EvaluationResult{Trace: decision(trace.VerdictTrace{Status: trace.VerdictError, Message: err.Error()})}, err
	}
	// PASS/FAIL line up with how results are reported (the CLI reports a dpol whose conditions
	// all hold as pass and one with a false condition as fail), so the trace explains that result
	verdict := trace.VerdictTrace{Status: trace.VerdictPass, Message: "conditions held: the resource would be deleted"}
	if !match {
		verdict = trace.VerdictTrace{Status: trace.VerdictFail, Message: keptMessage(conditionTraces)}
	}
	return &EvaluationResult{Result: match, Trace: decision(verdict)}, nil
}

// keptMessage explains why the resource is not deleted. match stops at the first condition that
// is false, so that is the last condition recorded.
func keptMessage(conditionTraces []trace.NamedExpressionTrace) string {
	if len(conditionTraces) == 0 {
		return "a condition was false: the resource is kept"
	}
	return fmt.Sprintf("condition %q is false: the resource is kept", conditionTraces[len(conditionTraces)-1].Name)
}

// exemptMessage names the exceptions that exempted the resource, as the rule message does.
func exemptMessage(exceptions []*policiesv1beta1.PolicyException) string {
	names := make([]string, 0, len(exceptions))
	for _, polex := range exceptions {
		names = append(names, polex.GetNamespace()+"/"+polex.GetName())
	}
	return "exempted by policy exception " + strings.Join(names, ", ") + ": the resource is kept"
}

// buildExpressionTrace turns one traced evaluation into an ExpressionTrace. The source text is
// read back from the retained AST. When the evaluation failed outright and produced no value,
// the error itself becomes the result so the trace shows why. Mirrors vpol's
// buildExpressionTrace (pkg/cel/policies/vpol/compiler/policy.go).
func buildExpressionTrace(ast *cel.Ast, out ref.Val, details *cel.EvalDetails, err error) trace.ExpressionTrace {
	if out == nil && err != nil {
		out = types.WrapErr(err)
	}
	source := ""
	if ast != nil {
		source = ast.Source().Content()
	}
	return trace.Build(source, ast, out, details)
}

// match evaluates conditions in order. record, when non-nil, is called for every condition that
// is evaluated, with its index into conditions; pass nil when there is nothing to trace.
func (p *Policy) match(ctx context.Context, data map[string]any, conditions []cel.Program, record func(int, ref.Val, *cel.EvalDetails, error)) (bool, error) {
	var errs []error
	for i, condition := range conditions {
		// evaluate the condition
		out, details, err := condition.ContextEval(ctx, data)
		if record != nil {
			record(i, out, details, err)
		}
		// check error
		if err != nil {
			errs = append(errs, err)
			continue
		}
		// try to convert to a bool
		result, err := utils.ConvertToNative[bool](out)
		// check error
		if err != nil {
			errs = append(errs, err)
			continue
		}
		// if condition is false, skip
		if !result {
			return false, nil
		}
	}
	if err := multierr.Combine(errs...); err == nil {
		return true, nil
	} else {
		return false, err
	}
}
