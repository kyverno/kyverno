package admissionpolicy

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	"k8s.io/apiserver/pkg/cel/environment"
)

type Compiler struct {
	compositedCompiler cel.CompositedCompiler
	validations        []admissionregistrationv1.Validation
	mutations          []admissionregistrationv1alpha1.Mutation
	auditAnnotations   []admissionregistrationv1.AuditAnnotation
	matchConditions    []admissionregistrationv1.MatchCondition
	variables          []admissionregistrationv1.Variable
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

func (c Compiler) CompileValidations(optionalVars cel.OptionalVariableDeclarations) cel.ConditionEvaluator {
	return c.compositedCompiler.CompileCondition(
		c.convertValidations(),
		optionalVars,
		environment.StoredExpressions,
	)
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
