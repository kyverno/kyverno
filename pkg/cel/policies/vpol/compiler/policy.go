package compiler

import (
	"context"
	"fmt"
	"strconv"
	"strings"

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
	"k8s.io/client-go/tools/cache"
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

// Tracing reports whether the policy was compiled with tracing on, i.e. whether its evaluation
// results carry a Trace.
func (p *Policy) Tracing() bool {
	return p.trace
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
	var refused *RefusedException
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
			if !match {
				continue
			}
			// controls gate the bypass, not the resource: one that fails grants nothing, another
			// exception may still cover it, and if none does the policy runs as usual. Only the
			// first refusal is kept, deterministic because exceptions are compiled sorted.
			if refusal := p.evaluateExceptionValidations(ctx, dataNew, polex); refusal != nil {
				if refused == nil {
					refused = refusal
				}
				continue
			}
			matchedExceptions = append(matchedExceptions, polex.Exception)
			if len(polex.Exception.Spec.Images) == 0 && len(polex.Exception.Spec.AllowedValues) == 0 {
				fullExemptionFound = true
			} else if !fullExemptionFound {
				// partial scopes are irrelevant once a full exemption is granted
				allowedImages = append(allowedImages, polex.Exception.Spec.Images...)
				allowedValues = append(allowedValues, polex.Exception.Spec.AllowedValues...)
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
	// excludedBy is the condition that came out false, if any; erroredMatches are the ones that
	// errored or did not return a bool. With failurePolicy Ignore, errors alone skip the policy
	// after every condition has run, so the skip message needs these rather than the last trace.
	var excludedBy string
	var erroredMatches []string
	var recordMatch func(int, ref.Val, error)
	if p.trace {
		// out and err are the deciding evaluation's; the tracking twin is only re-run here to
		// collect node values for the trace (see compiler.TracedProgram)
		recordMatch = func(i int, out ref.Val, err error) {
			if i >= len(p.tracedMatchConditions) {
				return
			}
			t := p.tracedMatchConditions[i]
			details := compiler.TraceDetails(ctx, t.Traced, dataNew, err)
			matchTraces = append(matchTraces, trace.NamedExpressionTrace{
				Name:            t.Name,
				ExpressionTrace: buildExpressionTrace(t.AST, out, details, err),
			})
			if err != nil {
				erroredMatches = append(erroredMatches, t.Name)
			} else if result, err := utils.ConvertToNative[bool](out); err != nil {
				erroredMatches = append(erroredMatches, t.Name)
			} else if !result {
				excludedBy = t.Name
			}
		}
	}
	match, err := p.match(ctx, dataNew, p.matchConditions, recordMatch)
	if err != nil {
		if !p.trace {
			return nil, err
		}
		// the error stays the second return value so callers handle it exactly as before; the
		// result only carries the match traces recorded up to the failure, so --explain can show
		// which condition errored instead of a bare ERROR
		return &EvaluationResult{Trace: &trace.Decision{
			Match:   matchTraces,
			Verdict: trace.VerdictTrace{Status: trace.VerdictError, Message: err.Error()},
		}}, err
	}
	if !match {
		if !p.trace {
			return nil, nil
		}
		return &EvaluationResult{Skipped: true, Trace: &trace.Decision{
			Match:   matchTraces,
			Verdict: trace.VerdictTrace{Status: trace.VerdictSkip, Message: skipMessage(len(matchTraces), excludedBy, erroredMatches)},
		}}, nil
	}
	vars := lazy.NewMapValue(compiler.VariablesType)
	dataNew[compiler.VariablesKey] = vars
	for name, variable := range p.variables {
		vars.Append(name, func(*lazy.MapValue) ref.Val {
			out, _, err := variable.ContextEval(ctx, dataNew)
			if p.trace {
				if t, ok := p.tracedVariables[name]; ok {
					// out is still the deciding program's value; the twin only explains it. Any
					// variable the re-run reads comes from this same lazy map, so it matches.
					details := compiler.TraceDetails(ctx, t.Traced, dataNew, err)
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
	// the last one evaluated when everything passes. validationTraces keeps every validation that
	// ran, each with its own status. Both are only ever read when tracing is on.
	verdict := trace.VerdictTrace{Status: trace.VerdictPass}
	var validationTraces []trace.ValidationTrace
	ran := func(index int, status string) {
		if p.trace {
			validationTraces = append(validationTraces, trace.ValidationTrace{Index: index, Status: status, ExpressionTrace: verdict.ExpressionTrace})
		}
	}
	decision := func() *trace.Decision {
		if !p.trace {
			return nil
		}
		// evaluation stops at the first validation that does not pass; the rest are listed so
		// the reader sees them, but they are never evaluated just for the trace
		validations := validationTraces
		for i := len(validationTraces); i < len(p.validations); i++ {
			validations = append(validations, trace.ValidationTrace{
				Index:           i,
				Status:          trace.VerdictNotRun,
				ExpressionTrace: buildExpressionTrace(p.validations[i].AST, nil, nil, nil),
			})
		}
		return &trace.Decision{Match: matchTraces, Variables: variableTraces, Validations: validations, Verdict: verdict}
	}
	for index, validation := range p.validations {
		out, _, err := validation.Program.ContextEval(ctx, dataNew)
		if p.trace {
			details := compiler.TraceDetails(ctx, validation.Traced, dataNew, err)
			verdict = trace.VerdictTrace{
				Status:          trace.VerdictPass,
				ExpressionTrace: buildExpressionTrace(validation.AST, out, details, err),
			}
		}
		if err != nil {
			ran(index, trace.VerdictError)
			verdict.Status, verdict.Message = trace.VerdictError, err.Error()
			return &EvaluationResult{Error: err, Index: index, Identifier: validation.Identifier, Trace: decision()}, nil
		}
		if outcome, err := utils.ConvertToNative[bool](out); err == nil && !outcome {
			ran(index, trace.VerdictFail)
			message := p.resolveMessage(ctx, dataNew, validation, fmt.Sprintf("CEL expression validation failed at index %d", index))
			verdict.Status, verdict.Message = trace.VerdictFail, message
			auditAnnotations, err := p.evaluateAuditAnnotations(ctx, dataNew)
			if err != nil {
				verdict.Status, verdict.Message = trace.VerdictError, err.Error()
				return &EvaluationResult{Error: err, Index: index, Identifier: validation.Identifier, Trace: decision()}, nil
			}
			return &EvaluationResult{
				Result:           outcome,
				Message:          message,
				Index:            index,
				Identifier:       validation.Identifier,
				AuditAnnotations: auditAnnotations,
				RefusedException: refused,
				Trace:            decision(),
			}, nil
		} else if err != nil {
			ran(index, trace.VerdictError)
			verdict.Status, verdict.Message = trace.VerdictError, err.Error()
			return &EvaluationResult{Error: err, Index: index, Identifier: validation.Identifier, Trace: decision()}, nil
		}
		ran(index, trace.VerdictPass)
	}
	auditAnnotations, err := p.evaluateAuditAnnotations(ctx, dataNew)
	if err != nil {
		return nil, err
	}
	return &EvaluationResult{Result: true, AuditAnnotations: auditAnnotations, Trace: decision()}, nil
}

// skipMessage says why the match conditions skipped the policy: either one came out false (match
// stops there), or none did and some errored, which failurePolicy Ignore treats as a non-match.
func skipMessage(recorded int, excludedBy string, errored []string) string {
	switch {
	case recorded == 0:
		return "a match condition excluded this resource"
	case excludedBy != "":
		return fmt.Sprintf("match condition %q did not pass, so the policy was skipped", excludedBy)
	case len(errored) == 1:
		return fmt.Sprintf("match condition %q failed to evaluate and failurePolicy is Ignore, so the policy was skipped", errored[0])
	case len(errored) > 1:
		quoted := make([]string, 0, len(errored))
		for _, name := range errored {
			quoted = append(quoted, strconv.Quote(name))
		}
		return fmt.Sprintf("match conditions %s failed to evaluate and failurePolicy is Ignore, so the policy was skipped", strings.Join(quoted, ", "))
	}
	return "a match condition did not pass, so the policy was skipped"
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

// resolveMessage returns the message to report for a failed validation, preferring
// messageExpression over the static message and falling back when neither yields anything.
func (p *Policy) resolveMessage(
	ctx context.Context,
	data map[string]any,
	validation compiler.Validation,
	fallback string,
) string {
	message := validation.Message
	if validation.MessageExpression != nil {
		out, _, err := validation.MessageExpression.ContextEval(ctx, data)
		if err != nil {
			return fmt.Sprintf("failed to evaluate message expression: %s", err)
		}
		msg, err := utils.ConvertToNative[string](out)
		if err != nil {
			return fmt.Sprintf("failed to convert message expression to string: %s", err)
		}
		message = msg
	}
	if message == "" {
		return fallback
	}
	return message
}

// evaluateExceptionValidations evaluates the compensating controls of an exception already known
// to match. nil means every control passed and the bypass is granted; otherwise the refusal
// carries the failing control's message, or its error when a control could not be evaluated.
func (p *Policy) evaluateExceptionValidations(
	ctx context.Context,
	data map[string]any,
	polex compiler.Exception,
) *RefusedException {
	for index, validation := range polex.Validations {
		out, _, err := validation.Program.ContextEval(ctx, data)
		if err != nil {
			return &RefusedException{Exception: polex.Exception, Error: err}
		}
		outcome, err := utils.ConvertToNative[bool](out)
		if err != nil {
			return &RefusedException{Exception: polex.Exception, Error: err}
		}
		if !outcome {
			fallback := fmt.Sprintf(
				"compensating control at index %d failed for policy exception %s",
				index, cache.MetaObjectToName(polex.Exception),
			)
			return &RefusedException{
				Exception: polex.Exception,
				Message:   p.resolveMessage(ctx, data, validation, fallback),
			}
		}
	}
	return nil
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
	record func(index int, out ref.Val, err error),
) (bool, error) {
	var errs []error
	for i, matchCondition := range matchConditions {
		// evaluate the condition
		out, _, err := matchCondition.ContextEval(ctx, data)
		if record != nil {
			record(i, out, err)
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
