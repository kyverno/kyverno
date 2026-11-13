// Package compiler compiles AuthorizingPolicy resources into CEL programs that
// support partial evaluation: `request` and `variables` are known when the
// SubjectAccessReview arrives, while `object` and `oldObject` stay unknown
// until the AuthorizationConditionsReview callback.
package compiler

import (
	"fmt"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/version"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/cel/environment"
)

var compilerVersion = version.MajorMinor(1, 0)

// CompiledRule is one AuthorizingPolicy rule after compilation.
type CompiledRule struct {
	Name        string
	Effect      policiesv1alpha1.AuthorizingRuleEffect
	Description string
	// AST is the checked AST, kept to compute residual expressions.
	AST *cel.Ast
	// Program is built with partial evaluation and state tracking enabled.
	Program cel.Program
}

// CompiledVariable is one AuthorizingPolicy variable after compilation.
type CompiledVariable struct {
	Name    string
	Program cel.Program
}

// Policy is the compiled form of one AuthorizingPolicy.
type Policy struct {
	Policy    *policiesv1alpha1.AuthorizingPolicy
	Env       *cel.Env
	Variables []CompiledVariable
	Rules     []CompiledRule
}

// Compiler compiles an AuthorizingPolicy into ready-to-evaluate CEL programs.
type Compiler interface {
	Compile(policy *policiesv1alpha1.AuthorizingPolicy) (*Policy, field.ErrorList)
}

type compilerImpl struct{}

// NewCompiler returns a new AuthorizingPolicy compiler.
func NewCompiler() Compiler {
	return &compilerImpl{}
}

// baseEnv returns the pure Kubernetes CEL environment (standard library plus
// Kubernetes libraries). Kyverno libraries that perform I/O are deliberately
// not registered: residual conditions must be pure functions of the snapshot.
func baseEnv() (*cel.Env, error) {
	return environment.MustBaseEnvSet(compilerVersion).Env(environment.StoredExpressions)
}

// ProgramOptions are the program options shared by authorization-time and
// conditions-review-time programs.
func ProgramOptions(partial bool) []cel.ProgramOption {
	opts := []cel.ProgramOption{cel.CostLimit(celconfig.PerCallLimit)}
	if partial {
		opts = append(opts, cel.EvalOptions(cel.OptPartialEval, cel.OptTrackState))
	}
	return opts
}

// NewConditionsEnv returns the environment used to compile residual
// conditions during the AuthorizationConditionsReview callback. Only `object`
// and `oldObject` are declared, so a residual that still references `request`
// or `variables` fails to compile and is treated as an evaluation error.
func NewConditionsEnv() (*cel.Env, error) {
	env, err := baseEnv()
	if err != nil {
		return nil, err
	}
	return env.Extend(
		cel.Variable(celcompiler.ObjectKey, cel.DynType),
		cel.Variable(celcompiler.OldObjectKey, cel.DynType),
	)
}

func (c *compilerImpl) Compile(policy *policiesv1alpha1.AuthorizingPolicy) (*Policy, field.ErrorList) {
	var allErrs field.ErrorList
	errFmt := fmt.Sprintf("authorizing policy compiler %s: %%s", compilerVersion)
	base, err := baseEnv()
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(errFmt, err)))
	}
	variablesProvider := celcompiler.NewVariablesProvider(base.CELTypeProvider())
	// Variables are evaluated at authorization time only, so they cannot
	// reference object or oldObject. Declaring only request and variables makes
	// such references a compile error instead of a silent runtime failure.
	varEnv, err := base.Extend(
		cel.Variable(celcompiler.RequestKey, cel.DynType),
		cel.Variable(celcompiler.VariablesKey, celcompiler.VariablesType),
		cel.CustomTypeProvider(variablesProvider),
	)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(errFmt, err)))
	}
	path := field.NewPath("spec")
	spec := policy.Spec
	variables := make([]CompiledVariable, 0, len(spec.Variables))
	for i, variable := range spec.Variables {
		vpath := path.Child("variables").Index(i).Child("expression")
		ast, issues := varEnv.Compile(variable.Expression)
		if err := issues.Err(); err != nil {
			allErrs = append(allErrs, field.Invalid(vpath, variable.Expression, err.Error()))
			continue
		}
		variablesProvider.RegisterField(variable.Name, ast.OutputType())
		prog, err := varEnv.Program(ast, ProgramOptions(true)...)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(vpath, variable.Expression, err.Error()))
			continue
		}
		variables = append(variables, CompiledVariable{Name: variable.Name, Program: prog})
	}
	ruleEnv, err := varEnv.Extend(
		cel.Variable(celcompiler.ObjectKey, cel.DynType),
		cel.Variable(celcompiler.OldObjectKey, cel.DynType),
	)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(errFmt, err)))
	}
	rules := make([]CompiledRule, 0, len(spec.Rules))
	for i, rule := range spec.Rules {
		rpath := path.Child("rules").Index(i).Child("expression")
		ast, issues := ruleEnv.Compile(rule.Expression)
		if err := issues.Err(); err != nil {
			allErrs = append(allErrs, field.Invalid(rpath, rule.Expression, err.Error()))
			continue
		}
		if !ast.OutputType().IsExactType(types.BoolType) {
			allErrs = append(allErrs, field.Invalid(rpath, rule.Expression, "must evaluate to bool"))
			continue
		}
		prog, err := ruleEnv.Program(ast, ProgramOptions(true)...)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(rpath, rule.Expression, err.Error()))
			continue
		}
		rules = append(rules, CompiledRule{
			Name:        rule.Name,
			Effect:      rule.Effect,
			Description: rule.Description,
			AST:         ast,
			Program:     prog,
		})
	}
	if len(allErrs) > 0 {
		return nil, allErrs
	}
	return &Policy{
		Policy:    policy,
		Env:       ruleEnv,
		Variables: variables,
		Rules:     rules,
	}, nil
}
