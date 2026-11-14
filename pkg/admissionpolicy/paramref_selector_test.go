package admissionpolicy

import (
	"errors"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const invalidSelectorSentinel = "invalid paramRef.selector"

const (
	policyKindVAP = "vap"
	policyKindMAP = "map"
)

// findVAPRule returns the rule recorded for a validating policy binding, so a
// test does not depend on where outside the binding loop the rule was appended.
func findVAPRule(t *testing.T, rules []engineapi.RuleResponse, binding string) *engineapi.RuleResponse {
	t.Helper()

	for i := range rules {
		rule := &rules[i]
		if b := rule.ValidatingAdmissionPolicyBinding(); b != nil && b.GetName() == binding {
			return rule
		}
	}
	t.Fatalf("no rule for validatingadmissionpolicybinding %q", binding)
	return nil
}

// findMAPRule returns the rule recorded for a mutating policy binding.
func findMAPRule(t *testing.T, rules []engineapi.RuleResponse, binding string) *engineapi.RuleResponse {
	t.Helper()

	for i := range rules {
		rule := &rules[i]
		if b := rule.MutatingAdmissionPolicyBinding(); b != nil && b.GetName() == binding {
			return rule
		}
	}
	t.Fatalf("no rule for mutatingadmissionpolicybinding %q", binding)
	return nil
}

func invalidLabelSelector() *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "env",
				Operator: "Bogus",
				Values:   []string{"prod"},
			},
		},
	}
}

func matchEnvSelector(env string) *metav1.LabelSelector {
	return &metav1.LabelSelector{MatchLabels: map[string]string{"env": env}}
}

// paramRefSpec describes one binding's paramRef for the evaluation matrix.
// Setting none leaves the binding without a paramRef.
type paramRefSpec struct {
	none         bool
	name         string
	namespace    string
	selector     *metav1.LabelSelector
	denyNotFound bool
}

func (s paramRefSpec) toVAP() *admissionregistrationv1.ParamRef {
	ref := &admissionregistrationv1.ParamRef{Name: s.name, Namespace: s.namespace, Selector: s.selector}
	if s.denyNotFound {
		action := admissionregistrationv1.DenyAction
		ref.ParameterNotFoundAction = &action
	}
	return ref
}

func (s paramRefSpec) toMAP() *admissionregistrationv1beta1.ParamRef {
	ref := &admissionregistrationv1beta1.ParamRef{Name: s.name, Namespace: s.namespace, Selector: s.selector}
	if s.denyNotFound {
		action := admissionregistrationv1beta1.DenyAction
		ref.ParameterNotFoundAction = &action
	}
	return ref
}

func newParam(namespace string, labels map[string]string) *unstructured.Unstructured {
	param := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "replica-limits",
				"namespace": namespace,
			},
			"data": map[string]any{"maxReplicas": "5"},
		},
	}
	param.SetLabels(labels)
	return param
}

func newDeployment() unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "workload",
			"namespace": "default",
		},
		"spec": map[string]any{"replicas": int64(1)},
	}}
}

func newClientPathVAP() *admissionregistrationv1.ValidatingAdmissionPolicy {
	return &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "replica-limit"},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			ParamKind: &admissionregistrationv1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"},
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
							Resources:   []string{"deployments"},
						},
					},
				}},
			},
			Validations: []admissionregistrationv1.Validation{{Expression: "object.spec.replicas > 0"}},
		},
	}
}

func newClientPathMAP() *admissionregistrationv1beta1.MutatingAdmissionPolicy {
	return &admissionregistrationv1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "label-deployment"},
		Spec: admissionregistrationv1beta1.MutatingAdmissionPolicySpec{
			ParamKind: &admissionregistrationv1beta1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"},
			MatchConstraints: &admissionregistrationv1beta1.MatchResources{
				ResourceRules: []admissionregistrationv1beta1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1beta1.RuleWithOperations{
						Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create},
						Rule: admissionregistrationv1beta1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
							Resources:   []string{"deployments"},
						},
					},
				}},
			},
			Mutations: []admissionregistrationv1beta1.Mutation{
				{
					PatchType: admissionregistrationv1beta1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &admissionregistrationv1beta1.ApplyConfiguration{
						Expression: `Object{ metadata: Object.metadata{ labels: {"applied": "yes"} } }`,
					},
				},
			},
		},
	}
}

// bindingOrders places the failing binding first and last, so a rule attributed to
// bindings[0] instead of bindings[i] cannot pass both cases and continuing past
// the failing binding is proven where it actually matters.
func bindingOrders(t *testing.T, failing, matching string) []struct {
	name        string
	failedFirst bool
} {
	t.Helper()
	return []struct {
		name        string
		failedFirst bool
	}{
		{name: failing + " before " + matching, failedFirst: true},
		{name: failing + " after " + matching, failedFirst: false},
	}
}

// TestParamRefSelectorClientPath covers the cluster-backed branch, which takes a
// different route through CollectParams than the offline one.
func TestParamRefSelectorClientPath(t *testing.T) {
	t.Parallel()

	invalid := paramRefSpec{selector: invalidLabelSelector()}

	// a valid selector that matches none of the seeded parameters
	unmatched := newParam("default", map[string]string{"env": "testing"})

	t.Run("vap invalid selector", func(t *testing.T) {
		t.Parallel()

		policy := newClientPathVAP()
		bindings := []admissionregistrationv1.ValidatingAdmissionPolicyBinding{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "bad"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: policy.Name,
					ParamRef:   invalid.toVAP(),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "no-params"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: policy.Name,
				},
			},
		}
		data := engineapi.NewValidatingAdmissionPolicyData(policy, bindings...)

		resource := newDeployment()
		response, err := Validate(
			data,
			resource,
			resource.GroupVersionKind(),
			schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			nil,
			newTestDClient(t),
			nil,
			false,
		)

		assert.Assert(t, err != nil, "expected an error")
		assert.Assert(t, strings.Contains(err.Error(), invalidSelectorSentinel), "unexpected error: %v", err)
		assert.Assert(t, len(response.PolicyResponse.Rules) == 2, "expected two rule responses, got %d", len(response.PolicyResponse.Rules))
		assert.Equal(t, findVAPRule(t, response.PolicyResponse.Rules, "no-params").Status(), engineapi.RuleStatusPass)
		rule := findVAPRule(t, response.PolicyResponse.Rules, "bad")
		assert.Equal(t, rule.Status(), engineapi.RuleStatusError)
		assert.Equal(t, rule.RuleType(), engineapi.Validation)
	})

	t.Run("vap deny when no params match", func(t *testing.T) {
		t.Parallel()

		for _, order := range bindingOrders(t, "deny", "matched") {
			t.Run(order.name, func(t *testing.T) {
				policy := newClientPathVAP()
				denyAll := admissionregistrationv1.DenyAction
				deny := admissionregistrationv1.ValidatingAdmissionPolicyBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "deny"},
					Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
						PolicyName: policy.Name,
						ParamRef: &admissionregistrationv1.ParamRef{
							Selector:                matchEnvSelector("prod"),
							ParameterNotFoundAction: &denyAll,
						},
					},
				}
				matched := admissionregistrationv1.ValidatingAdmissionPolicyBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "matched"},
					Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
						PolicyName: policy.Name,
						ParamRef: &admissionregistrationv1.ParamRef{
							Selector: matchEnvSelector("testing"),
						},
					},
				}
				bindings := []admissionregistrationv1.ValidatingAdmissionPolicyBinding{matched, deny}
				if order.failedFirst {
					bindings = []admissionregistrationv1.ValidatingAdmissionPolicyBinding{deny, matched}
				}
				data := engineapi.NewValidatingAdmissionPolicyData(policy, bindings...)

				resource := newDeployment()
				response, err := Validate(
					data,
					resource,
					resource.GroupVersionKind(),
					schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
					nil,
					newParamsDClient(t, unmatched),
					nil,
					false,
				)

				// the deny binding must not abort the loop: the paramRef-bearing
				// binding is still collected and evaluated, and the deny is reported
				// alongside that binding's rule
				assert.Assert(t, errors.Is(err, errNoParamsFound), "unexpected error: %v", err)
				assert.Assert(t, len(response.PolicyResponse.Rules) == 2, "expected two rule responses, got %d", len(response.PolicyResponse.Rules))
				denied := findVAPRule(t, response.PolicyResponse.Rules, "deny")
				assert.Equal(t, denied.Status(), engineapi.RuleStatusError)
				assert.Equal(t, denied.RuleType(), engineapi.Validation)
				assert.Equal(t, findVAPRule(t, response.PolicyResponse.Rules, "matched").Status(), engineapi.RuleStatusPass)
			})
		}
	})

	t.Run("map invalid selector", func(t *testing.T) {
		t.Parallel()

		policy := newClientPathMAP()
		bindings := []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "bad"},
				Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
					PolicyName: policy.Name,
					ParamRef:   invalid.toMAP(),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "no-params"},
				Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
					PolicyName: policy.Name,
				},
			},
		}
		data := engineapi.NewMutatingAdmissionPolicyData(policy, bindings...)

		resource := newDeployment()
		response, err := Mutate(
			data,
			resource,
			resource.GroupVersionKind(),
			schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			nil,
			newTestDClient(t),
			nil,
			false,
			false,
		)

		assert.Assert(t, err != nil, "expected an error")
		assert.Assert(t, strings.Contains(err.Error(), invalidSelectorSentinel), "unexpected error: %v", err)
		assert.Assert(t, len(response.PolicyResponse.Rules) == 2, "expected two rule responses, got %d", len(response.PolicyResponse.Rules))
		assert.Equal(t, findMAPRule(t, response.PolicyResponse.Rules, "no-params").Status(), engineapi.RuleStatusPass)
		rule := findMAPRule(t, response.PolicyResponse.Rules, "bad")
		assert.Equal(t, rule.Status(), engineapi.RuleStatusError)
		assert.Equal(t, rule.RuleType(), engineapi.Mutation)
		assert.Equal(t, response.PatchedResource.GetLabels()["applied"], "yes")
	})

	t.Run("map deny when no params match", func(t *testing.T) {
		t.Parallel()

		for _, order := range bindingOrders(t, "deny", "matched") {
			t.Run(order.name, func(t *testing.T) {
				policy := newClientPathMAP()
				denyAll := admissionregistrationv1beta1.DenyAction
				deny := admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "deny"},
					Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
						PolicyName: policy.Name,
						ParamRef: &admissionregistrationv1beta1.ParamRef{
							Selector:                matchEnvSelector("prod"),
							ParameterNotFoundAction: &denyAll,
						},
					},
				}
				matched := admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "matched"},
					Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
						PolicyName: policy.Name,
						ParamRef: &admissionregistrationv1beta1.ParamRef{
							Selector: matchEnvSelector("testing"),
						},
					},
				}
				bindings := []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{matched, deny}
				if order.failedFirst {
					bindings = []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{deny, matched}
				}
				data := engineapi.NewMutatingAdmissionPolicyData(policy, bindings...)

				resource := newDeployment()
				response, err := Mutate(
					data,
					resource,
					resource.GroupVersionKind(),
					schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
					nil,
					newParamsDClient(t, unmatched),
					nil,
					false,
					false,
				)

				// the deny binding must not abort the loop: the paramRef-bearing
				// binding is still collected and evaluated, and the deny is reported
				// alongside that binding's rule
				assert.Assert(t, errors.Is(err, errNoParamsFound), "unexpected error: %v", err)
				assert.Assert(t, len(response.PolicyResponse.Rules) == 2, "expected two rule responses, got %d", len(response.PolicyResponse.Rules))
				denied := findMAPRule(t, response.PolicyResponse.Rules, "deny")
				assert.Equal(t, denied.Status(), engineapi.RuleStatusError)
				assert.Equal(t, denied.RuleType(), engineapi.Mutation)
				assert.Equal(t, findMAPRule(t, response.PolicyResponse.Rules, "matched").Status(), engineapi.RuleStatusPass)
				assert.Equal(t, response.PatchedResource.GetLabels()["applied"], "yes")
			})
		}
	})
}

func newTestDClient(t *testing.T) dclient.Interface {
	t.Helper()

	return newParamsDClient(t)
}

// newParamsDClient returns a fake client whose parameter listing contains params.
// The list kind has to be registered: the fake dynamic client panics on LIST
// otherwise, and it filters the entries by label selector the way the API server
// does, so a selector that matches none of them yields no parameters.
func newParamsDClient(t *testing.T, params ...runtime.Object) dclient.Interface {
	t.Helper()

	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			{Version: "v1", Resource: "configmaps"}: "ConfigMapList",
		},
		params...,
	)
	kube := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	return dclient.NewFakeClientWithDisco(dyn, kube, dclient.NewFakeDiscoveryClient(nil))
}

func TestMatchesSelector(t *testing.T) {
	t.Parallel()

	param := func(namespace string, labels map[string]string) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "replica-limits",
					"namespace": namespace,
				},
			},
		}
		obj.SetLabels(labels)
		return obj
	}

	tests := []struct {
		name    string
		obj     *unstructured.Unstructured
		ref     *admissionregistrationv1.ParamRef
		want    bool
		wantErr bool
	}{
		{
			name: "name only matches",
			obj:  param("default", nil),
			ref:  &admissionregistrationv1.ParamRef{Name: "replica-limits"},
			want: true,
		},
		{
			name: "name only does not match",
			obj:  param("default", nil),
			ref:  &admissionregistrationv1.ParamRef{Name: "other"},
			want: false,
		},
		{
			name: "namespace and name match",
			obj:  param("trusted", nil),
			ref:  &admissionregistrationv1.ParamRef{Name: "replica-limits", Namespace: "trusted"},
			want: true,
		},
		{
			name: "namespace matches but name does not",
			obj:  param("trusted", nil),
			ref:  &admissionregistrationv1.ParamRef{Name: "other", Namespace: "trusted"},
			want: false,
		},
		{
			name: "valid selector matches labels",
			obj:  param("default", map[string]string{"env": "prod"}),
			ref:  &admissionregistrationv1.ParamRef{Selector: matchEnvSelector("prod")},
			want: true,
		},
		{
			name: "valid selector does not match labels",
			obj:  param("default", map[string]string{"env": "testing"}),
			ref:  &admissionregistrationv1.ParamRef{Selector: matchEnvSelector("prod")},
			want: false,
		},
		{
			name: "empty selector matches everything",
			obj:  param("default", nil),
			ref:  &admissionregistrationv1.ParamRef{Selector: &metav1.LabelSelector{}},
			want: true,
		},
		{
			name: "cluster scoped param matches by name",
			obj:  param("", nil),
			ref:  &admissionregistrationv1.ParamRef{Name: "replica-limits"},
			want: true,
		},
		{
			name:    "invalid selector returns an error",
			obj:     param("default", map[string]string{"env": "prod"}),
			ref:     &admissionregistrationv1.ParamRef{Selector: invalidLabelSelector()},
			wantErr: true,
		},
		{
			name: "neither name nor selector set",
			obj:  param("default", nil),
			ref:  &admissionregistrationv1.ParamRef{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := matchesSelector(tt.obj, tt.ref)
			if tt.wantErr {
				assert.Assert(t, err != nil, "expected an error")
				assert.Assert(t, strings.Contains(err.Error(), invalidSelectorSentinel), "unexpected error: %v", err)
				assert.Assert(t, !got, "expected no match")
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}

func runVAPEvaluation(t *testing.T, refs []paramRefSpec, params []*unstructured.Unstructured) (engineapi.EngineResponse, error) {
	t.Helper()

	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "replica-limit"},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			ParamKind: &admissionregistrationv1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"},
			// independent of params so a parameter failure cannot be mistaken for
			// a validation result
			Validations: []admissionregistrationv1.Validation{{Expression: "object.spec.replicas > 0"}},
		},
	}
	bindings := make([]admissionregistrationv1.ValidatingAdmissionPolicyBinding, 0, len(refs))
	for i, ref := range refs {
		binding := admissionregistrationv1.ValidatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "binding-" + string(rune('a'+i))},
			Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
				PolicyName: policy.Name,
			},
		}
		if !ref.none {
			binding.Spec.ParamRef = ref.toVAP()
		}
		bindings = append(bindings, binding)
	}
	data := engineapi.NewValidatingAdmissionPolicyData(policy, bindings...)
	for _, param := range params {
		data.AddParam(param)
	}

	resource := newDeployment()
	return Validate(
		data,
		resource,
		resource.GroupVersionKind(),
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		nil,
		nil,
		nil,
		true,
	)
}

func runMAPEvaluation(t *testing.T, refs []paramRefSpec, params []*unstructured.Unstructured) (engineapi.EngineResponse, error) {
	t.Helper()

	policy := &admissionregistrationv1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "label-deployment"},
		Spec: admissionregistrationv1beta1.MutatingAdmissionPolicySpec{
			ParamKind: &admissionregistrationv1beta1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"},
			Mutations: []admissionregistrationv1beta1.Mutation{
				{
					PatchType: admissionregistrationv1beta1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &admissionregistrationv1beta1.ApplyConfiguration{
						Expression: `Object{ metadata: Object.metadata{ labels: {"applied": "yes"} } }`,
					},
				},
			},
		},
	}
	bindings := make([]admissionregistrationv1beta1.MutatingAdmissionPolicyBinding, 0, len(refs))
	for i, ref := range refs {
		binding := admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "binding-" + string(rune('a'+i))},
			Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
				PolicyName: policy.Name,
			},
		}
		if !ref.none {
			binding.Spec.ParamRef = ref.toMAP()
		}
		bindings = append(bindings, binding)
	}
	data := engineapi.NewMutatingAdmissionPolicyData(policy, bindings...)
	for _, param := range params {
		data.AddParam(param)
	}

	resource := newDeployment()
	return Mutate(
		data,
		resource,
		resource.GroupVersionKind(),
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		nil,
		nil,
		nil,
		true,
		false,
	)
}

// TestParamRefSelectorEvaluation is the policy-evaluation matrix for invalid,
// missing and matched parameter references on both policy types.
func TestParamRefSelectorEvaluation(t *testing.T) {
	t.Parallel()

	invalid := paramRefSpec{selector: invalidLabelSelector()}
	missing := paramRefSpec{selector: matchEnvSelector("other")}
	missingDeny := paramRefSpec{selector: matchEnvSelector("other"), denyNotFound: true}
	byName := paramRefSpec{name: "replica-limits", namespace: "default"}
	matched := newParam("default", map[string]string{"env": "prod"})

	tests := []struct {
		name          string
		policyKind    string
		refs          []paramRefSpec
		params        []*unstructured.Unstructured
		wantStatuses  []engineapi.RuleStatus
		wantErr       bool
		wantErrSubstr string
		wantPatched   bool
	}{
		{
			name:          "vap invalid selector with a parameter",
			policyKind:    policyKindVAP,
			refs:          []paramRefSpec{invalid},
			params:        []*unstructured.Unstructured{matched},
			wantStatuses:  []engineapi.RuleStatus{engineapi.RuleStatusError},
			wantErr:       true,
			wantErrSubstr: "invalid paramRef.selector",
		},
		{
			name:          "vap invalid selector without parameters",
			policyKind:    policyKindVAP,
			refs:          []paramRefSpec{invalid},
			wantStatuses:  []engineapi.RuleStatus{engineapi.RuleStatusError},
			wantErr:       true,
			wantErrSubstr: "invalid paramRef.selector",
		},
		{
			name:         "vap selector matches nothing and allows",
			policyKind:   policyKindVAP,
			refs:         []paramRefSpec{missing},
			params:       []*unstructured.Unstructured{matched},
			wantStatuses: nil,
		},
		{
			name:          "vap selector matches nothing and denies",
			policyKind:    policyKindVAP,
			refs:          []paramRefSpec{missingDeny},
			params:        []*unstructured.Unstructured{matched},
			wantStatuses:  []engineapi.RuleStatus{engineapi.RuleStatusError},
			wantErr:       true,
			wantErrSubstr: "no params found",
		},
		{
			name:          "vap valid binding before the invalid one",
			policyKind:    policyKindVAP,
			refs:          []paramRefSpec{byName, invalid},
			params:        []*unstructured.Unstructured{matched},
			wantStatuses:  []engineapi.RuleStatus{engineapi.RuleStatusPass, engineapi.RuleStatusError},
			wantErr:       true,
			wantErrSubstr: "invalid paramRef.selector",
		},
		{
			name:          "vap invalid binding before the valid one",
			policyKind:    policyKindVAP,
			refs:          []paramRefSpec{invalid, byName},
			params:        []*unstructured.Unstructured{matched},
			wantStatuses:  []engineapi.RuleStatus{engineapi.RuleStatusPass, engineapi.RuleStatusError},
			wantErr:       true,
			wantErrSubstr: "invalid paramRef.selector",
		},
		{
			name:          "map invalid selector with a parameter",
			policyKind:    policyKindMAP,
			refs:          []paramRefSpec{invalid},
			params:        []*unstructured.Unstructured{matched},
			wantStatuses:  []engineapi.RuleStatus{engineapi.RuleStatusError},
			wantErr:       true,
			wantErrSubstr: "invalid paramRef.selector",
		},
		{
			name:          "map invalid selector without parameters",
			policyKind:    policyKindMAP,
			refs:          []paramRefSpec{invalid},
			wantStatuses:  []engineapi.RuleStatus{engineapi.RuleStatusError},
			wantErr:       true,
			wantErrSubstr: "invalid paramRef.selector",
		},
		{
			name:         "map selector matches nothing and allows",
			policyKind:   policyKindMAP,
			refs:         []paramRefSpec{missing},
			params:       []*unstructured.Unstructured{matched},
			wantStatuses: nil,
		},
		{
			name:          "map selector matches nothing and denies",
			policyKind:    policyKindMAP,
			refs:          []paramRefSpec{missingDeny},
			params:        []*unstructured.Unstructured{matched},
			wantStatuses:  []engineapi.RuleStatus{engineapi.RuleStatusError},
			wantErr:       true,
			wantErrSubstr: "no params found",
		},
		{
			name:          "map valid binding before the invalid one",
			policyKind:    policyKindMAP,
			refs:          []paramRefSpec{byName, invalid},
			params:        []*unstructured.Unstructured{matched},
			wantStatuses:  []engineapi.RuleStatus{engineapi.RuleStatusPass, engineapi.RuleStatusError},
			wantErr:       true,
			wantErrSubstr: "invalid paramRef.selector",
			wantPatched:   true,
		},
		{
			name:          "map invalid binding before the valid one",
			policyKind:    policyKindMAP,
			refs:          []paramRefSpec{invalid, byName},
			params:        []*unstructured.Unstructured{matched},
			wantStatuses:  []engineapi.RuleStatus{engineapi.RuleStatusPass, engineapi.RuleStatusError},
			wantErr:       true,
			wantErrSubstr: "invalid paramRef.selector",
			wantPatched:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				response engineapi.EngineResponse
				err      error
			)
			switch tt.policyKind {
			case policyKindVAP:
				response, err = runVAPEvaluation(t, tt.refs, tt.params)
			case policyKindMAP:
				response, err = runMAPEvaluation(t, tt.refs, tt.params)
			default:
				t.Fatalf("unknown policy kind %q", tt.policyKind)
			}

			if tt.wantErr {
				assert.Assert(t, err != nil, "expected an error")
				assert.Assert(t, strings.Contains(err.Error(), tt.wantErrSubstr), "unexpected error: %v", err)
			} else {
				assert.NilError(t, err)
			}

			var statuses []engineapi.RuleStatus
			for _, rule := range response.PolicyResponse.Rules {
				statuses = append(statuses, rule.Status())
			}
			assert.DeepEqual(t, statuses, tt.wantStatuses)

			if tt.wantPatched {
				assert.Equal(t, response.PatchedResource.GetLabels()["applied"], "yes")
			}
		})
	}
}
