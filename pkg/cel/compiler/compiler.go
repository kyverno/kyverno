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

func CompileMatchConditions(path *field.Path, matchConditions []admissionregistrationv1.MatchCondition, env *cel.Env) ([]cel.Program, field.ErrorList) {
	if len(matchConditions) == 0 {
		return nil, nil
	}
	var allErrs field.ErrorList
	result := make([]cel.Program, 0, len(matchConditions))
	for i, matchCondition := range matchConditions {
		path := path.Index(i).Child("expression")
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
		result = append(result, prog)
	}
	return result, nil
}

func CompileVariables(path *field.Path, variables []admissionregistrationv1.Variable, variablesProvider *variablesProvider, env *cel.Env, result map[string]cel.Program) field.ErrorList {
	var allErrs field.ErrorList
	for i, variable := range variables {
		path := path.Index(i).Child("expression")
		ast, issues := env.Compile(variable.Expression)
		if err := issues.Err(); err != nil {
			return append(allErrs, field.Invalid(path, variable.Expression, err.Error()))
		}
		variablesProvider.RegisterField(variable.Name, ast.OutputType())
		prog, err := env.Program(ast)
		if err != nil {
			return append(allErrs, field.Invalid(path, variable.Expression, err.Error()))
		}
		result[variable.Name] = prog
	}
	return nil
}

func CompileAuditAnnotations(path *field.Path, auditAnnotations []admissionregistrationv1.AuditAnnotation, env *cel.Env, result map[string]cel.Program) field.ErrorList {
	var allErrs field.ErrorList
	for i, auditAnnotation := range auditAnnotations {
		path := path.Index(i).Child("valueExpression")
		ast, issues := env.Compile(auditAnnotation.ValueExpression)
		if err := issues.Err(); err != nil {
			return append(allErrs, field.Invalid(path, auditAnnotation.ValueExpression, err.Error()))
		}
		if !ast.OutputType().IsExactType(types.StringType) && !ast.OutputType().IsExactType(types.NullType) {
			msg := fmt.Sprintf("output is expected to be either of type %s or %s", types.StringType.TypeName(), types.NullType.TypeName())
			return append(allErrs, field.Invalid(path, auditAnnotation.ValueExpression, msg))
		}
		prog, err := env.Program(ast)
		if err != nil {
			return append(allErrs, field.Invalid(path, auditAnnotation.ValueExpression, err.Error()))
		}
		result[auditAnnotation.Key] = prog
	}
	return nil
}

func CompileValidation(path *field.Path, rule admissionregistrationv1.Validation, env *cel.Env) (Validation, field.ErrorList) {
	var allErrs field.ErrorList
	compiled := Validation{
		Message: rule.Message,
	}
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
	return compiled, nil
}
