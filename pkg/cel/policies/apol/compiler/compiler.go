package compiler

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apiserver/pkg/cel/environment"
)

var apolCompilerVersion = version.MajorMinor(1, 0)

// CompiledCondition holds one compiled condition attached to an AuthorizingRule.
type CompiledCondition struct {
	ID          string
	Condition   string
	Expression  cel.Program
	Effect      policiesv1alpha1.AuthorizingConditionEffect
	Description string
}

// CompiledRule holds the compiled programs for one AuthorizingRule.
type CompiledRule struct {
	Name            string
	MatchConditions []cel.Program
	// Expression evaluates the rule; nil means the rule is unconditional.
	Expression cel.Program
	Effect     policiesv1alpha1.AuthorizingRuleEffect
	Conditions []CompiledCondition
}

// Policy holds all compiled programs for one AuthorizingPolicy.
type Policy struct {
	Name            string
	FailurePolicy   policiesv1alpha1.AuthorizingFailurePolicyType
	MatchConditions []cel.Program
	Variables       map[string]cel.Program
	Rules           []CompiledRule
}

// Compiler compiles an AuthorizingPolicy into ready-to-evaluate CEL programs.
type Compiler interface {
	Compile(policy *policiesv1alpha1.AuthorizingPolicy) (*Policy, field.ErrorList)
}

type compilerImpl struct{}

// NewCompiler returns a new AuthorizingPolicy Compiler.
func NewCompiler() Compiler {
	return &compilerImpl{}
}

func (c *compilerImpl) Compile(policy *policiesv1alpha1.AuthorizingPolicy) (*Policy, field.ErrorList) {
	var allErrs field.ErrorList
	errFmt := fmt.Sprintf("authorizing policy compiler %s: %%s", apolCompilerVersion)

	// Base CEL environment — authorization phase exposes only request + variables.
	base := environment.MustBaseEnvSet(apolCompilerVersion)
	env, err := base.Env(environment.StoredExpressions)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(errFmt, err)))
	}

	variablesProvider := compiler.NewVariablesProvider(env.CELTypeProvider())
	env, err = env.Extend(
		cel.Variable(compiler.RequestKey, cel.DynType),
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
		cel.CustomTypeProvider(variablesProvider),
	)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(errFmt, err)))
	}

	path := field.NewPath("spec")
	spec := policy.Spec

	// Compile match conditions (policy-level pre-filter).
	matchConds, errs := compiler.CompileMatchConditions(path.Child("matchConditions"), env, toV1MatchConditions(spec.MatchConditions)...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	// Compile variables.
	variables, errs := compiler.CompileVariables(path.Child("variables"), env, variablesProvider, toV1Variables(spec.Variables)...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	// Compile rules.
	compiledRules := make([]CompiledRule, 0, len(spec.Rules))
	for i, rule := range spec.Rules {
		rpath := path.Child("rules").Index(i)

		ruleMatchConds, errs := compiler.CompileMatchConditions(rpath.Child("matchConditions"), env, toV1MatchConditions(rule.MatchConditions)...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}

		var exprProg cel.Program
		if rule.Expression != "" {
			ast, issues := env.Compile(rule.Expression)
			if issues != nil && issues.Err() != nil {
				return nil, append(allErrs, field.Invalid(rpath.Child("expression"), rule.Expression, issues.Err().Error()))
			}
			if !ast.OutputType().IsExactType(types.BoolType) {
				return nil, append(allErrs, field.Invalid(rpath.Child("expression"), rule.Expression, "must evaluate to bool"))
			}
			exprProg, err = env.Program(ast)
			if err != nil {
				return nil, append(allErrs, field.InternalError(rpath.Child("expression"), err))
			}
		}

		compiledConds := make([]CompiledCondition, 0, len(rule.Conditions))
		for j, cond := range rule.Conditions {
			cpath := rpath.Child("conditions").Index(j)
			ast, issues := env.Compile(cond.Expression)
			if issues != nil && issues.Err() != nil {
				return nil, append(allErrs, field.Invalid(cpath.Child("expression"), cond.Expression, issues.Err().Error()))
			}
			if !ast.OutputType().IsExactType(types.BoolType) {
				return nil, append(allErrs, field.Invalid(cpath.Child("expression"), cond.Expression, "must evaluate to bool"))
			}
			condProg, err := env.Program(ast)
			if err != nil {
				return nil, append(allErrs, field.InternalError(cpath.Child("expression"), err))
			}
			compiledConds = append(compiledConds, CompiledCondition{
				ID:          cond.ID,
				Condition:   cond.Expression,
				Expression:  condProg,
				Effect:      cond.Effect,
				Description: cond.Description,
			})
		}

		compiledRules = append(compiledRules, CompiledRule{
			Name:            rule.Name,
			MatchConditions: ruleMatchConds,
			Expression:      exprProg,
			Effect:          rule.Effect,
			Conditions:      compiledConds,
		})
	}

	return &Policy{
		Name:            policy.Name,
		FailurePolicy:   spec.FailurePolicy,
		MatchConditions: matchConds,
		Variables:       variables,
		Rules:           compiledRules,
	}, nil
}

// toV1MatchConditions converts AuthorizingMatchCondition to admissionregistrationv1.MatchCondition.
func toV1MatchConditions(in []policiesv1alpha1.AuthorizingMatchCondition) []admissionregistrationv1.MatchCondition {
	out := make([]admissionregistrationv1.MatchCondition, 0, len(in))
	for _, mc := range in {
		out = append(out, admissionregistrationv1.MatchCondition{
			Name:       mc.Name,
			Expression: mc.Expression,
		})
	}
	return out
}

// toV1Variables converts AuthorizingVariable to admissionregistrationv1.Variable.
func toV1Variables(in []policiesv1alpha1.AuthorizingVariable) []admissionregistrationv1.Variable {
	out := make([]admissionregistrationv1.Variable, 0, len(in))
	for _, v := range in {
		out = append(out, admissionregistrationv1.Variable{
			Name:       v.Name,
			Expression: v.Expression,
		})
	}
	return out
}
