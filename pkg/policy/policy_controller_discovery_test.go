package policy

import (
	"errors"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	discoveryfake "k8s.io/client-go/discovery/fake"
)

type partialDiscoveryClient struct {
	discoveryfake.FakeDiscovery
	groups    []*metav1.APIGroup
	resources []*metav1.APIResourceList
	err       error
}

func (d *partialDiscoveryClient) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return d.groups, d.resources, d.err
}

func TestNewRESTMapperLogsPartialDiscoveryError(t *testing.T) {
	coreVersion := metav1.GroupVersionForDiscovery{GroupVersion: "v1", Version: "v1"}
	appsVersion := metav1.GroupVersionForDiscovery{GroupVersion: "apps/v1", Version: "v1"}
	failingVersion := metav1.GroupVersionForDiscovery{GroupVersion: "repro.example.com/v1beta1", Version: "v1beta1"}
	groups := []*metav1.APIGroup{
		{Name: "", Versions: []metav1.GroupVersionForDiscovery{coreVersion}, PreferredVersion: coreVersion},
		{Name: "apps", Versions: []metav1.GroupVersionForDiscovery{appsVersion}, PreferredVersion: appsVersion},
		{Name: "repro.example.com", Versions: []metav1.GroupVersionForDiscovery{failingVersion}, PreferredVersion: failingVersion},
	}
	resources := []*metav1.APIResourceList{
		{GroupVersion: "v1", APIResources: []metav1.APIResource{{Name: "pods", Kind: "Pod", Namespaced: true}}},
		{GroupVersion: "apps/v1", APIResources: []metav1.APIResource{{Name: "deployments", Kind: "Deployment", Namespaced: true}}},
	}
	discoveryErr := &discovery.ErrGroupDiscoveryFailed{Groups: map[schema.GroupVersion]error{
		{Group: "repro.example.com", Version: "v1beta1"}: errors.New("simulated APIService unavailable"),
	}}
	client := &partialDiscoveryClient{groups: groups, resources: resources, err: discoveryErr}
	gotGroups, gotResources, gotErr := client.ServerGroupsAndResources()
	var groupDiscoveryErr *discovery.ErrGroupDiscoveryFailed
	require.ErrorAs(t, gotErr, &groupDiscoveryErr)
	require.Len(t, gotGroups, 3)
	require.Len(t, gotResources, 2)

	var logs []string
	logger := funcr.New(func(_, args string) { logs = append(logs, args) }, funcr.Options{})
	mapper := newRESTMapper(client, logger)
	require.Len(t, logs, 1)
	assert.Contains(t, logs[0], "REST mapper was built from partial discovery results")
	assert.Contains(t, logs[0], "repro.example.com/v1beta1")

	deploymentGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	deploymentGVK, err := mapper.KindFor(deploymentGVR)
	require.NoError(t, err)
	assert.Equal(t, "Deployment", deploymentGVK.Kind)
	_, err = mapper.RESTMapping(schema.GroupKind{Group: "apps", Kind: "Deployment"}, "v1")
	require.NoError(t, err)

	missingGVR := schema.GroupVersionResource{Group: "repro.example.com", Version: "v1beta1", Resource: "widgets"}
	_, kindErr := mapper.KindFor(missingGVR)
	require.Error(t, kindErr)
	assert.Contains(t, kindErr.Error(), "repro.example.com/v1beta1")
	assert.NotContains(t, kindErr.Error(), "discovery")
	_, mappingErr := mapper.RESTMapping(schema.GroupKind{Group: "repro.example.com", Kind: "Widget"}, "v1beta1")
	require.Error(t, mappingErr)
	assert.Contains(t, mappingErr.Error(), "repro.example.com/v1beta1")
	assert.NotContains(t, mappingErr.Error(), "discovery")
}
