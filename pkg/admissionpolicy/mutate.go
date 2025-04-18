package admissionpolicy

import (
	"context"
	"strings"
	"time"

	celutils "github.com/kyverno/kyverno/pkg/cel/utils"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
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

func MutateResource(
	policy admissionregistrationv1alpha1.MutatingAdmissionPolicy,
	binding *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding,
	resource unstructured.Unstructured,
) (engineapi.EngineResponse, error) {
	startTime := time.Now()

	engineResponse := engineapi.NewEngineResponse(resource, engineapi.NewMutatingAdmissionPolicy(&policy), nil)
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
	variables := make([]admissionregistrationv1.Variable, 0, len(policy.Spec.Variables))
	for _, v := range policy.Spec.Variables {
		variables = append(variables, admissionregistrationv1.Variable(v))
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
			ruleResp := engineapi.RuleSkip(policy.GetName(), engineapi.Mutation, "match conditions not met", nil)
			policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
			return engineResponse.WithPolicyResponse(policyResp), nil
		}
	}
	// compile mutations
	patchers := compiler.CompileMutations(optionalVars)
	if len(patchers) == 0 {
		ruleResp := engineapi.RuleSkip(policy.GetName(), engineapi.Mutation, "mutation returned no patchers", nil)
		policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
		return engineResponse.WithPolicyResponse(policyResp), nil
	}
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
		original := versionedAttributes.VersionedObject

		newVersionedObject, err := patcher.Patch(context.TODO(), patchRequest, celconfig.RuntimeCELCostBudget)
		if err != nil {
			ruleResp := engineapi.RuleError(policy.GetName(), engineapi.Mutation, err.Error(), nil, nil)
			policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
			return engineResponse.WithPolicyResponse(policyResp), nil
		}

		// check if mutation actually changed anything
		if equality.Semantic.DeepEqual(original, newVersionedObject) {
			ruleResp := engineapi.RuleSkip(policy.GetName(), engineapi.Mutation, "mutation had no effect", nil)
			policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
			return engineResponse.WithPolicyResponse(policyResp), nil
		}

		versionedAttributes.VersionedObject = newVersionedObject

	}

	patchedResource, err := celutils.ConvertObjectToUnstructured(versionedAttributes.VersionedObject)
	if err != nil {
		ruleResp := engineapi.RuleError(policy.GetName(), engineapi.Mutation, err.Error(), nil, nil)
		policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
		return engineResponse.WithPolicyResponse(policyResp), nil
	}

	ruleResp := engineapi.RulePass(policy.GetName(), engineapi.Mutation, "", nil)
	if binding != nil {
		ruleResp = ruleResp.WithMutatingBinding(binding)
	}
	policyResp.Add(engineapi.NewExecutionStats(startTime, time.Now()), *ruleResp)
	engineResponse = engineResponse.
		WithPatchedResource(*patchedResource).
		WithPolicyResponse(policyResp)

	return engineResponse, nil
}

// MutatingPolicyData holds a MAP and its associated MAPBs
type MutatingPolicyData struct {
	definition admissionregistrationv1alpha1.MutatingAdmissionPolicy
	bindings   []admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding
}

// NewMutatingPolicyData initializes a MAP wrapper with no bindings
func NewMutatingPolicyData(p admissionregistrationv1alpha1.MutatingAdmissionPolicy) MutatingPolicyData {
	return MutatingPolicyData{definition: p}
}

// AddBinding appends a MAPB to the policy data
func (m *MutatingPolicyData) AddBinding(b admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) {
	m.bindings = append(m.bindings, b)
}

func Mutate(data MutatingPolicyData, resource unstructured.Unstructured) (engineapi.EngineResponse, error) {
	var response engineapi.EngineResponse
	for i, _ := range data.bindings {
		resp, err := MutateResource(data.definition, &data.bindings[i], resource)
		if err != nil {
			return response, err
		}
		if !resp.IsEmpty() {
			return resp, nil
		}
	}
	// fallback: no bindings matched? call with nil
	return MutateResource(data.definition, nil, resource)
}
