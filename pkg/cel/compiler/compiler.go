package compiler

import (
	"fmt"

	"github.com/gobwas/glob"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func CompileMatchCondition(path *field.Path, env *cel.Env, matchCondition admissionregistrationv1.MatchCondition) (cel.Program, field.ErrorList) {
	var allErrs field.ErrorList
	{
		path := path.Child("expression")
		ast, issues := env.Compile(matchCondition.Expression)
		if err := issues.Err(); err != nil {
			return nil, append(allErrs, field.Invalid(path, matchCondition.Expression, err.Error()))
		}
		if !ast.OutputType().IsExactType(types.BoolType) {
			msg := fmt.Sprintf("output is expected to be of type %s", types.BoolType.TypeName())
			return nil, append(allErrs, field.Invalid(path, matchCondition.Expression, msg))
		}
		prog, err := env.Program(ast)
		if err != nil {
			return nil, append(allErrs, field.Invalid(path, matchCondition.Expression, err.Error()))
		}
		return prog, allErrs
	}
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

func CompileVariable(path *field.Path, env *cel.Env, variablesProvider *variablesProvider, variable admissionregistrationv1.Variable) (cel.Program, field.ErrorList) {
	var allErrs field.ErrorList
	{
		path := path.Child("expression")
		ast, issues := env.Compile(variable.Expression)
		if err := issues.Err(); err != nil {
			return nil, append(allErrs, field.Invalid(path, variable.Expression, err.Error()))
		}
		variablesProvider.RegisterField(variable.Name, ast.OutputType())
		prog, err := env.Program(ast)
		if err != nil {
			return nil, append(allErrs, field.Invalid(path, variable.Expression, err.Error()))
		}
		return prog, allErrs
	}
}

func CompileVariables(path *field.Path, env *cel.Env, variablesProvider *variablesProvider, variables ...admissionregistrationv1.Variable) (result map[string]cel.Program, allErrs field.ErrorList) {
	if len(variables) == 0 {
		return nil, nil
	}
	result = make(map[string]cel.Program, len(variables))
	for i, variable := range variables {
		prog, errs := CompileVariable(path.Index(i), env, variablesProvider, variable)
		allErrs = append(allErrs, errs...)
		if prog != nil {
			result[variable.Name] = prog
		}
	}
	return result, allErrs
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

func CompileValidation(path *field.Path, env *cel.Env, rule admissionregistrationv1.Validation) (Validation, field.ErrorList) {
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

func CompileMatchImageReference(path *field.Path, env *cel.Env, match v1alpha1.MatchImageReference) (MatchImageReference, field.ErrorList) {
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

func CompileMatchImageReferences(path *field.Path, env *cel.Env, matches ...v1alpha1.MatchImageReference) (result []MatchImageReference, allErrs field.ErrorList) {
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

func compileGeneration(path *field.Path, env *cel.Env, generation policiesv1alpha1.Generation) (cel.Program, field.ErrorList) {
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

func CompileGenerations(path *field.Path, env *cel.Env, generations ...policiesv1alpha1.Generation) (result []cel.Program, allErrs field.ErrorList) {
	if len(generations) == 0 {
		return nil, nil
	}
	result = make([]cel.Program, 0, len(generations))
	for i, generation := range generations {
		prog, errs := compileGeneration(path.Index(i), env, generation)
		allErrs = append(allErrs, errs...)
		if prog != nil {
			result = append(result, prog)
		}
	}
	return result, allErrs
}
