package compiler

import (
	"strings"
	"sync"

	"github.com/google/cel-go/cel"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/version"
	apiservercel "k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/environment"
)

var (
	policyExceptionEnvSetInit sync.Once
	policyExceptionEnvSet     *environment.EnvSet
	errPolicyExceptionEnvSet  error
)

// CompilePolicyExceptionMatchConditions validates PolicyException match conditions against
// the admission variables shared by Kyverno CEL policy evaluators.
func CompilePolicyExceptionMatchConditions(conditions []admissionregistrationv1.MatchCondition, preexistingExpressions map[string]bool) field.ErrorList {
	path := field.NewPath("spec").Child("matchConditions")
	var allErrors field.ErrorList
	conditionNames := sets.New[string]()

	if len(conditions) > 64 {
		allErrors = append(allErrors, field.TooMany(path, len(conditions), 64))
	}

	envSet, err := getPolicyExceptionEnvSet()
	if err != nil {
		return append(allErrors, field.InternalError(path, err))
	}

	for i, condition := range conditions {
		conditionPath := path.Index(i)
		trimmedExpression := strings.TrimSpace(condition.Expression)
		if trimmedExpression == "" {
			allErrors = append(allErrors, field.Required(conditionPath.Child("expression"), ""))
		} else {
			envType := environment.NewExpressions
			if preexistingExpressions[condition.Expression] {
				envType = environment.StoredExpressions
			}
			env, err := envSet.Env(envType)
			if err != nil {
				allErrors = append(allErrors, field.InternalError(conditionPath.Child("expression"), err))
			} else {
				condition.Expression = trimmedExpression
				_, errs := CompileMatchCondition(conditionPath, env, condition)
				allErrors = append(allErrors, errs...)
			}
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

func getPolicyExceptionEnvSet() (*environment.EnvSet, error) {
	policyExceptionEnvSetInit.Do(func() {
		base := environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion())
		storedEnv, err := base.Env(environment.StoredExpressions)
		if err != nil {
			errPolicyExceptionEnvSet = err
			return
		}
		declProvider := apiservercel.NewDeclTypeProvider(NamespaceType, RequestType)
		declOptions, err := declProvider.EnvOptions(storedEnv.CELTypeProvider())
		if err != nil {
			errPolicyExceptionEnvSet = err
			return
		}
		envOptions := make([]cel.EnvOption, 0, 6+len(declOptions))
		envOptions = append(envOptions,
			cel.Variable(NamespaceObjectKey, NamespaceType.CelType()),
			cel.Variable(ObjectKey, cel.DynType),
			cel.Variable(OldObjectKey, cel.DynType),
			cel.Variable(RequestKey, RequestType.CelType()),
			cel.Types(NamespaceType.CelType()),
			cel.Types(RequestType.CelType()),
		)
		envOptions = append(envOptions, declOptions...)
		policyExceptionEnvSet, errPolicyExceptionEnvSet = base.Extend(environment.VersionedOptions{
			IntroducedVersion: version.MajorMinor(1, 0),
			EnvOptions:        envOptions,
		})
	})
	return policyExceptionEnvSet, errPolicyExceptionEnvSet
}
