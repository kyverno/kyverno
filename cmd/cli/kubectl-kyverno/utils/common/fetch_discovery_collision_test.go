package common

import (
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakediscovery "k8s.io/client-go/discovery/fake"
	clienttesting "k8s.io/client-go/testing"
)

type discoveryGroupCollisionClient struct {
	groupCollisionClient
	discovery dclient.IDiscovery
}

// Discovery exposes the real resource discovery implementation over fake API responses.
func (c discoveryGroupCollisionClient) Discovery() dclient.IDiscovery {
	return c.discovery
}

// TestDiscoveredSubresourcesPreserveGroupVersion covers inherited and explicit identities from discovery through loading.
func TestDiscoveredSubresourcesPreserveGroupVersion(t *testing.T) {
	alphaV1 := schema.GroupVersion{Group: "alpha.example.com", Version: "v1"}
	betaV1 := schema.GroupVersion{Group: "beta.example.com", Version: "v1"}
	alphaV2 := schema.GroupVersion{Group: "alpha.example.com", Version: "v2"}
	coreV1 := schema.GroupVersion{Version: "v1"}
	childV1 := schema.GroupVersion{Group: "status.example.com", Version: "v1"}
	childV2 := schema.GroupVersion{Group: "status.example.com", Version: "v2"}
	tests := []struct {
		name    string
		parents []schema.GroupVersion
		child   schema.GroupVersion
		want    []schema.GroupVersion
	}{
		{
			name:    "inherits group and version across groups",
			parents: []schema.GroupVersion{alphaV1, betaV1},
			want:    []schema.GroupVersion{alphaV1, betaV1},
		},
		{
			name:    "inherits distinct versions within a group",
			parents: []schema.GroupVersion{alphaV1, alphaV2},
			want:    []schema.GroupVersion{alphaV1, alphaV2},
		},
		{
			name:    "inherits group and preserves explicit version",
			parents: []schema.GroupVersion{alphaV1},
			child:   schema.GroupVersion{Version: "v2"},
			want:    []schema.GroupVersion{alphaV2},
		},
		{
			name:    "inherits version and preserves explicit group",
			parents: []schema.GroupVersion{alphaV1},
			child:   schema.GroupVersion{Group: childV1.Group},
			want:    []schema.GroupVersion{childV1},
		},
		{
			name:    "preserves explicit group and version",
			parents: []schema.GroupVersion{coreV1},
			child:   childV2,
			want:    []schema.GroupVersion{childV2},
		},
		{
			name:    "inherits version in core API group",
			parents: []schema.GroupVersion{coreV1},
			want:    []schema.GroupVersion{coreV1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &resourceTypeInfo{
				gvkMap:         make(map[schema.GroupVersionKind]bool),
				subresourceMap: make(map[subresourceKey]v1alpha1.Subresource),
			}
			fetcher := &ResourceFetcher{Namespace: "default"}
			for _, parentGV := range tt.parents {
				// Query each parent explicitly, as separate policy kind selectors would.
				delegate := &fakediscovery.FakeDiscovery{
					Fake: &clienttesting.Fake{
						Resources: []*metav1.APIResourceList{{
							GroupVersion: parentGV.String(),
							APIResources: []metav1.APIResource{
								{Name: "widgets", Kind: "Widget", Namespaced: true},
								{
									Name: "widgets/status", Kind: "Widget", Namespaced: true,
									Group: tt.child.Group, Version: tt.child.Version,
								},
							},
						}},
					},
				}
				fetcher.Client = discoveryGroupCollisionClient{
					discovery: dclient.NewServerResourcesDiscovery(delegate),
				}
				fetcher.addToresourceTypeInfo(parentGV.String()+"/Widget/status", info)
			}

			if len(info.subresourceMap) != len(tt.want) {
				t.Fatalf("discovery retained %d subresources, want %d", len(info.subresourceMap), len(tt.want))
			}
			for i, wantGV := range tt.want {
				subresource, ok := info.subresourceMap[subresourceKey{GroupVersionKind: wantGV.WithKind("Widget"), parent: tt.parents[i].WithResource("widgets"), name: "status"}]
				if !ok {
					t.Fatalf("missing discovered subresource %s", wantGV)
				}
				if subresource.ParentResource.Group != tt.parents[i].Group || subresource.ParentResource.Version != tt.parents[i].Version {
					t.Errorf("wrong parent for subresource %s: %+v", wantGV, subresource.ParentResource)
				}
			}

			resources, err := fetcher.listResources(t.Context(), info)
			if err != nil {
				t.Fatal(err)
			}
			if len(resources) != len(tt.want) {
				t.Fatalf("loaded %d subresources, want %d", len(resources), len(tt.want))
			}
			seen := make(map[schema.GroupVersionKind]int)
			for _, resource := range resources {
				if resource.GetName() != "shared" || resource.GetNamespace() != "default" || resource.GetAnnotations()["test.kyverno.io/subresource"] != "status" {
					t.Errorf("unexpected subresource: %+v", resource.Object)
				}
				seen[resource.GroupVersionKind()]++
			}
			for _, wantGV := range tt.want {
				if seen[wantGV.WithKind("Widget")] != 1 {
					t.Errorf("expected one loaded subresource for %s, got %v", wantGV, seen)
				}
			}
		})
	}
}
