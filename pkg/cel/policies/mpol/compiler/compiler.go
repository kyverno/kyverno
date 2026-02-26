package compiler

import (
	"context"
	"fmt"

	cel "github.com/google/cel-go/cel"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	compiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/sdk/cel/libs/globalcontext"
	"github.com/kyverno/sdk/cel/libs/http"
	"github.com/kyverno/sdk/cel/libs/image"
	"github.com/kyverno/sdk/cel/libs/imagedata"
	"github.com/kyverno/sdk/cel/libs/json"
	"github.com/kyverno/sdk/cel/libs/math"
	"github.com/kyverno/sdk/cel/libs/random"
	"github.com/kyverno/sdk/cel/libs/resource"
	"github.com/kyverno/sdk/cel/libs/time"
	"github.com/kyverno/sdk/cel/libs/transform"
	"github.com/kyverno/sdk/cel/libs/user"
	"github.com/kyverno/sdk/cel/libs/x509"
	"github.com/kyverno/sdk/cel/libs/yaml"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/version"
	plugincel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating"
	patch "k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	matchconditions "k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	environment "k8s.io/apiserver/pkg/cel/environment"
)

var (
	mpolCompilerVersion = version.MajorMinor(2, 0)
	compileError        = "mutating policy compiler " + mpolCompilerVersion.String() + " error: %s"
)

type Compiler interface {
	Compile(policy policiesv1beta1.MutatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*Policy, field.ErrorList)
}

func NewCompiler() Compiler {
	return &compilerImpl{}
}

type compilerImpl struct{}

func (c *compilerImpl) Compile(policy policiesv1beta1.MutatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*Policy, field.ErrorList) {
	var allErrs field.ErrorList
	libCtx := libs.GetLibsCtx()

	baseEnvSet := environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion())
	extendedEnvSet, err := baseEnvSet.Extend(
		environment.VersionedOptions{
			IntroducedVersion: version.MajorMinor(1, 0),
			EnvOptions: []cel.EnvOption{
				cel.Variable(compiler.NamespaceObjectKey, compiler.NamespaceType.CelType()),
				cel.Variable(compiler.ObjectKey, cel.DynType),
				cel.Variable(compiler.OldObjectKey, cel.DynType),
				cel.Variable(compiler.RequestKey, compiler.RequestType.CelType()),
				cel.Variable(compiler.ImagesKey, image.ImageType),
				cel.Types(compiler.NamespaceType.CelType()),
				cel.Types(compiler.RequestType.CelType()),
				globalcontext.Lib(globalcontext.Context{ContextInterface: libCtx}, globalcontext.Latest()),
				http.Lib(http.Context{ContextInterface: http.NewHTTP(nil)}, http.Latest()),
				image.Lib(image.Latest()),
				imagedata.Lib(imagedata.Context{ContextInterface: libCtx}, imagedata.Latest()),
				math.Lib(math.Latest()),
				resource.Lib(resource.Context{ContextInterface: libCtx}, policy.GetNamespace(), resource.Latest()),
				user.Lib(user.Latest()),
				json.Lib(&json.JsonImpl{}, json.Latest()),
				yaml.Lib(&yaml.YamlImpl{}, yaml.Latest()),
				random.Lib(random.Latest()),
				x509.Lib(x509.Latest()),
				time.Lib(time.Latest()),
				transform.Lib(transform.Latest()),
			},
		},
	)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, err)))
	}

	compositedCompiler, err := plugincel.NewCompositedCompiler(extendedEnvSet)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, err)))
	}

	optionsVars := plugincel.OptionalVariableDeclarations{
		HasParams:     false,
		HasAuthorizer: false,
		HasPatchTypes: true,
	}

	if policy.GetSpec().Variables != nil {
		compositedCompiler.CompileAndStoreVariables(ConvertVariables(policy.GetSpec().Variables), optionsVars, environment.StoredExpressions)
	}

	// Compile match conditions and collect errors
	var matcher matchconditions.Matcher = nil
	matchConditions := policy.GetSpec().MatchConditions
	if len(matchConditions) > 0 {
		matchExpressionAccessors := make([]plugincel.ExpressionAccessor, len(matchConditions))
		for i := range matchConditions {
			matchExpressionAccessors[i] = (*matchconditions.MatchCondition)(&matchConditions[i])
		}

		evaluator := compositedCompiler.ConditionCompiler.CompileCondition(matchExpressionAccessors, optionsVars, environment.StoredExpressions)
		for _, err := range evaluator.CompilationErrors() {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec").Child("matchConditions"),
				matchConditions[0].Expression,
				err.Error(),
			))
		}

		failurePolicy := policy.GetFailurePolicy(toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore())
		matcher = matchconditions.NewMatcher(
			evaluator,
			&failurePolicy,
			"policy", "validate", policy.GetName())
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
	for i, m := range policy.GetSpec().Mutations {
		switch m.PatchType {
		case admissionregistrationv1alpha1.PatchTypeJSONPatch:
			if m.JSONPatch != nil {
				accessor := &patch.JSONPatchCondition{Expression: m.JSONPatch.Expression}
				compileResult := compositedCompiler.CompileMutatingEvaluator(accessor, patchOptions, environment.StoredExpressions)
				for _, err := range compileResult.CompilationErrors() {
					allErrs = append(allErrs, field.Invalid(
						field.NewPath("spec").Child("mutations").Index(i).Child("jsonPatch"),
						m.JSONPatch.Expression,
						err.Error(),
					))
				}

				patchers = append(patchers, patch.NewJSONPatcher(compileResult))
			}
		case admissionregistrationv1alpha1.PatchTypeApplyConfiguration:
			if m.ApplyConfiguration != nil {
				accessor := &patch.ApplyConfigurationCondition{Expression: m.ApplyConfiguration.Expression}
				compileResult := compositedCompiler.CompileMutatingEvaluator(accessor, patchOptions, environment.StoredExpressions)
				for _, err := range compileResult.CompilationErrors() {
					allErrs = append(allErrs, field.Invalid(
						field.NewPath("spec").Child("mutations").Index(i).Child("applyConfiguration"),
						m.ApplyConfiguration.Expression,
						err.Error(),
					))
				}
				patchers = append(patchers, patch.NewApplyConfigurationPatcher(compileResult))
			}
		}
	}
	return &Policy{
		evaluator:        mutating.PolicyEvaluator{Matcher: matcher, Mutators: patchers, CompositionEnv: compositedCompiler.CompositionEnv},
		exceptions:       compiledExceptions,
		matchConstraints: policy.GetSpec().MatchConstraints,
	}, allErrs
}
