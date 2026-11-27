package policy

import (
	"encoding/json"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestNewCELMutateUR(t *testing.T) {
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "my-mpol"},
	}

	ur := newCELMutateUR(mpol)

	assert.NotNil(t, ur)
	assert.Equal(t, kyvernov2.CELMutate, ur.Spec.Type)
	assert.Equal(t, "my-mpol", ur.Spec.Policy)
	assert.Equal(t, kyvernov2.SchemeGroupVersion.String(), ur.TypeMeta.APIVersion)
	assert.Equal(t, "UpdateRequest", ur.TypeMeta.Kind)
	assert.Equal(t, "ur-", ur.GenerateName)
}

func TestNewCELMutateURFromNamespacedPolicy(t *testing.T) {
	nmpol := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "my-nmpol", Namespace: "my-ns"},
	}

	ur := newCELMutateURFromNamespacedPolicy(nmpol)

	assert.NotNil(t, ur)
	assert.Equal(t, kyvernov2.CELMutate, ur.Spec.Type)
	assert.Equal(t, "my-ns/my-nmpol", ur.Spec.Policy)
	assert.Equal(t, kyvernov2.SchemeGroupVersion.String(), ur.TypeMeta.APIVersion)
	assert.Equal(t, "UpdateRequest", ur.TypeMeta.Kind)
	assert.Equal(t, "ur-", ur.GenerateName)
}

func TestNewCELMutateURForTrigger(t *testing.T) {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, meta.RESTScopeNamespace)
	controller := &policyController{restMapper: mapper}
	trigger := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "trigger-pod",
			"namespace": "default",
		},
	}}

	ur, err := controller.newCELMutateURForTrigger("resize-policy", trigger, "status")

	assert.NoError(t, err)
	assert.Equal(t, kyvernov2.CELMutate, ur.Spec.Type)
	assert.Equal(t, "resize-policy", ur.Spec.Policy)
	request := ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest
	assert.NotNil(t, request)
	assert.Equal(t, "pods", request.Resource.Resource)
	assert.Equal(t, "status", request.SubResource)
	assert.Equal(t, "default", request.Namespace)
	assert.Equal(t, "trigger-pod", request.Name)
	var object map[string]interface{}
	assert.NoError(t, json.Unmarshal(request.Object.Raw, &object))
	assert.Equal(t, "trigger-pod", object["metadata"].(map[string]interface{})["name"])
}

func TestCreateURForNamespacedMutatingPolicyScopesTriggers(t *testing.T) {
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{gvr.GroupVersion()})
	mapper.Add(gvr.GroupVersion().WithKind("Pod"), meta.RESTScopeNamespace)
	local := podNamed("pod-a")
	local.SetNamespace("team-a")
	other := podNamed("pod-b")
	other.SetNamespace("team-b")
	client, err := dclient.NewFakeClient(runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "PodList"}, local, other)
	require.NoError(t, err)
	client.SetDiscovery(dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{gvr}))
	generator := &captureURGenerator{}
	controller := &policyController{client: client, restMapper: mapper, urGenerator: generator}
	policy := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mutate-pods", Namespace: "team-a"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Rule: admissionregistrationv1.Rule{
							APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"},
						},
					},
				}},
			},
			TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{Expression: "object"},
		},
	}

	require.NoError(t, controller.createURForNamespacedMutatingPolicy(policy))

	var listNamespaces []string
	for _, action := range client.GetDynamicInterface().(*dynamicfake.FakeDynamicClient).Actions() {
		if action.Matches("list", "pods") {
			listNamespaces = append(listNamespaces, action.GetNamespace())
		}
	}
	assert.Equal(t, []string{"team-a"}, listNamespaces)
	require.Len(t, generator.captured, 1)
	ur := generator.captured[0]
	assert.Equal(t, kyvernov2.CELMutate, ur.Spec.Type)
	assert.Equal(t, "team-a/mutate-pods", ur.Spec.Policy)
	request := ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest
	require.NotNil(t, request)
	assert.Equal(t, "team-a", request.Namespace)
	assert.Equal(t, "pod-a", request.Name)
}
