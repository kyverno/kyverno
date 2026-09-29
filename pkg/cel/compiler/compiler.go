package compiler

import (
	"context"
	"errors"
	"fmt"

	"github.com/gobwas/glob"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/interpreter"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// TracedProgram holds the two programs built for one traced expression, and the retained AST
// that trace.Build needs to map traced node ids back to source text.
//
// Program is built exactly as it is without tracing and is the only one whose result decides
// anything. Traced is the same expression built with cel.OptTrackState; it is only ever re-run,
// through TraceDetails, to collect per-node values for an explanation. The two must not be merged:
// in cel-go v0.31 any state tracking switches off enforcement of the per-call cost limit, so a
// tracking program can finish (and pass) an expression the normal program stops with an error.
type TracedProgram struct {
	Name    string
	Program cel.Program
	Traced  cel.Program
	AST     *cel.Ast
}

// tracingProgram builds the explain-only twin of a program: same AST, with state tracking on.
func tracingProgram(env *cel.Env, ast *cel.Ast) (cel.Program, error) {
	return env.Program(ast, cel.EvalOptions(cel.OptTrackState))
}

// TraceDetails re-runs traced on the same activation to collect the per-node values for a trace.
// Call it only after the normal program has decided, passing the error that run returned: the
// re-run never decides anything, and it is skipped when the decision was cancelled (the cost
// limit, or a cancelled context), since the tracking program does not enforce the cost limit and
// would run that expression to completion. An expression that finished under the limit costs the
// same the second time, so the re-run is bounded. Returns nil when there is nothing to trace,
// which trace.Build handles by showing only the result.
func TraceDetails(ctx context.Context, traced cel.Program, activation any, decidedErr error) *cel.EvalDetails {
	if traced == nil {
		return nil
	}
	var cancelled interpreter.EvalCancelledError
	if errors.As(decidedErr, &cancelled) {
		return nil
	}
	_, details, _ := traced.ContextEval(ctx, activation)
	return details
}

func CompileMatchCondition(path *field.Path, env *cel.Env, matchCondition admissionregistrationv1.MatchCondition) (cel.Program, field.ErrorList) {
	traced, errs := compileMatchCondition(path, env, matchCondition, false)
	return traced.Program, errs
}

// compileMatchCondition always builds the normal program; with trace it also builds the
// explain-only tracking twin and keeps the AST (see TracedProgram).
func compileMatchCondition(path *field.Path, env *cel.Env, matchCondition admissionregistrationv1.MatchCondition, trace bool) (TracedProgram, field.ErrorList) {
	var allErrs field.ErrorList
	{
		path := path.Child("expression")
		ast, issues := env.Compile(matchCondition.Expression)
		if err := issues.Err(); err != nil {
			return TracedProgram{}, append(allErrs, field.Invalid(path, matchCondition.Expression, err.Error()))
		}
		if !ast.OutputType().IsExactType(types.BoolType) {
			msg := fmt.Sprintf("output is expected to be of type %s", types.BoolType.TypeName())
			return TracedProgram{}, append(allErrs, field.Invalid(path, matchCondition.Expression, msg))
		}
		prog, err := env.Program(ast)
		if err != nil {
			return TracedProgram{}, append(allErrs, field.Invalid(path, matchCondition.Expression, err.Error()))
		}
		compiled := TracedProgram{Name: matchCondition.Name, Program: prog}
		if trace {
			if compiled.Traced, err = tracingProgram(env, ast); err != nil {
				return TracedProgram{}, append(allErrs, field.Invalid(path, matchCondition.Expression, err.Error()))
			}
			compiled.AST = ast
		}
		return compiled, allErrs
	}
}

// CompileMatchConditionsWithTrace is the tracing-aware entry point for match conditions. With
// trace false it is exactly CompileMatchConditions and the traced result is nil. With trace true
// the returned programs are still the normal ones that decide; the traced result, index-aligned
// with them, adds each condition's explain-only tracking program and AST (see TracedProgram).
// Policy kinds that support tracing should call this with their own trace flag; callers that
// don't trace keep using CompileMatchConditions.
func CompileMatchConditionsWithTrace(path *field.Path, env *cel.Env, trace bool, matchConditions ...admissionregistrationv1.MatchCondition) ([]cel.Program, []TracedProgram, field.ErrorList) {
	if !trace {
		programs, errs := CompileMatchConditions(path, env, matchConditions...)
		return programs, nil, errs
	}
	traced, errs := compileMatchConditionsTraced(path, env, matchConditions...)
	programs := make([]cel.Program, 0, len(traced))
	for _, t := range traced {
		programs = append(programs, t.Program)
	}
	return programs, traced, errs
}

func compileMatchConditionsTraced(path *field.Path, env *cel.Env, matchConditions ...admissionregistrationv1.MatchCondition) (result []TracedProgram, allErrs field.ErrorList) {
	if len(matchConditions) == 0 {
		return nil, nil
	}
	for i, matchCondition := range matchConditions {
		compiled, errs := compileMatchCondition(path.Index(i), env, matchCondition, true)
		allErrs = append(allErrs, errs...)
		if compiled.Program != nil {
			result = append(result, compiled)
		}
	}
	return result, allErrs
}

func CompileMatchConditions(path *field.Path, env *cel.Env, matchConditions ...admissionregistrationv1.MatchCondition) (result []cel.Program, allErrs field.ErrorList) {
	if len(matchConditions) == 0 {
		return nil, nil
	}
	for i, matchCondition := range matchConditions {
		prog, errs := CompileMatchCondition(path.Index(i), env, matchCondition)
		allErrs = append(allErrs, errs...)
		if prog != nil {
			result = append(result, prog)
		}
	}
	return result, allErrs
}

func CompileVariable(path *field.Path, env *cel.Env, VariablesProvider *VariablesProvider, variable admissionregistrationv1.Variable) (cel.Program, field.ErrorList) {
	traced, errs := compileVariable(path, env, VariablesProvider, variable, false)
	return traced.Program, errs
}

// compileVariable always builds the normal program; with trace it also builds the explain-only
// tracking twin and keeps the AST (see TracedProgram).
func compileVariable(path *field.Path, env *cel.Env, VariablesProvider *VariablesProvider, variable admissionregistrationv1.Variable, trace bool) (TracedProgram, field.ErrorList) {
	var allErrs field.ErrorList
	{
		path := path.Child("expression")
		ast, issues := env.Compile(variable.Expression)
		if err := issues.Err(); err != nil {
			return TracedProgram{}, append(allErrs, field.Invalid(path, variable.Expression, err.Error()))
		}
		VariablesProvider.RegisterField(variable.Name, ast.OutputType())
		prog, err := env.Program(ast)
		if err != nil {
			return TracedProgram{}, append(allErrs, field.Invalid(path, variable.Expression, err.Error()))
		}
		compiled := TracedProgram{Name: variable.Name, Program: prog}
		if trace {
			if compiled.Traced, err = tracingProgram(env, ast); err != nil {
				return TracedProgram{}, append(allErrs, field.Invalid(path, variable.Expression, err.Error()))
			}
			compiled.AST = ast
		}
		return compiled, allErrs
	}
}

// CompileVariablesWithTrace is the variable equivalent of CompileMatchConditionsWithTrace: with
// trace false it is exactly CompileVariables; with trace true the returned programs are still the
// normal ones, and the traced result adds each variable's tracking program and AST, keyed by name.
func CompileVariablesWithTrace(path *field.Path, env *cel.Env, VariablesProvider *VariablesProvider, trace bool, variables ...admissionregistrationv1.Variable) (map[string]cel.Program, map[string]TracedProgram, field.ErrorList) {
	if !trace {
		programs, errs := CompileVariables(path, env, VariablesProvider, variables...)
		return programs, nil, errs
	}
	traced, errs := compileVariablesTraced(path, env, VariablesProvider, variables...)
	programs := make(map[string]cel.Program, len(traced))
	for name, t := range traced {
		programs[name] = t.Program
	}
	return programs, traced, errs
}

func compileVariablesTraced(path *field.Path, env *cel.Env, VariablesProvider *VariablesProvider, variables ...admissionregistrationv1.Variable) (result map[string]TracedProgram, allErrs field.ErrorList) {
	if len(variables) == 0 {
		return nil, nil
	}
	result = make(map[string]TracedProgram, len(variables))
	for i, variable := range variables {
		compiled, errs := compileVariable(path.Index(i), env, VariablesProvider, variable, true)
		allErrs = append(allErrs, errs...)
		if compiled.Program != nil {
			result[variable.Name] = compiled
		}
	}
	return result, allErrs
}

func CompileVariables(path *field.Path, env *cel.Env, VariablesProvider *VariablesProvider, variables ...admissionregistrationv1.Variable) (result map[string]cel.Program, allErrs field.ErrorList) {
	if len(variables) == 0 {
		return nil, nil
	}
	result = make(map[string]cel.Program, len(variables))
	for i, variable := range variables {
		prog, errs := CompileVariable(path.Index(i), env, VariablesProvider, variable)
		allErrs = append(allErrs, errs...)
		if prog != nil {
			result[variable.Name] = prog
		}
	}
	return result, allErrs
}

func CompileMutation(path *field.Path, env *cel.Env, expression string, returnType *types.Type) (cel.Program, field.ErrorList) {
	prog, _, errs := compileMutation(path, env, expression, returnType, false)
	return prog, errs
}

func compileMutation(path *field.Path, env *cel.Env, expression string, returnType *types.Type, trace bool) (cel.Program, *cel.Ast, field.ErrorList) {
	var allErrs field.ErrorList
	{
		path := path.Child("expression")
		ast, issues := env.Compile(expression)
		if err := issues.Err(); err != nil {
			return nil, nil, append(allErrs, field.Invalid(path, expression, err.Error()))
		}
		if !ast.OutputType().IsExactType(returnType) {
			msg := fmt.Sprintf("output is expected to be of type %s", returnType.TypeName())
			return nil, nil, append(allErrs, field.Invalid(path, expression, msg))
		}
		prog, err := env.Program(ast, programOptions(trace)...)
		if err != nil {
			return nil, nil, append(allErrs, field.Invalid(path, expression, err.Error()))
		}
		return prog, ast, allErrs
	}
}

// CompileMutationWithTrace is the tracing-aware entry point for a mutation expression
// (ApplyConfiguration or JSONPatch). With trace false it is exactly CompileMutation and the
// returned AST is nil. With trace true the program is built with state tracking on and its AST
// is retained, so trace.Build can turn a later evaluation into a per-node breakdown of the
// mutation, the same way it already does for match conditions, variables and validations.
func CompileMutationWithTrace(path *field.Path, env *cel.Env, expression string, returnType *types.Type, trace bool) (TracedProgram, field.ErrorList) {
	prog, ast, errs := compileMutation(path, env, expression, returnType, trace)
	if prog == nil {
		return TracedProgram{}, errs
	}
	traced := TracedProgram{Program: prog}
	if trace {
		traced.AST = ast
	}
	return traced, errs
}

func CompileAuditAnnotation(path *field.Path, env *cel.Env, auditAnnotation admissionregistrationv1.AuditAnnotation) (cel.Program, field.ErrorList) {
	var allErrs field.ErrorList
	{
		path := path.Child("valueExpression")
		ast, issues := env.Compile(auditAnnotation.ValueExpression)
		if err := issues.Err(); err != nil {
			return nil, append(allErrs, field.Invalid(path, auditAnnotation.ValueExpression, err.Error()))
		}
		if !ast.OutputType().IsExactType(types.StringType) && !ast.OutputType().IsExactType(types.NullType) {
			msg := fmt.Sprintf("output is expected to be either of type %s or %s", types.StringType.TypeName(), types.NullType.TypeName())
			return nil, append(allErrs, field.Invalid(path, auditAnnotation.ValueExpression, msg))
		}
		prog, err := env.Program(ast)
		if err != nil {
			return nil, append(allErrs, field.Invalid(path, auditAnnotation.ValueExpression, err.Error()))
		}
		return prog, allErrs
	}
}

func CompileAuditAnnotations(path *field.Path, env *cel.Env, auditAnnotations ...admissionregistrationv1.AuditAnnotation) (result map[string]cel.Program, allErrs field.ErrorList) {
	if len(auditAnnotations) == 0 {
		return nil, nil
	}
	result = make(map[string]cel.Program, len(auditAnnotations))
	for i, auditAnnotation := range auditAnnotations {
		prog, errs := CompileAuditAnnotation(path.Index(i), env, auditAnnotation)
		allErrs = append(allErrs, errs...)
		if prog != nil {
			result[auditAnnotation.Key] = prog
		}
	}
	return result, allErrs
}

func CompileValidation(path *field.Path, env *cel.Env, rule admissionregistrationv1.Validation, trace bool) (Validation, field.ErrorList) {
	var allErrs field.ErrorList
	compiled := Validation{Message: rule.Message}
	{
		path := path.Child("expression")
		ast, issues := env.Compile(rule.Expression)
		if err := issues.Err(); err != nil {
			return Validation{}, append(allErrs, field.Invalid(path, rule.Expression, err.Error()))
		}
		if !ast.OutputType().IsExactType(types.BoolType) {
			msg := fmt.Sprintf("output is expected to be of type %s", types.BoolType.TypeName())
			return Validation{}, append(allErrs, field.Invalid(path, rule.Expression, msg))
		}
		program, err := env.Program(ast)
		if err != nil {
			return Validation{}, append(allErrs, field.Invalid(path, rule.Expression, err.Error()))
		}
		compiled.Program = program
		if trace {
			// explain-only twin; Program above still decides (see TracedProgram)
			if compiled.Traced, err = tracingProgram(env, ast); err != nil {
				return Validation{}, append(allErrs, field.Invalid(path, rule.Expression, err.Error()))
			}
			compiled.AST = ast
		}
	}
	if rule.MessageExpression != "" {
		path := path.Child("messageExpression")
		ast, issues := env.Compile(rule.MessageExpression)
		if err := issues.Err(); err != nil {
			return Validation{}, append(allErrs, field.Invalid(path, rule.MessageExpression, err.Error()))
		}
		if !ast.OutputType().IsExactType(types.StringType) {
			msg := fmt.Sprintf("output is expected to be of type %s", types.StringType.TypeName())
			return Validation{}, append(allErrs, field.Invalid(path, rule.MessageExpression, msg))
		}
		program, err := env.Program(ast)
		if err != nil {
			return Validation{}, append(allErrs, field.Invalid(path, rule.MessageExpression, err.Error()))
		}
		compiled.MessageExpression = program
	}
	return compiled, allErrs
}

func CompileMatchImageReference(path *field.Path, env *cel.Env, match v1beta1.MatchImageReference) (MatchImageReference, field.ErrorList) {
	var allErrs field.ErrorList
	if match.Glob != "" {
		path := path.Child("glob")
		g, err := glob.Compile(match.Glob)
		if err != nil {
			return nil, append(allErrs, field.Invalid(path, match.Glob, err.Error()))
		}
		return &matchGlob{Glob: g}, nil
	}
	if match.Expression != "" {
		path := path.Child("expression")
		ast, issues := env.Compile(match.Expression)
		if err := issues.Err(); err != nil {
			return nil, append(allErrs, field.Invalid(path, match.Expression, err.Error()))
		}
		if !ast.OutputType().IsExactType(types.BoolType) {
			msg := fmt.Sprintf("output is expected to be of type %s", types.BoolType.TypeName())
			return nil, append(allErrs, field.Invalid(path, match.Expression, msg))
		}
		prog, err := env.Program(ast)
		if err != nil {
			return nil, append(allErrs, field.Invalid(path, match.Expression, err.Error()))
		}
		return &matchCel{Program: prog}, nil
	}
	return nil, append(allErrs, field.Invalid(path, match, "either glob or expression must be set"))
}

func CompileMatchImageReferences(path *field.Path, env *cel.Env, matches ...v1beta1.MatchImageReference) (result []MatchImageReference, allErrs field.ErrorList) {
	if len(matches) == 0 {
		return nil, nil
	}
	result = make([]MatchImageReference, 0, len(matches))
	for i, match := range matches {
		match, errs := CompileMatchImageReference(path.Index(i), env, match)
		allErrs = append(allErrs, errs...)
		if match != nil {
			result = append(result, match)
		}
	}
	return result, allErrs
}

// CompileGeneration compiles the CEL expression of a generation entry.
func CompileGeneration(path *field.Path, env *cel.Env, generation policiesv1beta1.Generation) (cel.Program, field.ErrorList) {
	var allErrs field.ErrorList
	{
		path := path.Child("expression")
		ast, issues := env.Compile(generation.Expression)
		if err := issues.Err(); err != nil {
			return nil, append(allErrs, field.Invalid(path, generation.Expression, err.Error()))
		}
		if !ast.OutputType().IsExactType(types.BoolType) {
			msg := fmt.Sprintf("output is expected to be of type %s", types.BoolType.TypeName())
			return nil, append(allErrs, field.Invalid(path, generation.Expression, msg))
		}
		prog, err := env.Program(ast)
		if err != nil {
			return nil, append(allErrs, field.Invalid(path, generation.Expression, err.Error()))
		}
		return prog, allErrs
	}
}

func CompileGenerations(path *field.Path, env *cel.Env, generations ...policiesv1beta1.Generation) (result []cel.Program, allErrs field.ErrorList) {
	if len(generations) == 0 {
		return nil, nil
	}
	result = make([]cel.Program, 0, len(generations))
	for i, generation := range generations {
		prog, errs := CompileGeneration(path.Index(i), env, generation)
		allErrs = append(allErrs, errs...)
		if prog != nil {
			result = append(result, prog)
		}
	}
	return result, allErrs
}
