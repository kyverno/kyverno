package policy

import (
	"encoding/json"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	ur, err := controller.newCELMutateURForTrigger("resize-policy", trigger)

	assert.NoError(t, err)
	assert.Equal(t, kyvernov2.CELMutate, ur.Spec.Type)
	assert.Equal(t, "resize-policy", ur.Spec.Policy)
	request := ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest
	assert.NotNil(t, request)
	assert.Equal(t, "pods", request.Resource.Resource)
	assert.Equal(t, "default", request.Namespace)
	assert.Equal(t, "trigger-pod", request.Name)
	var object map[string]interface{}
	assert.NoError(t, json.Unmarshal(request.Object.Raw, &object))
	assert.Equal(t, "trigger-pod", object["metadata"].(map[string]interface{})["name"])
}
