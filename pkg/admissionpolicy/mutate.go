package admissionpolicy

import (
	"context"
	"strings"
	"time"

	celutils "github.com/kyverno/kyverno/pkg/cel/utils"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

func mutateResource(
	policy admissionregistrationv1alpha1.MutatingAdmissionPolicy,
	resource unstructured.Unstructured,
) (engineapi.EngineResponse, error) {
	startTime := time.Now()

	engineResponse := engineapi.NewEngineResponse(resource, engineapi.NewMutatingAdmissionPolicy(policy), nil)
	policyResp := engineapi.NewPolicyResponse()

	gvk := resource.GroupVersionKind()
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: strings.ToLower(gvk.Kind) + "s",
	}

	var namespace *corev1.Namespace
	namespaceName := resource.GetNamespace()
	// Special case, the namespace object has the namespace of itself.
	// unset it if the incoming object is a namespace
	if gvk.Kind == "Namespace" && gvk.Version == "v1" && gvk.Group == "" {
		namespaceName = ""
	}

	if namespaceName != "" {
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		}
	}

	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, resource.GroupVersionKind(), resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
	versionedAttributes, _ := admission.NewVersionedAttributes(a, a.GetKind(), nil)
	o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())

	matchConditions := make([]admissionregistrationv1.MatchCondition, 0, len(policy.Spec.MatchConditions))
	for _, m := range policy.Spec.MatchConditions {
		matchConditions = append(matchConditions, admissionregistrationv1.MatchCondition(m))
	}
	variables := make([]admissionregistrationv1beta1.Variable, 0, len(policy.Spec.Variables))
	for _, v := range policy.Spec.Variables {
		variables = append(variables, admissionregistrationv1beta1.Variable(v))
	}

	// create compiler
	compiler, err := NewCompiler(matchConditions, variables)
	if err != nil {
		return engineResponse, err
	}
	compiler.WithMutations(policy.Spec.Mutations)

	optionalVars := cel.OptionalVariableDeclarations{
		HasParams:     false,
		HasAuthorizer: false,
		HasPatchTypes: true,
		StrictCost:    true,
	}
	// compile variables
	compiler.CompileVariables(optionalVars)

	var failPolicy admissionregistrationv1.FailurePolicyType
	if policy.Spec.FailurePolicy == nil {
		failPolicy = admissionregistrationv1.Fail
	} else {
		failPolicy = admissionregistrationv1.FailurePolicyType(*policy.Spec.FailurePolicy)
	}

	// compile matchers
	matcher := matchconditions.NewMatcher(compiler.CompileMatchConditions(optionalVars), &failPolicy, "policy", "mutate", policy.Name)
	if matcher != nil {
		matchResults := matcher.Match(context.TODO(), versionedAttributes, nil, nil)

		// if preconditions are not met, then skip mutations
		if !matchResults.Matches {
			return engineResponse, nil
		}
	}
	// compile mutations
	patchers := compiler.CompileMutations(optionalVars)
	// apply mutations
	for _, patcher := range patchers {
		patchRequest := patch.Request{
			MatchedResource:     gvr,
			VersionedAttributes: versionedAttributes,
			ObjectInterfaces:    o,
			OptionalVariables:   cel.OptionalVariableBindings{VersionedParams: nil, Authorizer: nil},
			Namespace:           namespace,
			TypeConverter:       managedfields.NewDeducedTypeConverter(),
		}
		newVersionedObject, err := patcher.Patch(context.TODO(), patchRequest, celconfig.RuntimeCELCostBudget)
		if err != nil {
			return engineResponse, nil
		}
		versionedAttributes.Dirty = true
		versionedAttributes.VersionedObject = newVersionedObject
	}
	patchedResource, err := celutils.ConvertObjectToUnstructured(versionedAttributes.VersionedObject)
	if err != nil {
		return engineResponse, err
	}

	ruleResp := engineapi.RulePass(policy.GetName(), engineapi.Mutation, "", nil)
	policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
	engineResponse = engineResponse.
		WithPatchedResource(*patchedResource).
		WithPolicyResponse(policyResp)

	return engineResponse, nil
}
