package compiler

import (
	cel "github.com/google/cel-go/cel"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	compiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/http"
	"github.com/kyverno/kyverno/pkg/cel/libs/image"
	"github.com/kyverno/kyverno/pkg/cel/libs/imagedata"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"github.com/kyverno/kyverno/pkg/cel/libs/user"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/version"
	plugincel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating"
	patch "k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	matchconditions "k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	environment "k8s.io/apiserver/pkg/cel/environment"
)

type Compiler interface {
	Compile(policy *policiesv1alpha1.MutatingPolicy, exceptions []*policiesv1alpha1.PolicyException) (*Policy, field.ErrorList)
}

func NewCompiler() Compiler {
	return &compilerImpl{}
}

type compilerImpl struct{}

func (c *compilerImpl) Compile(policy *policiesv1alpha1.MutatingPolicy, exceptions []*policiesv1alpha1.PolicyException) (*Policy, field.ErrorList) {
	var allErrs field.ErrorList

	baseEnvSet := environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion(), false)
	extendedEnvSet, err := baseEnvSet.Extend(
		environment.VersionedOptions{
			IntroducedVersion: version.MajorMinor(1, 0),
			EnvOptions: []cel.EnvOption{
				cel.Variable(compiler.NamespaceObjectKey, compiler.NamespaceType.CelType()),
				cel.Variable(compiler.ObjectKey, cel.DynType),
				cel.Variable(compiler.OldObjectKey, cel.DynType),
				cel.Variable(compiler.RequestKey, compiler.RequestType.CelType()),
				cel.Variable(compiler.GlobalContextKey, globalcontext.ContextType),
				cel.Variable(compiler.HttpKey, http.ContextType),
				cel.Variable(compiler.ImageDataKey, imagedata.ContextType),
				cel.Variable(compiler.ImagesKey, image.ImageType),
				cel.Variable(compiler.ResourceKey, resource.ContextType),
				cel.Types(compiler.NamespaceType.CelType()),
				cel.Types(compiler.RequestType.CelType()),
				globalcontext.Lib(),
				http.Lib(),
				image.Lib(),
				imagedata.Lib(),
				resource.Lib(),
				user.Lib(),
			},
		},
	)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}

	compositedCompiler, err := plugincel.NewCompositedCompiler(extendedEnvSet)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}

	optionsVars := plugincel.OptionalVariableDeclarations{
		HasParams:     false,
		HasAuthorizer: false,
		HasPatchTypes: true,
		StrictCost:    true,
	}

	if policy.Spec.Variables != nil {
		compositedCompiler.CompileAndStoreVariables(convertVariables(policy.Spec.Variables), optionsVars, environment.StoredExpressions)
	}

	// Compile match conditions and collect errors
	var matcher matchconditions.Matcher = nil
	matchConditions := policy.Spec.MatchConditions
	if len(matchConditions) > 0 {
		matchExpressionAccessors := make([]plugincel.ExpressionAccessor, len(matchConditions))
		for i := range matchConditions {
			matchExpressionAccessors[i] = (*matchconditions.MatchCondition)(&matchConditions[i])
		}

		evaluator := compositedCompiler.CompileCondition(matchExpressionAccessors, optionsVars, environment.StoredExpressions)
		for _, err := range evaluator.CompilationErrors() {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec").Child("matchConditions"),
				matchConditions[0].Expression,
				err.Error(),
			))
		}

		failurePolicy := policy.GetFailurePolicy()
		matcher = matchconditions.NewMatcher(
			evaluator,
			&failurePolicy,
			"policy", "validate", policy.Name)
	}

	compiledExceptions := make([]compiler.Exception, 0, len(exceptions))
	for _, polex := range exceptions {
		polexMatchConditions, errs := compiler.CompileMatchConditions(field.NewPath("spec").Child("matchConditions"), extendedEnvSet.StoredExpressionsEnv(), polex.Spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}

		compiledExceptions = append(compiledExceptions, compiler.Exception{
			Exception:       polex,
			MatchConditions: polexMatchConditions,
		})
	}

	var patchers []patch.Patcher
	patchOptions := optionsVars
	patchOptions.HasPatchTypes = true
	for _, m := range policy.Spec.Mutations {
		switch m.PatchType {
		case admissionregistrationv1alpha1.PatchTypeJSONPatch:
			if m.JSONPatch != nil {
				accessor := &patch.JSONPatchCondition{Expression: m.JSONPatch.Expression}
				compileResult := compositedCompiler.CompileMutatingEvaluator(accessor, patchOptions, environment.StoredExpressions)
				patchers = append(patchers, patch.NewJSONPatcher(compileResult))
			}
		case admissionregistrationv1alpha1.PatchTypeApplyConfiguration:
			if m.ApplyConfiguration != nil {
				accessor := &patch.ApplyConfigurationCondition{Expression: m.ApplyConfiguration.Expression}
				compileResult := compositedCompiler.CompileMutatingEvaluator(accessor, patchOptions, environment.StoredExpressions)
				patchers = append(patchers, patch.NewApplyConfigurationPatcher(compileResult))
			}
		}
	}
	return &Policy{
		evaluator:  mutating.PolicyEvaluator{Matcher: matcher, Mutators: patchers, CompositionEnv: compositedCompiler.CompositionEnv},
		exceptions: compiledExceptions,
	}, allErrs
}
