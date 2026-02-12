package compiler

import (
	"fmt"
	"strings"
	"sync"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	plugincel "k8s.io/apiserver/pkg/admission/plugin/cel"
	matchconditions "k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	"k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/environment"
)

var (
	lazyNonStrictStatelessCELCompilerInit sync.Once
	lazyNonStrictStatelessCELCompiler     plugincel.Compiler
)

func CompileMatchConditionsWithKubernetesEnv(conditions []admissionregistrationv1.MatchCondition, preexistingExpressions map[string]bool) field.ErrorList {
	var allErrors field.ErrorList
	conditionNames := sets.New[string]()

	if len(conditions) > 64 {
		allErrors = append(allErrors, field.TooMany(field.NewPath("matchConditions"), len(conditions), 64))
	}

	for i, condition := range conditions {
		allErrors = append(allErrors, validateMatchCondition(&condition, preexistingExpressions, field.NewPath("matchConditions").Index(i))...)
		if len(condition.Name) > 0 {
			if conditionNames.Has(condition.Name) {
				allErrors = append(allErrors, field.Duplicate(field.NewPath("matchConditions").Index(i).Child("name"), condition.Name))
			} else {
				conditionNames.Insert(condition.Name)
			}
		}
	}
	return allErrors
}

func validateMatchCondition(condition *admissionregistrationv1.MatchCondition, preexistingExpressions map[string]bool, fldPath *field.Path) field.ErrorList {
	var allErrors field.ErrorList
	trimmedExpression := strings.TrimSpace(condition.Expression)
	if len(trimmedExpression) == 0 {
		allErrors = append(allErrors, field.Required(fldPath.Child("expression"), ""))
	} else {
		allErrors = append(allErrors, validateMatchConditionsExpression(trimmedExpression, preexistingExpressions, fldPath.Child("expression"))...)
	}

	if len(condition.Name) == 0 {
		allErrors = append(allErrors, field.Required(fldPath.Child("name"), ""))
	} else {
		if errs := validation.IsQualifiedName(condition.Name); len(errs) > 0 {
			for _, err := range errs {
				allErrors = append(allErrors, field.Invalid(fldPath.Child("name"), condition.Name, err))
			}
		}
	}

	return allErrors
}

func validateMatchConditionsExpression(expression string, preexistingExpressions map[string]bool, fldPath *field.Path) field.ErrorList {
	envType := environment.NewExpressions
	if preexistingExpressions[expression] {
		envType = environment.StoredExpressions
	}

	lazyNonStrictStatelessCELCompilerInit.Do(func() {
		lazyNonStrictStatelessCELCompiler = plugincel.NewCompiler(environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion()))
	})

	compiler := lazyNonStrictStatelessCELCompiler
	return validateCELCondition(compiler, &matchconditions.MatchCondition{
		Expression: expression,
	}, plugincel.OptionalVariableDeclarations{
		HasParams:     false,
		HasAuthorizer: true,
	}, envType, fldPath)
}

func validateCELCondition(compiler plugincel.Compiler, expression plugincel.ExpressionAccessor, variables plugincel.OptionalVariableDeclarations, envType environment.Type, fldPath *field.Path) field.ErrorList {
	var allErrors field.ErrorList
	result := compiler.CompileCELExpression(expression, variables, envType)
	if result.Error != nil {
		allErrors = append(allErrors, convertCELErrorToValidationError(fldPath, expression, result.Error))
	}
	return allErrors
}

func convertCELErrorToValidationError(fldPath *field.Path, expression plugincel.ExpressionAccessor, err error) *field.Error {
	if celErr, ok := err.(*cel.Error); ok {
		switch celErr.Type {
		case cel.ErrorTypeRequired:
			return field.Required(fldPath, celErr.Detail)
		case cel.ErrorTypeInvalid:
			return field.Invalid(fldPath, expression.GetExpression(), celErr.Detail)
		case cel.ErrorTypeInternal:
			return field.InternalError(fldPath, celErr)
		}
	}
	return field.InternalError(fldPath, fmt.Errorf("unsupported error type: %w", err))
}
