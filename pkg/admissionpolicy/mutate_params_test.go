package admissionpolicy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"gotest.tools/v3/assert"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

// namespacedDisco makes the fake discovery report every resource as namespaced,
// so that CollectParams treats a ConfigMap paramKind as namespace-scoped.
type namespacedDisco struct {
	dclient.IDiscovery
}

func (d namespacedDisco) FindResources(group, version, kind, subresource string) (map[dclient.TopLevelApiDescription]metav1.APIResource, error) {
	res, err := d.IDiscovery.FindResources(group, version, kind, subresource)
	if err != nil {
		return nil, err
	}
	for k, v := range res {
		v.Namespaced = true
		res[k] = v
	}
	return res, nil
}

func Test_Mutate_ClusterScopedResourceWithParamRef(t *testing.T) {
	rawPolicy := []byte(`{
  "apiVersion": "admissionregistration.k8s.io/v1beta1",
  "kind": "MutatingAdmissionPolicy",
  "metadata": {"name": "label-namespaces-with-params"},
  "spec": {
    "paramKind": {"apiVersion": "v1", "kind": "ConfigMap"},
    "matchConstraints": {
      "resourceRules": [{
        "apiGroups": [""],
        "apiVersions": ["v1"],
        "operations": ["CREATE"],
        "resources": ["namespaces"]
      }]
    },
    "failurePolicy": "Fail",
    "reinvocationPolicy": "Never",
    "mutations": [{
  "patchType": "ApplyConfiguration",
  "applyConfiguration": {
		"expression": "object.metadata.?labels[\"bug3\"].hasValue() ? Object{} : Object{ metadata: Object.metadata{ labels: {\"bug3\": params.data.key}}}"
	}
	}]
  }
}`)
	rawResource := []byte(`{
  "apiVersion": "v1",
  "kind": "Namespace",
  "metadata": {"name": "test-ns"}
}`)

	failPolicy := admissionregistrationv1beta1.Fail
	ignorePolicy := admissionregistrationv1beta1.Ignore
	allowAction := admissionregistrationv1beta1.AllowAction

	tests := []struct {
		name              string
		failurePolicy     *admissionregistrationv1beta1.FailurePolicyType
		paramRefNamespace string
		wantErr           string
		paramRefSelector  *metav1.LabelSelector
		notFoundAction    *admissionregistrationv1beta1.ParameterNotFoundActionType
		expectRuleError   bool
		wantLabel         string
	}{
		{name: "paramRef.namespace set", failurePolicy: &failPolicy, paramRefNamespace: "default", wantLabel: "value"},
		{name: "paramRef.namespace unset under failurePolicy Fail", failurePolicy: &failPolicy, paramRefNamespace: "", expectRuleError: true, wantErr: "cluster-scoped"},
		{name: "paramRef.namespace unset under failurePolicy nil defaults to Fail", failurePolicy: nil, paramRefNamespace: "", expectRuleError: true, wantErr: "cluster-scoped"},
		{name: "paramRef.namespace unset under failurePolicy Ignore", failurePolicy: &ignorePolicy, paramRefNamespace: ""},
		{name: "paramRef.selector matches no params with Allow", failurePolicy: &failPolicy, paramRefNamespace: "default", paramRefSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"does-not-match": "true"}}, notFoundAction: &allowAction},
		{name: "paramRef.selector matches no params with omitted action defaults to Deny", failurePolicy: &failPolicy, paramRefNamespace: "default", paramRefSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"does-not-match": "true"}}, notFoundAction: nil, expectRuleError: true, wantErr: "no params found"},
		{name: "paramRef.selector matches exactly 1 param preserves single object", failurePolicy: &failPolicy, paramRefNamespace: "default", paramRefSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "mutate"}}, wantLabel: "value"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var policy admissionregistrationv1beta1.MutatingAdmissionPolicy
			assert.NilError(t, json.Unmarshal(rawPolicy, &policy))
			policy.Spec.FailurePolicy = tc.failurePolicy

			binding := admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "binding"},
				Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
					PolicyName: policy.Name,
					ParamRef: &admissionregistrationv1beta1.ParamRef{
						Name:                    "map-params",
						Namespace:               tc.paramRefNamespace,
						ParameterNotFoundAction: tc.notFoundAction,
						Selector:                tc.paramRefSelector,
					},
				},
			}
			if tc.paramRefSelector != nil {
				binding.Spec.ParamRef.Name = ""
			}
			resource, err := kubeutils.BytesToUnstructured(rawResource)
			assert.NilError(t, err)
			gvk := resource.GroupVersionKind()
			gvr := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}

			// param ConfigMap seeded into the fake dynamic client
			cmGVK := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
			cm := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata":   map[string]any{"name": "map-params", "namespace": "default", "labels": map[string]any{"app": "mutate"}},
				"data":       map[string]any{"key": "value"},
			}}
			scheme := runtime.NewScheme()
			scheme.AddKnownTypeWithName(cmGVK, &unstructured.Unstructured{})
			scheme.AddKnownTypeWithName(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMapList"}, &unstructured.UnstructuredList{})
			dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
				scheme,
				map[schema.GroupVersionResource]string{{Version: "v1", Resource: "configmaps"}: "ConfigMapList"},
				cm,
			)
			kube := kubefake.NewSimpleClientset(
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-ns"}},
			)
			client := dclient.NewFakeClientWithDisco(dyn, kube, namespacedDisco{dclient.NewFakeDiscoveryClient(nil)})

			data := engineapi.NewMutatingAdmissionPolicyData(&policy, binding)
			er, err := Mutate(data, *resource, gvk, gvr, nil, client, nil, false, false)

			assert.NilError(t, err)
			if tc.expectRuleError {
				assert.Equal(t, er.IsSuccessful(), false)
				assert.Assert(t, len(er.PolicyResponse.Rules) == 1)
				r := er.PolicyResponse.Rules[0]
				assert.Equal(t, r.Status(), engineapi.RuleStatusError)
				assert.Equal(t, r.RuleType(), engineapi.Mutation)
				assert.Assert(t, r.MutatingAdmissionPolicyBinding() != nil)
				if tc.wantErr != "" {
					assert.Assert(t, strings.Contains(r.Message(), tc.wantErr))
				}
				assert.Equal(t, er.PatchedResource.GetLabels()["bug3"], "")
				return
			}
			assert.Equal(t, er.IsSuccessful(), true)
			if tc.wantLabel != "" {
				assert.Assert(t, len(er.PolicyResponse.Rules) == 1)
				assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusPass)
				assert.Equal(t, er.PatchedResource.GetLabels()["bug3"], tc.wantLabel)
			} else {
				assert.Equal(t, len(er.PolicyResponse.Rules), 0)
				assert.Equal(t, er.PatchedResource.GetLabels()["bug3"], "")
			}
		})
	}
}

func Test_Mutate_MultiBinding_MutationBleedPrevented(t *testing.T) {
	rawPolicy := []byte(`{
  "apiVersion": "admissionregistration.k8s.io/v1beta1",
  "kind": "MutatingAdmissionPolicy",
  "metadata": {"name": "label-namespaces-with-params"},
  "spec": {
    "paramKind": {"apiVersion": "v1", "kind": "ConfigMap"},
    "matchConstraints": {
      "resourceRules": [{
        "apiGroups": [""],
        "apiVersions": ["v1"],
        "operations": ["CREATE"],
        "resources": ["namespaces"]
      }]
    },
    "failurePolicy": "Fail",
    "reinvocationPolicy": "Never",
    "mutations": [{
      "patchType": "ApplyConfiguration",
      "applyConfiguration": {
        "expression": "Object{ metadata: Object.metadata{ labels: {\"bug3\": params.data.key}}}"
      }
    }]
  }
}`)
	rawResource := []byte(`{
  "apiVersion": "v1",
  "kind": "Namespace",
  "metadata": {"name": "test-ns"}
}`)

	var policy admissionregistrationv1beta1.MutatingAdmissionPolicy
	assert.NilError(t, json.Unmarshal(rawPolicy, &policy))

	binding1 := admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "binding-success"},
		Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
			PolicyName: policy.Name,
			ParamRef: &admissionregistrationv1beta1.ParamRef{
				Name:      "map-params",
				Namespace: "default",
			},
		},
	}
	deny := admissionregistrationv1beta1.DenyAction
	binding2 := admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "binding-missing-deny"},
		Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
			PolicyName: policy.Name,
			ParamRef: &admissionregistrationv1beta1.ParamRef{
				Name:                    "missing-params",
				Namespace:               "default",
				ParameterNotFoundAction: &deny,
			},
		},
	}

	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)
	gvk := resource.GroupVersionKind()
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}

	cmGVK := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	cm := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "map-params", "namespace": "default"},
		"data":       map[string]any{"key": "value"},
	}}
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(cmGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMapList"}, &unstructured.UnstructuredList{})
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{{Version: "v1", Resource: "configmaps"}: "ConfigMapList"},
		cm,
	)
	kube := kubefake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-ns"}},
	)
	client := dclient.NewFakeClientWithDisco(dyn, kube, namespacedDisco{dclient.NewFakeDiscoveryClient(nil)})

	data := engineapi.NewMutatingAdmissionPolicyData(&policy, binding1, binding2)
	er, err := Mutate(data, *resource, gvk, gvr, nil, client, nil, false, false)
	assert.NilError(t, err)
	assert.Equal(t, er.IsSuccessful(), false)
	assert.Assert(t, len(er.PolicyResponse.Rules) == 2)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusPass)
	assert.Equal(t, er.PolicyResponse.Rules[1].Status(), engineapi.RuleStatusError)
	// Under failurePolicy: Fail, partial mutation from binding1 must be rolled back to prevent security bleed
	assert.Equal(t, er.PatchedResource.GetLabels()["bug3"], "")
}

func Test_Mutate_ParamRefSelector_UnstructuredListAggregation(t *testing.T) {
	rawPolicy := []byte(`{
  "apiVersion": "admissionregistration.k8s.io/v1beta1",
  "kind": "MutatingAdmissionPolicy",
  "metadata": {"name": "label-with-param-list"},
  "spec": {
    "paramKind": {"apiVersion": "v1", "kind": "ConfigMap"},
    "matchConstraints": {
      "resourceRules": [{
        "apiGroups": [""],
        "apiVersions": ["v1"],
        "operations": ["CREATE"],
        "resources": ["namespaces"]
      }]
    },
    "failurePolicy": "Fail",
    "reinvocationPolicy": "Never",
    "mutations": [{
      "patchType": "ApplyConfiguration",
      "applyConfiguration": {
        "expression": "Object{ metadata: Object.metadata{ labels: {\"count\": string(params.items.size()), \"first\": string(params.items[0].data.item)}}}"
      }
    }]
  }
}`)
	rawBinding := []byte(`{
  "apiVersion": "admissionregistration.k8s.io/v1beta1",
  "kind": "MutatingAdmissionPolicyBinding",
  "metadata": {"name": "binding-selector"},
  "spec": {
    "policyName": "label-with-param-list",
    "paramRef": {
      "namespace": "default",
      "selector": {
        "matchLabels": {"env": "prod"}
      }
    }
  }
}`)
	rawResource := []byte(`{
  "apiVersion": "v1",
  "kind": "Namespace",
  "metadata": {"name": "test-ns"}
}`)

	var policy admissionregistrationv1beta1.MutatingAdmissionPolicy
	assert.NilError(t, json.Unmarshal(rawPolicy, &policy))
	var binding admissionregistrationv1beta1.MutatingAdmissionPolicyBinding
	assert.NilError(t, json.Unmarshal(rawBinding, &binding))

	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)
	gvk := resource.GroupVersionKind()
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}

	cmGVK := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	// Test fixtures supply 2 ConfigMaps matching selector to verify list aggregation (len > 1)
	cm1 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "cm1", "namespace": "default", "labels": map[string]any{"env": "prod"}},
		"data":       map[string]any{"item": "first-val"},
	}}
	cm2 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "cm2", "namespace": "default", "labels": map[string]any{"env": "prod"}},
		"data":       map[string]any{"item": "second-val"},
	}}

	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(cmGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMapList"}, &unstructured.UnstructuredList{})
	objects := []runtime.Object{cm1, cm2}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{{Version: "v1", Resource: "configmaps"}: "ConfigMapList"},
		objects...,
	)
	kube := kubefake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-ns"}},
	)
	client := dclient.NewFakeClientWithDisco(dyn, kube, namespacedDisco{dclient.NewFakeDiscoveryClient(nil)})

	data := engineapi.NewMutatingAdmissionPolicyData(&policy, binding)
	er, err := Mutate(data, *resource, gvk, gvr, nil, client, nil, false, false)
	assert.NilError(t, err)
	assert.Equal(t, er.IsSuccessful(), true)
	assert.Assert(t, len(er.PolicyResponse.Rules) == 1)
	r := er.PolicyResponse.Rules[0]
	assert.Equal(t, r.Status(), engineapi.RuleStatusPass)
	assert.Equal(t, r.RuleType(), engineapi.Mutation)
	assert.Assert(t, r.MutatingAdmissionPolicyBinding() != nil)
	assert.Equal(t, r.MutatingAdmissionPolicyBinding().GetName(), "binding-selector")
	assert.Equal(t, er.PatchedResource.GetLabels()["count"], "2")
	assert.Equal(t, er.PatchedResource.GetLabels()["first"], "first-val")
}
