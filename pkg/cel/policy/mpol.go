package policy

import (
	"context"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	contextlib "github.com/kyverno/kyverno/pkg/cel/libs/context"
	celutils "github.com/kyverno/kyverno/pkg/cel/utils"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	managedfields "k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	plugincel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	matchconditions "k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/cel/environment"
)

type compiledMpol struct {
	matcher  matchconditions.Matcher
	patchers []patch.Patcher
}

func (c *compiledMpol) Evaluate(
	ctx context.Context,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	context contextlib.ContextInterface,
	autogenIndex int,
) (*EvaluationResult, error) {
	resource := request.Object.Object
	gvk := resource.GetObjectKind().GroupVersionKind()
	gvr := schema.GroupVersionResource(*request.RequestResource)
	a := admission.NewAttributesRecord(resource.DeepCopyObject(),
		nil,
		gvk,
		request.Namespace,
		request.Name,
		gvr,
		"",
		admission.Operation(request.Operation),
		nil,
		false,
		attr.GetUserInfo(),
	)

	versionedAttributes, _ := admission.NewVersionedAttributes(a, a.GetKind(), nil)
	o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())
	matchResults := c.matcher.Match(ctx, versionedAttributes, nil, nil)
	if !matchResults.Matches {
		return nil, nil
	}

	ns := namespace.(*corev1.Namespace)
	for _, patcher := range c.patchers {
		patchRequest := patch.Request{
			MatchedResource:     gvr,
			VersionedAttributes: versionedAttributes,
			ObjectInterfaces:    o,
			OptionalVariables:   plugincel.OptionalVariableBindings{VersionedParams: nil, Authorizer: nil},
			Namespace:           ns,
			TypeConverter:       managedfields.NewDeducedTypeConverter(),
		}
		newVersionedObject, err := patcher.Patch(ctx, patchRequest, celconfig.RuntimeCELCostBudget)
		if err != nil {
			return &EvaluationResult{Error: err}, nil
		}
		versionedAttributes.Dirty = true
		versionedAttributes.VersionedObject = newVersionedObject
	}
	patchedResource, err := celutils.ConvertObjectToUnstructured(versionedAttributes.VersionedObject)
	if err != nil {
		return &EvaluationResult{Error: err}, nil
	}

	return &EvaluationResult{PatchedResource: *patchedResource}, nil
}

func (c *compiler) CompileMutating(policy *policiesv1alpha1.MutatingPolicy, exceptions []policiesv1alpha1.CELPolicyException) (CompiledPolicy, field.ErrorList) {
	var allErrs field.ErrorList

	opts := plugincel.OptionalVariableDeclarations{HasParams: policy.Spec.ParamKind != nil, StrictCost: true, HasAuthorizer: true}
	compiler, err := plugincel.NewCompositedCompiler(environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion(), true))
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}

	patchOptions := opts
	patchOptions.HasPatchTypes = true

	compiler.CompileAndStoreVariables(convertv1alpha1Variables(policy.Spec.Variables), opts, environment.StoredExpressions)

	var matcher matchconditions.Matcher = nil
	matchConditions := policy.Spec.MatchConditions
	if len(matchConditions) > 0 {
		matchExpressionAccessors := make([]plugincel.ExpressionAccessor, len(matchConditions))
		for i := range matchConditions {
			matchExpressionAccessors[i] = (*matchconditions.MatchCondition)(&matchConditions[i])
		}
		failurePolicy := policy.GetFailurePolicy()
		matcher = matchconditions.NewMatcher(compiler.CompileCondition(matchExpressionAccessors, opts, environment.StoredExpressions), &failurePolicy, "policy", "mutate", policy.Name)
	}

	var patchers []patch.Patcher
	for _, m := range policy.GetSpec().Mutations {
		switch m.PatchType {
		case admissionregistrationv1alpha1.PatchTypeJSONPatch:
			if m.JSONPatch != nil {
				accessor := &patch.JSONPatchCondition{Expression: m.JSONPatch.Expression}
				compileResult := compiler.CompileMutatingEvaluator(accessor, patchOptions, environment.StoredExpressions)
				patchers = append(patchers, patch.NewJSONPatcher(compileResult))
			}
		case admissionregistrationv1alpha1.PatchTypeApplyConfiguration:
			if m.ApplyConfiguration != nil {
				accessor := &patch.ApplyConfigurationCondition{Expression: m.ApplyConfiguration.Expression}
				compileResult := compiler.CompileMutatingEvaluator(accessor, patchOptions, environment.StoredExpressions)
				patchers = append(patchers, patch.NewApplyConfigurationPatcher(compileResult))
			}
		}
	}

	return &compiledMpol{
		matcher:  matcher,
		patchers: patchers,
	}, nil
}

func convertv1alpha1Variables(variables []admissionregistrationv1.Variable) []plugincel.NamedExpressionAccessor {
	namedExpressions := make([]plugincel.NamedExpressionAccessor, len(variables))
	for i, variable := range variables {
		namedExpressions[i] = &mutating.Variable{Name: variable.Name, Expression: variable.Expression}
	}
	return namedExpressions
}
