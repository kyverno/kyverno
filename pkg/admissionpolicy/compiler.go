package admissionpolicy

import (
	"fmt"

	celgo "github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	apiservercel "k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/environment"
)

type variableDeclEnvs map[cel.OptionalVariableDeclarations]*environment.EnvSet

type Compiler struct {
	compositedCompiler cel.CompositedCompiler
	varEnvs            variableDeclEnvs
	validations        []admissionregistrationv1.Validation
	mutations          []admissionregistrationv1alpha1.Mutation
	auditAnnotations   []admissionregistrationv1.AuditAnnotation
	matchConditions    []admissionregistrationv1.MatchCondition
	variables          []admissionregistrationv1.Variable
}

type CompositedConditionEvaluator struct {
	cel.ConditionEvaluator

	compositionEnv *cel.CompositionEnv
}

func NewCompiler(
	matchConditions []admissionregistrationv1.MatchCondition,
	variables []admissionregistrationv1.Variable,
) (*Compiler, error) {
	compositedCompiler, err := cel.NewCompositedCompiler(environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion(), false))
	if err != nil {
		return nil, err
	}
	return &Compiler{
		compositedCompiler: *compositedCompiler,
		matchConditions:    matchConditions,
		variables:          variables,
	}, nil
}

func (c *Compiler) WithValidations(validations []admissionregistrationv1.Validation) {
	c.validations = validations
}

func (c *Compiler) WithMutations(mutations []admissionregistrationv1alpha1.Mutation) {
	c.mutations = mutations
}

func (c *Compiler) WithAuditAnnotations(auditAnnotations []admissionregistrationv1.AuditAnnotation) {
	c.auditAnnotations = auditAnnotations
}

func (c Compiler) CompileMutations(patchOptions cel.OptionalVariableDeclarations) []patch.Patcher {
	var patchers []patch.Patcher
	for _, m := range c.mutations {
		switch m.PatchType {
		case admissionregistrationv1alpha1.PatchTypeJSONPatch:
			if m.JSONPatch != nil {
				accessor := &patch.JSONPatchCondition{
					Expression: m.JSONPatch.Expression,
				}
				compileResult := c.compositedCompiler.CompileMutatingEvaluator(accessor, patchOptions, environment.StoredExpressions)
				patchers = append(patchers, patch.NewJSONPatcher(compileResult))
			}
		case admissionregistrationv1alpha1.PatchTypeApplyConfiguration:
			if m.ApplyConfiguration != nil {
				accessor := &patch.ApplyConfigurationCondition{
					Expression: m.ApplyConfiguration.Expression,
				}
				compileResult := c.compositedCompiler.CompileMutatingEvaluator(accessor, patchOptions, environment.StoredExpressions)
				patchers = append(patchers, patch.NewApplyConfigurationPatcher(compileResult))
			}
		}
	}
	return patchers
}

func (c Compiler) CompileVariables(optionalVars cel.OptionalVariableDeclarations) {
	c.compositedCompiler.CompileAndStoreVariables(
		c.convertVariables(),
		optionalVars,
		environment.StoredExpressions,
	)
}

func (c *Compiler) CompileValidations(optionalVars cel.OptionalVariableDeclarations) cel.ConditionEvaluator {
	env, err := c.varEnvs[optionalVars].Env(environment.StoredExpressions)
	if err != nil {
		logger.Info("unexpected error loading CEL environment: %v", err)
		return nil
	}
	expressions := c.convertValidations()
	results := make([]cel.CompilationResult, len(expressions))
	for i, expr := range expressions {
		resultError := func(errorString string, errType apiservercel.ErrorType, cause error) cel.CompilationResult {
			return cel.CompilationResult{
				Error: &apiservercel.Error{
					Type:   errType,
					Detail: errorString,
					Cause:  cause,
				},
				ExpressionAccessor: expr,
			}
		}
		ast, issues := compiler.GetCompiledAST(expr.GetExpression(), env)
		if issues != nil {
			logger.Info("unexpected error compiling expression: %v", issues)
			results[i] = resultError("compilation failed: "+issues.String(), apiservercel.ErrorTypeInvalid, apiservercel.NewCompilationError(issues))
			continue
		}
		found := false
		returnTypes := expr.ReturnTypes()
		for _, returnType := range returnTypes {
			if ast.OutputType().IsExactType(returnType) || celgo.AnyType.IsExactType(returnType) {
				found = true
				break
			}
		}
		if !found {
			var reason string
			if len(returnTypes) == 1 {
				reason = fmt.Sprintf("must evaluate to %v but got %v", returnTypes[0].String(), ast.OutputType().String())
			} else {
				reason = fmt.Sprintf("must evaluate to one of %v but got %v", returnTypes, ast.OutputType().String())
			}

			results[i] = resultError(reason, apiservercel.ErrorTypeInvalid, nil)
			continue
		}
		prog, err := env.Program(ast,
			celgo.InterruptCheckFrequency(celconfig.CheckFrequency),
		)
		if err != nil {
			results[i] = resultError("program instantiation failed: "+err.Error(), apiservercel.ErrorTypeInternal, nil)
			continue
		}
		results[i] = cel.CompilationResult{Program: prog}
	}
	return &CompositedConditionEvaluator{
		ConditionEvaluator: cel.NewCondition(results),
		compositionEnv:     c.compositedCompiler.CompositionEnv,
	}
}

func (c Compiler) CompileMessageExpressions(optionalVars cel.OptionalVariableDeclarations) cel.ConditionEvaluator {
	return c.compositedCompiler.CompileCondition(
		c.convertMessageExpressions(),
		optionalVars,
		environment.StoredExpressions,
	)
}

func (c Compiler) CompileAuditAnnotationsExpressions(optionalVars cel.OptionalVariableDeclarations) cel.ConditionEvaluator {
	return c.compositedCompiler.CompileCondition(
		c.convertAuditAnnotations(),
		optionalVars,
		environment.StoredExpressions,
	)
}

func (c Compiler) CompileMatchConditions(optionalVars cel.OptionalVariableDeclarations) cel.ConditionEvaluator {
	return c.compositedCompiler.CompileCondition(
		c.convertMatchConditions(),
		optionalVars,
		environment.StoredExpressions,
	)
}

func (c Compiler) convertValidations() []cel.ExpressionAccessor {
	celExpressionAccessor := make([]cel.ExpressionAccessor, len(c.validations))
	for i, validation := range c.validations {
		validation := validating.ValidationCondition{
			Expression: validation.Expression,
			Message:    validation.Message,
			Reason:     validation.Reason,
		}
		celExpressionAccessor[i] = &validation
	}
	return celExpressionAccessor
}

func (c Compiler) convertMessageExpressions() []cel.ExpressionAccessor {
	celExpressionAccessor := make([]cel.ExpressionAccessor, len(c.validations))
	for i, validation := range c.validations {
		if validation.MessageExpression != "" {
			condition := validating.MessageExpressionCondition{
				MessageExpression: validation.MessageExpression,
			}
			celExpressionAccessor[i] = &condition
		}
	}
	return celExpressionAccessor
}

func (c Compiler) convertAuditAnnotations() []cel.ExpressionAccessor {
	celExpressionAccessor := make([]cel.ExpressionAccessor, len(c.auditAnnotations))
	for i, validation := range c.auditAnnotations {
		validation := validating.AuditAnnotationCondition{
			Key:             validation.Key,
			ValueExpression: validation.ValueExpression,
		}
		celExpressionAccessor[i] = &validation
	}
	return celExpressionAccessor
}

func (c Compiler) convertMatchConditions() []cel.ExpressionAccessor {
	celExpressionAccessor := make([]cel.ExpressionAccessor, len(c.matchConditions))
	for i, condition := range c.matchConditions {
		condition := matchconditions.MatchCondition{
			Name:       condition.Name,
			Expression: condition.Expression,
		}
		celExpressionAccessor[i] = &condition
	}
	return celExpressionAccessor
}

func (c Compiler) convertVariables() []cel.NamedExpressionAccessor {
	namedExpressions := make([]cel.NamedExpressionAccessor, len(c.variables))
	for i, variable := range c.variables {
		namedExpressions[i] = &validating.Variable{
			Name:       variable.Name,
			Expression: variable.Expression,
		}
	}
	return namedExpressions
}
