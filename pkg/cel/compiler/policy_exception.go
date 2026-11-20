package compiler

import (
	"strings"

	"github.com/google/cel-go/cel"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// CompilePolicyExceptionMatchConditions validates PolicyException match conditions against env,
// the environment the exception's expressions are compiled with at evaluation time.
func CompilePolicyExceptionMatchConditions(env *cel.Env, conditions []admissionregistrationv1.MatchCondition) field.ErrorList {
	path := field.NewPath("spec").Child("matchConditions")
	var allErrors field.ErrorList
	conditionNames := sets.New[string]()

	if len(conditions) > 64 {
		allErrors = append(allErrors, field.TooMany(path, len(conditions), 64))
	}

	for i, condition := range conditions {
		conditionPath := path.Index(i)
		trimmedExpression := strings.TrimSpace(condition.Expression)
		if trimmedExpression == "" {
			allErrors = append(allErrors, field.Required(conditionPath.Child("expression"), ""))
		} else {
			condition.Expression = trimmedExpression
			_, errs := CompileMatchCondition(conditionPath, env, condition)
			allErrors = append(allErrors, errs...)
		}

		if condition.Name == "" {
			allErrors = append(allErrors, field.Required(conditionPath.Child("name"), ""))
		} else if errs := validation.IsQualifiedName(condition.Name); len(errs) > 0 {
			for _, err := range errs {
				allErrors = append(allErrors, field.Invalid(conditionPath.Child("name"), condition.Name, err))
			}
		} else if conditionNames.Has(condition.Name) {
			allErrors = append(allErrors, field.Duplicate(conditionPath.Child("name"), condition.Name))
		} else {
			conditionNames.Insert(condition.Name)
		}
	}

	return allErrors
}
