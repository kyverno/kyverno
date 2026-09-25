package compiler

import (
	"context"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type Policy struct {
	mode             policiesv1beta1.EvaluationMode
	failurePolicy    admissionregistrationv1.FailurePolicyType
	matchConstraints *admissionregistrationv1.MatchResources
	matchConditions  []cel.Program
	variables        map[string]cel.Program
	validations      []compiler.Validation
	auditAnnotations map[string]cel.Program
	exceptions       []compiler.Exception

	// trace is set when the policy was compiled with tracing on. Only then are the traced
	// match conditions and variables populated (they hold the ASTs trace.Build needs), and
	// only then does evaluation attach a trace to its result. With tracing off all three are
	// zero values and evaluation takes exactly the same path as before.
	trace                 bool
	tracedMatchConditions []compiler.TracedProgram
	tracedVariables       map[string]compiler.TracedProgram
}

func (p *Policy) MatchConstraints() *admissionregistrationv1.MatchResources {
	return p.matchConstraints
}

func (p *Policy) Evaluate(
	ctx context.Context,
	json any,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	requestMapFn func() (map[string]any, error),
	context libs.Context,
) (*EvaluationResult, error) {
	switch p.mode {
	case policieskyvernoio.EvaluationModeJSON:
		return p.evaluateJson(ctx, json)
	default:
		return p.evaluateKubernetes(ctx, attr, request, namespace, requestMapFn, context)
	}
}

func (p *Policy) evaluateJson(
	ctx context.Context,
	json any,
) (*EvaluationResult, error) {
	data := evaluationData{
		Object:    json,
		Variables: lazy.NewMapValue(compiler.VariablesType),
	}
	return p.evaluateWithData(ctx, data)
}

func (p *Policy) evaluateKubernetes(
	ctx context.Context,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	requestMapFn func() (map[string]any, error),
	context libs.Context,
) (*EvaluationResult, error) {
	data, err := prepareK8sData(attr, request, namespace, requestMapFn, context)
	if err != nil {
		return nil, err
	}
	return p.evaluateWithData(ctx, data)
}

func (p *Policy) evaluateWithData(
	ctx context.Context,
	data evaluationData,
) (*EvaluationResult, error) {
	allowedImages := make([]string, 0)
	allowedValues := make([]string, 0)
	dataNew := map[string]any{
		compiler.NamespaceObjectKey: data.Namespace,
		compiler.ObjectKey:          data.Object,
		compiler.OldObjectKey:       data.OldObject,
		compiler.RequestKey:         data.Request,
	}
	// check if the resource matches an exception
	if len(p.exceptions) > 0 {
		matchedExceptions := make([]*policiesv1beta1.PolicyException, 0)
		fullExemptionFound := false
		for _, polex := range p.exceptions {
			match, err := p.match(ctx, dataNew, polex.MatchConditions, nil)
			if err != nil {
				if fullExemptionFound {
					// exception already granted; a broken later exception must not negate it
					continue
				}
				return nil, err
			}
			if match {
				matchedExceptions = append(matchedExceptions, polex.Exception)
				if len(polex.Exception.Spec.Images) == 0 && len(polex.Exception.Spec.AllowedValues) == 0 {
					fullExemptionFound = true
				} else if !fullExemptionFound {
					// partial scopes are irrelevant once a full exemption is granted
					allowedImages = append(allowedImages, polex.Exception.Spec.Images...)
					allowedValues = append(allowedValues, polex.Exception.Spec.AllowedValues...)
				}
			}
		}
		if fullExemptionFound {
			return &EvaluationResult{Exceptions: matchedExceptions}, nil
		}
	}
	dataNew[compiler.ExceptionsKey] = libs.Exception{
		AllowedImages: allowedImages,
		AllowedValues: allowedValues,
	}
	var matchTraces, variableTraces []trace.NamedExpressionTrace
	var recordMatch func(int, ref.Val, *cel.EvalDetails, error)
	if p.trace {
		recordMatch = func(i int, out ref.Val, details *cel.EvalDetails, err error) {
			if i >= len(p.tracedMatchConditions) {
				return
			}
			t := p.tracedMatchConditions[i]
			matchTraces = append(matchTraces, trace.NamedExpressionTrace{
				Name:            t.Name,
				ExpressionTrace: buildExpressionTrace(t.AST, out, details, err),
			})
		}
	}
	match, err := p.match(ctx, dataNew, p.matchConditions, recordMatch)
	if err != nil {
		return nil, err
	}
	if !match {
		return nil, nil
	}
	vars := lazy.NewMapValue(compiler.VariablesType)
	dataNew[compiler.VariablesKey] = vars
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
	// verdict tracks the validation that decides the outcome: the one that failed or errored, or
	// the last one evaluated when everything passes. It is only ever read when tracing is on.
	verdict := trace.VerdictTrace{Status: trace.VerdictPass}
	decision := func() *trace.Decision {
		if !p.trace {
			return nil
		}
		return &trace.Decision{Match: matchTraces, Variables: variableTraces, Verdict: verdict}
	}
	for index, validation := range p.validations {
		out, details, err := validation.Program.ContextEval(ctx, dataNew)
		if p.trace {
			verdict = trace.VerdictTrace{
				Status:          trace.VerdictPass,
				ExpressionTrace: buildExpressionTrace(validation.AST, out, details, err),
			}
		}
		if err != nil {
			verdict.Status, verdict.Message = trace.VerdictError, err.Error()
			return &EvaluationResult{Error: err, Index: index, Trace: decision()}, nil
		}
		if outcome, err := utils.ConvertToNative[bool](out); err == nil && !outcome {
			message := validation.Message
			if validation.MessageExpression != nil {
				if out, _, err := validation.MessageExpression.ContextEval(ctx, dataNew); err != nil {
					message = fmt.Sprintf("failed to evaluate message expression: %s", err)
				} else if msg, err := utils.ConvertToNative[string](out); err != nil {
					message = fmt.Sprintf("failed to convert message expression to string: %s", err)
				} else {
					message = msg
				}
			}
			// Add default message if empty
			if message == "" {
				message = fmt.Sprintf("CEL expression validation failed at index %d", index)
			}
			verdict.Status, verdict.Message = trace.VerdictFail, message
			auditAnnotations, err := p.evaluateAuditAnnotations(ctx, dataNew)
			if err != nil {
				verdict.Status, verdict.Message = trace.VerdictError, err.Error()
				return &EvaluationResult{Error: err, Index: index, Trace: decision()}, nil
			}
			return &EvaluationResult{
				Result:           outcome,
				Message:          message,
				Index:            index,
				AuditAnnotations: auditAnnotations,
				Trace:            decision(),
			}, nil
		} else if err != nil {
			verdict.Status, verdict.Message = trace.VerdictError, err.Error()
			return &EvaluationResult{Error: err, Index: index, Trace: decision()}, nil
		}
	}
	auditAnnotations, err := p.evaluateAuditAnnotations(ctx, dataNew)
	if err != nil {
		return nil, err
	}
	return &EvaluationResult{Result: true, AuditAnnotations: auditAnnotations, Trace: decision()}, nil
}

// buildExpressionTrace turns one traced evaluation into an ExpressionTrace. The source text is
// read back from the retained AST. When the evaluation failed outright and produced no value,
// the error itself becomes the result so the trace shows why.
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

func (p *Policy) evaluateAuditAnnotations(ctx context.Context, data map[string]any) (map[string]string, error) {
	auditAnnotations := make(map[string]string, len(p.auditAnnotations))
	for key, annotation := range p.auditAnnotations {
		out, _, err := annotation.ContextEval(ctx, data)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate auditAnnotation '%s': %w", key, err)
		}
		if outcome, err := utils.ConvertToNative[string](out); err == nil && outcome != "" {
			auditAnnotations[key] = outcome
		} else if err != nil {
			return nil, fmt.Errorf("failed to convert auditAnnotation '%s' expression: %w", key, err)
		}
	}
	return auditAnnotations, nil
}

// match evaluates the conditions in order. record, when non-nil, is called after each condition
// is evaluated (including one that errors or comes back false) so a trace can be captured.
func (p *Policy) match(
	ctx context.Context,
	data map[string]any,
	matchConditions []cel.Program,
	record func(index int, out ref.Val, details *cel.EvalDetails, err error),
) (bool, error) {
	var errs []error
	for i, matchCondition := range matchConditions {
		// evaluate the condition
		out, details, err := matchCondition.ContextEval(ctx, data)
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
	} else if p.failurePolicy == admissionregistrationv1.Ignore {
		return false, nil
	} else {
		return false, err
	}
}
