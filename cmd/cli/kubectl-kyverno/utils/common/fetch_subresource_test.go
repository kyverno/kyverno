package common

import (
	"fmt"
	"io"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cli/loader"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakediscovery "k8s.io/client-go/discovery/fake"
	clienttesting "k8s.io/client-go/testing"
)

// TestGetFromClusterPreservesSubresourceEndpoints exercises discovery and loading at sequential and concurrent settings.
func TestGetFromClusterPreservesSubresourceEndpoints(t *testing.T) {
	widget := metav1.APIResource{Name: "widgets", Kind: "Widget", Namespaced: true}
	status := metav1.APIResource{Name: "widgets/status", Kind: "Widget", Namespaced: true}
	resize := metav1.APIResource{Name: "widgets/resize", Kind: "Widget", Namespaced: true}
	widgetGVK := schema.GroupVersionKind{Group: "alpha.example.com", Version: "v1", Kind: "Widget"}
	scaleGVK := schema.GroupVersionKind{Group: "autoscaling", Version: "v1", Kind: "Scale"}
	tests := []struct {
		name      string
		endpoints []metav1.APIResource
		matches   []string
		want      map[string]schema.GroupVersionKind
	}{
		{
			name:      "parent with status and resize",
			endpoints: []metav1.APIResource{widget, status, resize},
			matches:   []string{"widgets", "widgets/status", "widgets/resize"},
			want: map[string]schema.GroupVersionKind{
				"Widget/": widgetGVK, "Widget/status": widgetGVK, "Widget/resize": widgetGVK,
			},
		},
		{
			name:      "status and resize without parent",
			endpoints: []metav1.APIResource{widget, status, resize},
			matches:   []string{"widgets/status", "widgets/resize"},
			want: map[string]schema.GroupVersionKind{
				"Widget/status": widgetGVK, "Widget/resize": widgetGVK,
			},
		},
		{
			name: "same scale kind from distinct parents",
			endpoints: []metav1.APIResource{
				widget,
				{Name: "widgets/scale", Kind: "Scale", Group: "autoscaling", Version: "v1", Namespaced: true},
				{Name: "gadgets", Kind: "Gadget", Namespaced: true},
				{Name: "gadgets/scale", Kind: "Scale", Group: "autoscaling", Version: "v1", Namespaced: true},
			},
			matches: []string{"widgets/scale", "gadgets/scale"},
			want: map[string]schema.GroupVersionKind{
				"Widget/scale": scaleGVK, "Gadget/scale": scaleGVK,
			},
		},
	}
	for _, tt := range tests {
		for _, concurrency := range []int{1, 2, 4} {
			t.Run(fmt.Sprintf("%s/concurrency=%d", tt.name, concurrency), func(t *testing.T) {
				delegate := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{
					Resources: []*metav1.APIResourceList{{
						GroupVersion: "alpha.example.com/v1", APIResources: tt.endpoints,
					}},
				}}
				constraints := makeMatchResources("alpha.example.com", "")
				constraints.ResourceRules[0].Resources = tt.matches
				fetcher := &ResourceFetcher{
					Client:    discoveryGroupCollisionClient{discovery: dclient.NewServerResourcesDiscovery(delegate)},
					Namespace: "default", Out: io.Discard,
					ResourceOptions: loader.ResourceOptions{Concurrency: concurrency, BatchSize: 100},
					Policies: []engineapi.GenericPolicy{engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
						Spec: policiesv1beta1.ValidatingPolicySpec{MatchConstraints: constraints},
					})},
				}
				resources, err := fetcher.getFromCluster()
				if err != nil {
					t.Fatal(err)
				}
				if len(resources) != len(tt.want) {
					t.Fatalf("got %d resources, want %d", len(resources), len(tt.want))
				}
				seen := make(map[string]int)
				for _, resource := range resources {
					parentKind := resource.GetAnnotations()["test.kyverno.io/parent-kind"]
					if parentKind == "" {
						parentKind = resource.GetKind()
					}
					endpoint := parentKind + "/" + resource.GetAnnotations()["test.kyverno.io/subresource"]
					wantGVK, ok := tt.want[endpoint]
					if !ok || resource.GroupVersionKind() != wantGVK || resource.GetName() != "shared" || resource.GetNamespace() != "default" {
						t.Errorf("unexpected input %s: %+v", endpoint, resource.Object)
					}
					seen[endpoint]++
				}
				for endpoint := range tt.want {
					if seen[endpoint] != 1 {
						t.Errorf("got %d inputs for %s, want 1", seen[endpoint], endpoint)
					}
				}
			})
		}
	}
}
