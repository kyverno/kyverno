package compiler

import (
	"context"
	"fmt"

	cel "github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	admission "k8s.io/apiserver/pkg/admission"
	plugincel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type Policy struct {
	patchers              []Patcher
	matchConditions       []cel.Program
	targetMatchConditions []cel.Program
	targetExpression      cel.Program
	variables             map[string]cel.Program
	auditAnnotations      map[string]cel.Program
	exceptions            []compiler.Exception
	matchConstraints      *admissionregistrationv1.MatchResources

	// trace is set when the policy was compiled with tracing on. Only then are the traced
	// fields below populated (they hold the ASTs trace.Build needs); with tracing off they are
	// all zero values and evaluation takes exactly the same path as before. Mirrors vpol's
	// Policy.trace (pkg/cel/policies/vpol/compiler/policy.go).
	trace                       bool
	tracedMatchConditions       []compiler.TracedProgram
	tracedTargetMatchConditions []compiler.TracedProgram
	tracedVariables             map[string]compiler.TracedProgram
	// tracedMutations is index-aligned with patchers: tracedMutations[i] is the traced compile
	// output for the expression patchers[i] was built from. Patcher itself only exposes
	// Patch(...), not the underlying Program/AST, so this is kept alongside it rather than
	// inside it.
	tracedMutations []compiler.TracedProgram
}

func (p *Policy) MatchConstraints() *admissionregistrationv1.MatchResources {
	return p.matchConstraints
}

// Tracing reports whether the policy was compiled with tracing on, i.e. whether its evaluation
// results will carry a trace. Mirrors vpol's Policy.Tracing().
func (p *Policy) Tracing() bool {
	return p.trace
}

// match evaluates the conditions in order. record, when non-nil, is called after each condition
// is evaluated (including one that errors or comes back false) with that deciding evaluation's
// result, so a trace can be captured. Mirrors vpol's Policy.match
// (pkg/cel/policies/vpol/compiler/policy.go).
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
	if err := multierr.Combine(errs...); err != nil {
		return false, err
	}

	return true, nil
}

// appendVariables is the variable equivalent of match: record, when non-nil, is called for each
// variable the first time it is actually read (variables are lazy), so a trace can be captured.
func (p *Policy) appendVariables(
	ctx context.Context,
	data map[string]any,
	record func(name string, out ref.Val, err error),
) *lazy.MapValue {
	vars := lazy.NewMapValue(compiler.VariablesType)
	data[compiler.VariablesKey] = vars

	for name, variable := range p.variables {
		vars.Append(name, func(*lazy.MapValue) ref.Val {
			out, _, err := variable.ContextEval(ctx, data)
			if record != nil {
				record(name, out, err)
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

	return vars
}

// MatchesConditions is called once per candidate policy inside the
// mutate-existing matching loops (staticProvider.MatchesMutateExisting,
// reconciler.MatchesMutateExisting) as well as once per UpdateRequest from
// the background mutate-existing processor. requestMapFn lets the former
// (genuinely per-policy) callers hoist the request-map build to once per
// request via a memoized thunk (see compiler.BuildNormalizedRequestMap and
// sync.OnceValues at the call sites); the latter (genuinely single-shot,
// one policy per UpdateRequest) callers pass nil and prepareData builds
// locally, which costs nothing extra since there is only one call.
func (p *Policy) MatchesConditions(ctx context.Context, attr admission.Attributes, request *admissionv1.AdmissionRequest, namespace *corev1.Namespace, requestMapFn func() (map[string]any, error), contextProvider libs.Context) bool {
	data, err := prepareData(attr, request, namespace, requestMapFn)
	if err != nil {
		return false
	}

	p.appendVariables(ctx, data, nil)

	result, err := p.match(ctx, data, p.matchConditions, nil)
	if err != nil {
		return false
	}

	return result
}

// EvaluateTargetExpression has exactly one caller
// (background/mpol/processor.go's getTargetsFromExpression), invoked at
// most once per UpdateRequest - each UpdateRequest resolves to exactly one
// mpol (processor.Process -> GetPolicy), so this is genuinely single-shot,
// not a per-policy loop. There is no hoist benefit; building the request
// map locally is correct here.
func (p *Policy) EvaluateTargetExpression(ctx context.Context, attr admission.Attributes, request *admissionv1.AdmissionRequest, namespace *corev1.Namespace) (map[string]interface{}, error) {
	if p.targetExpression == nil {
		return nil, nil
	}
	data, err := prepareData(attr, request, namespace, nil)
	if err != nil {
		return nil, err
	}
	p.appendVariables(ctx, data, nil)
	out, _, err := p.targetExpression.ContextEval(ctx, data)
	if err != nil {
		return nil, err
	}
	return utils.ConvertToNative[map[string]interface{}](out)
}

func (p *Policy) Evaluate(
	ctx context.Context,
	attr admission.Attributes,
	namespace *corev1.Namespace,
	request admissionv1.AdmissionRequest,
	tcm TypeConverterManager,
	requestMapFn func() (map[string]any, error),
	contextProvider libs.Context,
) *EvaluationResult {
	return p.evaluate(ctx, attr, namespace, request, tcm, requestMapFn, false)
}

func (p *Policy) EvaluateTarget(
	ctx context.Context,
	attr admission.Attributes,
	namespace *corev1.Namespace,
	request admissionv1.AdmissionRequest,
	tcm TypeConverterManager,
	requestMapFn func() (map[string]any, error),
	contextProvider libs.Context,
) *EvaluationResult {
	return p.evaluate(ctx, attr, namespace, request, tcm, requestMapFn, true)
}

func (p *Policy) evaluate(
	ctx context.Context,
	attr admission.Attributes,
	namespace *corev1.Namespace,
	request admissionv1.AdmissionRequest,
	tcm TypeConverterManager,
	requestMapFn func() (map[string]any, error),
	target bool,
) *EvaluationResult {
	versionedAttributes := &admission.VersionedAttributes{
		Attributes:      attr,
		VersionedObject: attr.GetObject(),
		VersionedKind:   attr.GetKind(),
	}
	data, err := prepareData(attr, &request, namespace, requestMapFn)
	if err != nil {
		return &EvaluationResult{Error: err}
	}

	allowedImages := make([]string, 0)
	allowedValues := make([]string, 0)
	// check if the resource matches an exception
	if len(p.exceptions) > 0 {
		matchedExceptions := make([]*policiesv1beta1.PolicyException, 0)
		fullExemptionFound := false
		for _, polex := range p.exceptions {
			match, err := p.match(ctx, data, polex.MatchConditions, nil)
			if err != nil {
				if fullExemptionFound {
					// exception already granted; a broken later exception must not negate it
					continue
				}
				return &EvaluationResult{Error: err}
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
		// if there are matched exceptions and no allowed images, no need to evaluate the policy
		// as the resource is excluded from policy evaluation
		if fullExemptionFound {
			return &EvaluationResult{Exceptions: matchedExceptions}
		}
	}
	data[compiler.ExceptionsKey] = libs.Exception{
		AllowedImages: allowedImages,
		AllowedValues: allowedValues,
	}

	// matchTraces, variableTraces and mutationTraces are only ever appended to when p.trace is
	// true (the record closures are nil otherwise, and appending to mutationTraces below is
	// itself gated on p.trace) -- so decision() returns nil, and none of this costs anything,
	// when the policy was compiled without tracing. Mirrors vpol's evaluateWithData
	// (pkg/cel/policies/vpol/compiler/policy.go).
	var matchTraces, variableTraces []trace.NamedExpressionTrace
	var mutationTraces []trace.MutationTrace
	var recordMatch func(int, ref.Val, error)
	var recordVariable func(string, ref.Val, error)
	if p.trace {
		// out and err are always the deciding evaluation's; each tracking twin is only re-run to
		// collect node values for the trace (see compiler.TracedProgram)
		tracedMatchConditions := p.tracedMatchConditions
		if target {
			tracedMatchConditions = p.tracedTargetMatchConditions
		}
		recordMatch = func(i int, out ref.Val, err error) {
			if i >= len(tracedMatchConditions) {
				return
			}
			t := tracedMatchConditions[i]
			details := compiler.TraceDetails(ctx, t.Traced, data, err)
			matchTraces = append(matchTraces, trace.NamedExpressionTrace{
				Name:            t.Name,
				ExpressionTrace: buildExpressionTrace(t.AST, out, details, err),
			})
		}
		recordVariable = func(name string, out ref.Val, err error) {
			if t, ok := p.tracedVariables[name]; ok {
				// any variable the re-run reads comes from the same lazy map, so it matches
				details := compiler.TraceDetails(ctx, t.Traced, data, err)
				variableTraces = append(variableTraces, trace.NamedExpressionTrace{
					Name:            name,
					ExpressionTrace: buildExpressionTrace(t.AST, out, details, err),
				})
			}
		}
	}
	// verdict summarizes the whole policy's outcome. Unlike vpol there is no single expression
	// that decides it -- every mutation that runs contributes -- so its ExpressionTrace stays
	// empty; Render already handles a source-less verdict (see the "no validations" case).
	verdict := trace.VerdictTrace{Status: trace.VerdictPass}
	decision := func() *trace.Decision {
		if !p.trace {
			return nil
		}
		return &trace.Decision{Match: matchTraces, Variables: variableTraces, Mutations: mutationTraces, Verdict: verdict}
	}

	if target {
		p.appendVariables(ctx, data, recordVariable)
		match, err := p.match(ctx, data, p.targetMatchConditions, recordMatch)
		if err != nil {
			verdict.Status, verdict.Message = trace.VerdictError, err.Error()
			return &EvaluationResult{Error: err, Trace: decision()}
		}
		if !match {
			if !p.trace {
				return nil
			}
			// variables are bound before the match conditions and are lazy, so these are exactly
			// the ones a condition read, e.g. variables.isSystem in !variables.isSystem
			return &EvaluationResult{Skipped: true, Trace: &trace.Decision{
				Match:     matchTraces,
				Variables: variableTraces,
				Verdict:   trace.VerdictTrace{Status: trace.VerdictSkip, Message: skipMessage(matchTraces)},
			}}
		}
	} else {
		// variables are lazily bound and remain visible to trigger match conditions
		// for backward compatibility with existing policies
		p.appendVariables(ctx, data, recordVariable)
		match, err := p.match(ctx, data, p.matchConditions, recordMatch)
		if err != nil {
			verdict.Status, verdict.Message = trace.VerdictError, err.Error()
			return &EvaluationResult{Error: err, Trace: decision()}
		}
		if !match {
			if !p.trace {
				return nil
			}
			// variables are bound before the match conditions and are lazy, so these are exactly
			// the ones a condition read, e.g. variables.isSystem in !variables.isSystem
			return &EvaluationResult{Skipped: true, Trace: &trace.Decision{
				Match:     matchTraces,
				Variables: variableTraces,
				Verdict:   trace.VerdictTrace{Status: trace.VerdictSkip, Message: skipMessage(matchTraces)},
			}}
		}
	}

	o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())
	for i, patcher := range p.patchers {
		patchRequest := patch.Request{
			MatchedResource:     attr.GetResource(),
			VersionedAttributes: versionedAttributes,
			ObjectInterfaces:    o,
			OptionalVariables:   plugincel.OptionalVariableBindings{VersionedParams: nil, Authorizer: nil},
			Namespace:           namespace,
			TypeConverter:       tcm.GetTypeConverter(versionedAttributes.VersionedKind),
		}

		newVersionedObject, mutEval, err := patcher.Patch(ctx, data, patchRequest, celconfig.RuntimeCELCostBudget)
		if p.trace && i < len(p.tracedMutations) {
			var out ref.Val
			if mutEval != nil {
				out = mutEval.Result
			}
			t := p.tracedMutations[i]
			// re-run before data moves on to the patched object, so the twin sees what the
			// patcher saw
			details := compiler.TraceDetails(ctx, t.Traced, data, err)
			mt := trace.MutationTrace{Name: t.Name, ExpressionTrace: buildExpressionTrace(t.AST, out, details, err)}
			if err != nil {
				mt.Error = err.Error()
			}
			mutationTraces = append(mutationTraces, mt)
		}
		if err != nil {
			verdict.Status, verdict.Message = trace.VerdictError, err.Error()
			return &EvaluationResult{Error: err, Trace: decision()}
		}

		versionedAttributes.Dirty = true
		versionedAttributes.VersionedObject = newVersionedObject

		// the program data (the object) is supplied through the data map. we need to update
		// the map to get the patched object from the previous patch
		data[compiler.ObjectKey] = newVersionedObject.(*unstructured.Unstructured).Object
	}

	auditAnnotations, err := p.evaluateAuditAnnotations(ctx, data)
	if err != nil {
		verdict.Status, verdict.Message = trace.VerdictError, err.Error()
		return &EvaluationResult{Error: err, Trace: decision()}
	}

	return &EvaluationResult{
		PatchedResource:  versionedAttributes.VersionedObject.(*unstructured.Unstructured),
		AuditAnnotations: auditAnnotations,
		Trace:            decision(),
	}
}

// skipMessage names the match condition that excluded the resource. The last one recorded is
// the one that stopped evaluation, since match returns as soon as a condition is false. Mirrors
// vpol's skipMessage (pkg/cel/policies/vpol/compiler/policy.go).
func skipMessage(matchTraces []trace.NamedExpressionTrace) string {
	if len(matchTraces) == 0 {
		return "a match condition excluded this resource"
	}
	last := matchTraces[len(matchTraces)-1]
	if last.Name != "" {
		return fmt.Sprintf("match condition %q did not pass, so the policy was skipped", last.Name)
	}
	return "a match condition did not pass, so the policy was skipped"
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

// evaluateAuditAnnotations evaluates each auditAnnotation valueExpression and returns the
// resulting key/value pairs to be surfaced as report result properties. Empty results are omitted.
func (p *Policy) evaluateAuditAnnotations(ctx context.Context, data map[string]any) (map[string]string, error) {
	if len(p.auditAnnotations) == 0 {
		return nil, nil
	}
	annotations := make(map[string]string, len(p.auditAnnotations))
	for key, prog := range p.auditAnnotations {
		out, _, err := prog.ContextEval(ctx, data)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate auditAnnotation %q: %w", key, err)
		}
		if outcome, err := utils.ConvertToNative[string](out); err == nil && outcome != "" {
			annotations[key] = outcome
		} else if err != nil {
			return nil, fmt.Errorf("failed to convert auditAnnotation %q expression: %w", key, err)
		}
	}
	return annotations, nil
}

func (p *Policy) GetCompiledVariables() map[string]cel.Program {
	return p.variables
}
