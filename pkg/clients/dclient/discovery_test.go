package dclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	networkPolicyAPIResource       = metav1.APIResource{Name: "networkpolicies", SingularName: "", Namespaced: true, Kind: "NetworkPolicy"}
	networkPolicyStatusAPIResource = metav1.APIResource{Name: "networkpolicies/status", SingularName: "", Namespaced: true, Kind: "NetworkPolicy"}
	podAPIResource                 = metav1.APIResource{Name: "pods", SingularName: "", Namespaced: true, Kind: "Pod"}
	podEvictionAPIResource         = metav1.APIResource{Name: "pods/eviction", SingularName: "", Namespaced: true, Group: "policy", Version: "v1", Kind: "Eviction"}
	podLogAPIResource              = metav1.APIResource{Name: "pods/log", SingularName: "", Namespaced: true, Kind: "Pod"}
	cronJobAPIResource             = metav1.APIResource{Name: "cronjobs", SingularName: "", Namespaced: true, Kind: "CronJob"}
)

func Test_findSubresource(t *testing.T) {
	serverGroupsAndResources := []*metav1.APIResourceList{
		{
			GroupVersion: "networking.k8s.io/v1",
			APIResources: []metav1.APIResource{
				networkPolicyAPIResource,
				networkPolicyStatusAPIResource,
			},
		},

		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				podAPIResource,
				podEvictionAPIResource,
			},
		},
	}

	apiResource, gvr, err := findSubresource("", "pods", "eviction", "Pod/eviction", serverGroupsAndResources)
	assert.NoError(t, err)
	assert.Equal(t, gvr, schema.GroupVersionResource{Resource: "pods/eviction", Group: "policy", Version: "v1"})

	// Not comparing directly because actual apiResource also contains fields like 'ShortNames' which are not set in the expected apiResource
	assert.Equal(t, apiResource.Name, podEvictionAPIResource.Name)
	assert.Equal(t, apiResource.Kind, podEvictionAPIResource.Kind)
	assert.Equal(t, apiResource.Group, podEvictionAPIResource.Group)
	assert.Equal(t, apiResource.Version, podEvictionAPIResource.Version)

	apiResource, gvr, err = findSubresource("v1", "pods", "eviction", "Pod/eviction", serverGroupsAndResources)
	assert.NoError(t, err)
	assert.Equal(t, gvr, schema.GroupVersionResource{Resource: "pods/eviction", Group: "policy", Version: "v1"})

	// Not comparing directly because actual apiResource also contains fields like 'ShortNames' which are not set in the expected apiResource
	assert.Equal(t, apiResource.Name, podEvictionAPIResource.Name)
	assert.Equal(t, apiResource.Kind, podEvictionAPIResource.Kind)
	assert.Equal(t, apiResource.Group, podEvictionAPIResource.Group)
	assert.Equal(t, apiResource.Version, podEvictionAPIResource.Version)

	apiResource, gvr, err = findSubresource("networking.k8s.io/*", "networkpolicies", "status", "NetworkPolicy/status", serverGroupsAndResources)
	assert.NoError(t, err)
	assert.Equal(t, gvr, schema.GroupVersionResource{Resource: "networkpolicies/status", Group: "networking.k8s.io", Version: "v1"})

	// Not comparing directly because actual apiResource also contains fields like 'ShortNames' which are not set in the expected apiResource
	assert.Equal(t, apiResource.Name, networkPolicyStatusAPIResource.Name)
	assert.Equal(t, apiResource.Kind, networkPolicyStatusAPIResource.Kind)

	// Resources with empty GV use the GV of APIResourceList
	assert.Equal(t, apiResource.Group, "networking.k8s.io")
	assert.Equal(t, apiResource.Version, "v1")
}

func Test_findResource(t *testing.T) {
	serverGroupsAndResources := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				podEvictionAPIResource,
				podLogAPIResource,
				podAPIResource,
			},
		},
		{
			GroupVersion: "batch/v1beta1",
			APIResources: []metav1.APIResource{
				cronJobAPIResource,
			},
		},
		{
			GroupVersion: "batch/v1",
			APIResources: []metav1.APIResource{
				cronJobAPIResource,
			},
		},
	}

	serverPreferredResourcesList := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				podAPIResource,
			},
		},
		{
			GroupVersion: "batch/v1",
			APIResources: []metav1.APIResource{
				cronJobAPIResource,
			},
		},
	}

	apiResource, parentAPIResource, gvr, err := findResource("", "Pod", serverPreferredResourcesList, serverGroupsAndResources)
	assert.NoError(t, err)
	assert.Equal(t, gvr, schema.GroupVersionResource{Resource: "pods", Group: "", Version: "v1"})
	assert.Equal(t, parentAPIResource, (*metav1.APIResource)(nil))

	// Not comparing directly because actual apiResource also contains fields like 'ShortNames' which are not set in the expected apiResource
	assert.Equal(t, apiResource.Name, podAPIResource.Name)
	assert.Equal(t, apiResource.Kind, podAPIResource.Kind)

	// Resources with empty GV use the GV of APIResourceList
	assert.Equal(t, apiResource.Group, "")
	assert.Equal(t, apiResource.Version, "v1")

	apiResource, parentAPIResource, gvr, err = findResource("policy/v1", "Eviction", serverPreferredResourcesList, serverGroupsAndResources)
	assert.NoError(t, err)
	assert.Equal(t, gvr, schema.GroupVersionResource{Resource: "pods/eviction", Group: "policy", Version: "v1"})

	assert.Equal(t, parentAPIResource.Name, podAPIResource.Name)
	assert.Equal(t, parentAPIResource.Kind, podAPIResource.Kind)
	assert.Equal(t, parentAPIResource.Group, "")
	assert.Equal(t, parentAPIResource.Version, "v1")

	// Not comparing directly because actual apiResource also contains fields like 'ShortNames' which are not set in the expected apiResource
	assert.Equal(t, apiResource.Name, podEvictionAPIResource.Name)
	assert.Equal(t, apiResource.Kind, podEvictionAPIResource.Kind)
	assert.Equal(t, apiResource.Group, podEvictionAPIResource.Group)
	assert.Equal(t, apiResource.Version, podEvictionAPIResource.Version)

	apiResource, parentAPIResource, gvr, err = findResource("", "CronJob", serverPreferredResourcesList, serverGroupsAndResources)
	assert.NoError(t, err)
	assert.Equal(t, gvr, schema.GroupVersionResource{Resource: "cronjobs", Group: "batch", Version: "v1"})

	assert.Equal(t, parentAPIResource, (*metav1.APIResource)(nil))

	// Not comparing directly because actual apiResource also contains fields like 'ShortNames' which are not set in the expected apiResource
	assert.Equal(t, apiResource.Name, cronJobAPIResource.Name)
	assert.Equal(t, apiResource.Kind, cronJobAPIResource.Kind)
	assert.Equal(t, apiResource.Group, "batch")
	assert.Equal(t, apiResource.Version, "v1")

	apiResource, parentAPIResource, gvr, err = findResource("batch/v1beta1", "CronJob", serverPreferredResourcesList, serverGroupsAndResources)
	assert.NoError(t, err)
	assert.Equal(t, gvr, schema.GroupVersionResource{Resource: "cronjobs", Group: "batch", Version: "v1beta1"})

	assert.Equal(t, parentAPIResource, (*metav1.APIResource)(nil))

	// Not comparing directly because actual apiResource also contains fields like 'ShortNames' which are not set in the expected apiResource
	assert.Equal(t, apiResource.Name, cronJobAPIResource.Name)
	assert.Equal(t, apiResource.Kind, cronJobAPIResource.Kind)
	assert.Equal(t, apiResource.Group, "batch")
	assert.Equal(t, apiResource.Version, "v1beta1")
}

func Test_getServerResourceGroupVersion(t *testing.T) {
	apiResource := &metav1.APIResource{Name: "pods", SingularName: "", Namespaced: true, Kind: "Pod"}
	apiResourceListGV := "v1"
	assert.Equal(t, getServerResourceGroupVersion(apiResourceListGV, apiResource.Group, apiResource.Version), "v1")

	apiResource = &metav1.APIResource{Name: "horizontalpodautoscalers", SingularName: "", Namespaced: true, Kind: "HorizontalPodAutoscaler"}
	apiResourceListGV = "autoscaling/v2beta1"
	assert.Equal(t, getServerResourceGroupVersion(apiResourceListGV, apiResource.Group, apiResource.Version), "autoscaling/v2beta1")

	apiResource = &metav1.APIResource{Name: "deployments/scale", SingularName: "", Namespaced: true, Group: "autoscaling", Version: "v1", Kind: "Scale"}
	apiResourceListGV = "apps/v1"
	assert.Equal(t, getServerResourceGroupVersion(apiResourceListGV, apiResource.Group, apiResource.Version), "autoscaling/v1")
}

func Test_findResourceFromResourceName(t *testing.T) {
	serverGroupsAndResources := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				podAPIResource,
				podEvictionAPIResource,
			},
		},
	}

	apiResource, err := findResourceFromResourceName(schema.GroupVersionResource{Version: "v1", Resource: "pods"}, serverGroupsAndResources)
	assert.NoError(t, err)
	assert.Equal(t, apiResource.Name, podAPIResource.Name)
	assert.Equal(t, apiResource.Kind, podAPIResource.Kind)
	assert.Equal(t, apiResource.Group, "")
	assert.Equal(t, apiResource.Version, "v1")

	apiResource, err = findResourceFromResourceName(schema.GroupVersionResource{Group: "policy", Version: "v1", Resource: "pods/eviction"}, serverGroupsAndResources)
	assert.Error(t, err)

	apiResource, err = findResourceFromResourceName(schema.GroupVersionResource{Version: "v1", Resource: "pods/eviction"}, serverGroupsAndResources)
	assert.NoError(t, err)
	assert.Equal(t, apiResource.Name, podEvictionAPIResource.Name)
	assert.Equal(t, apiResource.Kind, podEvictionAPIResource.Kind)
	assert.Equal(t, apiResource.Group, podEvictionAPIResource.Group)
	assert.Equal(t, apiResource.Version, podEvictionAPIResource.Version)
}
