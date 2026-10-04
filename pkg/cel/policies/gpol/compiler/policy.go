package compiler

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/template"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/kyverno/sdk/extensions/cel/libs/generator"
	"github.com/kyverno/sdk/extensions/cel/libs/resource"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type Policy struct {
	namespace        string
	matchConditions  []cel.Program
	variables        map[string]cel.Program
	generations      []Generation
	auditAnnotations map[string]cel.Program
	exceptions       []compiler.Exception
	matchConstraints *admissionregistrationv1.MatchResources

	// trace is whether this policy was compiled for decision tracing. tracedMatchConditions is
	// index-aligned with matchConditions and, like tracedVariables, is empty when trace is off.
	trace                 bool
	tracedMatchConditions []compiler.TracedProgram
	tracedVariables       map[string]compiler.TracedProgram
}

// Generation is a compiled generate entry: either a CEL expression program or
// a YAML template. Exactly one of the two is set.
type Generation struct {
	expression cel.Program
	template   *template.Template
	// name identifies the entry in a trace, e.g. "generate[0] (expression)"; traced and ast are
	// the expression's explain-only tracking twin and AST, set only when the policy was compiled
	// with tracing on (see compiler.TracedProgram)
	name   string
	traced cel.Program
	ast    *cel.Ast
}

// dryRunGenerator stands in for the real generator when a tracking twin is re-run to explain a
// decision: it reports success and generates nothing. The twin goes down the same path as the
// deciding run, and generator.Apply returns the same value, but nothing is generated twice.
type dryRunGenerator struct{}

func (dryRunGenerator) GenerateResources(string, []map[string]any) error { return nil }

// Tracing reports whether Evaluate fills EvaluationResult.Trace.
func (p *Policy) Tracing() bool {
	return p.trace
}

func (p *Policy) MatchConstraints() *admissionregistrationv1.MatchResources {
	return p.matchConstraints
}

// EvaluationResult is returned by Evaluate and carries generated resources, matched exceptions and
// evaluated audit annotations (to be surfaced as report result properties).
type EvaluationResult struct {
	GeneratedResources []*unstructured.Unstructured
	Exceptions         []*policiesv1beta1.PolicyException
	AuditAnnotations   map[string]string
	// Trace is the decision trace for this evaluation. It is nil unless the policy was compiled
	// with tracing on. Only Match, Variables, Generations and Verdict are filled here; the
	// policy/resource header and Scope are left for the engine to fill in. When Evaluate fails it
	// can be returned alongside the error, carrying what was traced before the failure.
	Trace *trace.Decision
	// Skipped is set when a match condition excluded the resource. Without tracing that case
	// returns a nil result, and it still does; a non-nil skipped result is only returned when
	// tracing is on, so the match traces are not lost. Consumers must treat it exactly like nil.
	Skipped bool
}

func (p *Policy) evaluateAuditAnnotations(ctx context.Context, data map[string]any) (map[string]string, error) {
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

func (p *Policy) Evaluate(
	ctx context.Context,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	context libs.Context,
) (*EvaluationResult, error) {
	data, err := prepareData(attr, request, namespace, context)
	if err != nil {
		return nil, err
	}
	// Ensure generated resources are always cleared, even on early returns
	// (exception-only match, match failure, errors, etc.), to prevent state leaks.
	defer data.Context.ClearGeneratedResources()

	allowedImages := make([]string, 0)
	allowedValues := make([]string, 0)
	dataNew := map[string]any{
		compiler.NamespaceObjectKey: data.Namespace,
		compiler.ObjectKey:          data.Object,
		compiler.OldObjectKey:       data.OldObject,
		compiler.RequestKey:         data.Request,
		compiler.ResourceKey:        resource.Context{ContextInterface: data.Context},
	}
	// the *Traces slices are only appended to when p.trace is set, and decision() returns nil
	// otherwise, so with tracing off none of this changes what Evaluate returns
	var matchTraces, variableTraces []trace.NamedExpressionTrace
	var generationTraces []trace.GenerationTrace
	decision := func(verdict trace.VerdictTrace) *trace.Decision {
		if !p.trace {
			return nil
		}
		return &trace.Decision{Match: matchTraces, Variables: variableTraces, Generations: generationTraces, Verdict: verdict}
	}
	// failed returns what Evaluate returned on this error before tracing existed (nil, err),
	// plus, with tracing on, a result carrying what was traced up to the failure
	failed := func(err error) (*EvaluationResult, error) {
		if !p.trace {
			return nil, err
		}
		return &EvaluationResult{Trace: decision(trace.VerdictTrace{Status: trace.VerdictError, Message: err.Error()})}, err
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
				return failed(err)
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
			// SKIP is the default report result for an exemption; the engine adjusts the status
			// when the exception's reportResult says otherwise
			return &EvaluationResult{
				Exceptions: matchedExceptions,
				Trace:      decision(trace.VerdictTrace{Status: trace.VerdictSkip, Message: exemptMessage(matchedExceptions)}),
			}, nil
		}
	}
	dataNew[compiler.ExceptionsKey] = libs.Exception{
		AllowedImages: allowedImages,
		AllowedValues: allowedValues,
	}
	var recordMatch func(int, ref.Val, error)
	if p.trace {
		// out and err are the deciding evaluation's; the tracking twin is only re-run to collect
		// node values for the trace (see compiler.TracedProgram). The generator is not bound yet,
		// so match conditions cannot generate anything on either run.
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
		}
	}
	match, err := p.match(ctx, dataNew, p.matchConditions, recordMatch)
	if err != nil {
		return failed(err)
	}
	if !match {
		if !p.trace {
			return nil, nil
		}
		return &EvaluationResult{Skipped: true, Trace: decision(trace.VerdictTrace{Status: trace.VerdictSkip, Message: skipMessage(matchTraces)})}, nil
	}
	vars := lazy.NewMapValue(compiler.VariablesType)
	dataNew[compiler.VariablesKey] = vars
	dataNew[compiler.GeneratorKey] = generator.Context{ContextInterface: data.Context}
	// explainData is what every tracking twin below is re-run against: the same data, the same
	// lazy variables, but a generator that generates nothing, so explaining a variable or a
	// generate expression can never generate its resources a second time
	var explainData map[string]any
	if p.trace {
		explainData = maps.Clone(dataNew)
		explainData[compiler.GeneratorKey] = generator.Context{ContextInterface: dryRunGenerator{}}
	}
	for name, variable := range p.variables {
		vars.Append(name, func(*lazy.MapValue) ref.Val {
			out, _, err := variable.ContextEval(ctx, dataNew)
			if p.trace {
				if t, ok := p.tracedVariables[name]; ok {
					details := compiler.TraceDetails(ctx, t.Traced, explainData, err)
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
	for _, generation := range p.generations {
		// the context only ever appends generated resources, so what this entry produced is
		// everything past the count taken before it ran
		before := 0
		if p.trace {
			before = len(data.Context.GetGeneratedResources())
		}
		record := func(et trace.ExpressionTrace, err error) {
			if !p.trace {
				return
			}
			gt := trace.GenerationTrace{Name: generation.name, ExpressionTrace: et}
			if err != nil {
				gt.Error = err.Error()
			}
			// the list is only appended to while Evaluate runs (it is cleared by the defer above),
			// but guard the bound anyway: a trace must never be able to panic an evaluation
			if generated := data.Context.GetGeneratedResources(); before <= len(generated) {
				for _, r := range generated[before:] {
					gt.Generated = append(gt.Generated, describeResource(r))
				}
			}
			generationTraces = append(generationTraces, gt)
		}
		if generation.template != nil {
			if err := p.applyTemplate(ctx, generation.template, dataNew, data.Context); err != nil {
				record(trace.ExpressionTrace{}, err)
				return failed(err)
			}
			record(trace.ExpressionTrace{}, nil)
			continue
		}
		out, _, err := generation.expression.ContextEval(ctx, dataNew)
		if p.trace {
			details := compiler.TraceDetails(ctx, generation.traced, explainData, err)
			record(buildExpressionTrace(generation.ast, out, details, err), err)
		}
		if err != nil {
			return failed(err)
		}
	}
	auditAnnotations, err := p.evaluateAuditAnnotations(ctx, dataNew)
	if err != nil {
		return failed(err)
	}
	generatedResources := data.Context.GetGeneratedResources()
	return &EvaluationResult{
		GeneratedResources: generatedResources,
		AuditAnnotations:   auditAnnotations,
		Trace:              decision(trace.VerdictTrace{Status: trace.VerdictPass, Message: generatedMessage(len(generatedResources))}),
	}, nil
}

func generatedMessage(n int) string {
	if n == 0 {
		return "completed; no resources were generated"
	}
	if n == 1 {
		return "generated 1 resource"
	}
	return fmt.Sprintf("generated %d resources", n)
}

// describeResource renders a generated resource for a trace: "Kind namespace/name", or
// "Kind name" for a cluster-scoped one.
func describeResource(r *unstructured.Unstructured) string {
	if ns := r.GetNamespace(); ns != "" {
		return fmt.Sprintf("%s %s/%s", r.GetKind(), ns, r.GetName())
	}
	return fmt.Sprintf("%s %s", r.GetKind(), r.GetName())
}

// exemptMessage names the exceptions that exempted the resource, as the rule message does.
func exemptMessage(exceptions []*policiesv1beta1.PolicyException) string {
	names := make([]string, 0, len(exceptions))
	for _, polex := range exceptions {
		names = append(names, polex.GetNamespace()+"/"+polex.GetName())
	}
	return "exempted by policy exception " + strings.Join(names, ", ")
}

// skipMessage mirrors vpol's (pkg/cel/policies/vpol/compiler/policy.go): match stops at the first
// condition that is false, so that is the last one recorded.
func skipMessage(matchTraces []trace.NamedExpressionTrace) string {
	if len(matchTraces) == 0 {
		return "a match condition excluded this resource"
	}
	if last := matchTraces[len(matchTraces)-1]; last.Name != "" {
		return fmt.Sprintf("match condition %q did not pass, so the policy was skipped", last.Name)
	}
	return "a match condition did not pass, so the policy was skipped"
}

// buildExpressionTrace mirrors vpol's (pkg/cel/policies/vpol/compiler/policy.go).
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

// applyTemplate renders a generation template and feeds the resulting
// resources into the same generation runtime path used by generator.Apply.
// The target namespace of each resource is taken from its rendered
// metadata.namespace. For a namespaced policy, resources without a namespace
// default to the policy namespace and cross-namespace generation is denied,
// mirroring the namespaced generator.Apply semantics.
func (p *Policy) applyTemplate(ctx context.Context, tpl *template.Template, activation map[string]any, libsCtx libs.Context) error {
	resources, err := tpl.Render(ctx, activation)
	if err != nil {
		return fmt.Errorf("failed to render generation template: %w", err)
	}
	namespaces := make([]string, 0, 1)
	grouped := map[string][]map[string]any{}
	for _, resource := range resources {
		obj := unstructured.Unstructured{Object: resource}
		namespace := obj.GetNamespace()
		if p.namespace != "" {
			if namespace == "" {
				namespace = p.namespace
			} else if namespace != p.namespace {
				return fmt.Errorf("cross-namespace generation denied: a policy in namespace %q cannot generate resources into namespace %q", p.namespace, namespace)
			}
		}
		if _, ok := grouped[namespace]; !ok {
			namespaces = append(namespaces, namespace)
		}
		grouped[namespace] = append(grouped[namespace], resource)
	}
	for _, namespace := range namespaces {
		if err := libsCtx.GenerateResources(namespace, grouped[namespace]); err != nil {
			return fmt.Errorf("failed to generate resources: %w", err)
		}
	}
	return nil
}

// match evaluates matchConditions in order. record, when non-nil, is called for every condition
// that is evaluated, with its index; pass nil when there is nothing to trace.
func (p *Policy) match(
	ctx context.Context,
	data map[string]any,
	matchConditions []cel.Program,
	record func(int, ref.Val, error),
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
	} else {
		return false, err
	}
}
