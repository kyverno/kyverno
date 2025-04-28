package compiler

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	GlobalContextKey   = "globalContext"
	HttpKey            = "http"
	ImageDataKey       = "image"
	NamespaceObjectKey = "namespaceObject"
	ObjectKey          = "object"
	OldObjectKey       = "oldObject"
	RequestKey         = "request"
	ResourceKey        = "resource"
	VariablesKey       = "variables"
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
	result = map[string]cel.Program{}
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
	result = map[string]cel.Program{}
	for i, auditAnnotation := range auditAnnotations {
		prog, errs := CompileAuditAnnotation(path.Index(i), env, auditAnnotation)
		allErrs = append(allErrs, errs...)
		if prog != nil {
			result[auditAnnotation.Key] = prog
		}
	}
	return result, allErrs
}

func CompileValidation(path *field.Path, rule admissionregistrationv1.Validation, env *cel.Env) (Validation, field.ErrorList) {
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
