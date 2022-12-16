package engine

import (
	"testing"

	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetSubresourceGVKToAPIResourceMap(t *testing.T) {

	podAPIResource := metav1.APIResource{
		Name:         "pods",
		SingularName: "",
		Namespaced:   true,
		Kind:         "Pod",
		Group:        "",
		Version:      "v1",
	}

	podStatusAPIResource := metav1.APIResource{
		Name:         "pods/status",
		SingularName: "",
		Namespaced:   true,
		Kind:         "Pod",
		Group:        "",
		Version:      "v1",
	}

	podEvictAPIResource := metav1.APIResource{
		Name:         "pods/eviction",
		SingularName: "",
		Namespaced:   true,
		Kind:         "Eviction",
		Group:        "policy",
		Version:      "v1",
	}

	policyContext := NewPolicyContext().
		WithSubresourcesInPolicy([]struct {
			APIResource    metav1.APIResource
			ParentResource metav1.APIResource
		}{
			{
				APIResource:    podStatusAPIResource,
				ParentResource: podAPIResource,
			},
			{
				APIResource:    podEvictAPIResource,
				ParentResource: podAPIResource,
			},
		})

	kindsInPolicy := []string{"Pod", "Eviction", "Pod/status", "Pod/eviction"}

	subresourceGVKToAPIResourceMap := GetSubresourceGVKToAPIResourceMap(kindsInPolicy, policyContext)

	podStatusResourceFromMap := subresourceGVKToAPIResourceMap["Pod/status"]
	assert.Equal(t, podStatusResourceFromMap.Name, podStatusAPIResource.Name)
	assert.Equal(t, podStatusResourceFromMap.Kind, podStatusAPIResource.Kind)
	assert.Equal(t, podStatusResourceFromMap.Group, podStatusAPIResource.Group)
	assert.Equal(t, podStatusResourceFromMap.Version, podStatusAPIResource.Version)

	podEvictResourceFromMap := subresourceGVKToAPIResourceMap["Pod/eviction"]
	assert.Equal(t, podEvictResourceFromMap.Name, podEvictAPIResource.Name)
	assert.Equal(t, podEvictResourceFromMap.Kind, podEvictAPIResource.Kind)
	assert.Equal(t, podEvictResourceFromMap.Group, podEvictAPIResource.Group)
	assert.Equal(t, podEvictResourceFromMap.Version, podEvictAPIResource.Version)

	podEvictResourceFromMap = subresourceGVKToAPIResourceMap["Eviction"]
	assert.Equal(t, podEvictResourceFromMap.Name, podEvictAPIResource.Name)
	assert.Equal(t, podEvictResourceFromMap.Kind, podEvictAPIResource.Kind)
	assert.Equal(t, podEvictResourceFromMap.Group, podEvictAPIResource.Group)
	assert.Equal(t, podEvictResourceFromMap.Version, podEvictAPIResource.Version)

	_, ok := subresourceGVKToAPIResourceMap["Pod"]
	assert.Equal(t, ok, false)
}
