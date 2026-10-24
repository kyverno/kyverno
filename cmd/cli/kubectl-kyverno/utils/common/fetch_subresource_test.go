package common

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cli/loader"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

type subresourceLoadingClient struct {
	discoveryGroupCollisionClient
	dynamicClient dynamic.Interface
	failListKind  string
	failGetName   string
	checkContext  func(context.Context)
}

// GetDynamicInterface exposes the ordinary-resource client used by concurrent loading.
func (c subresourceLoadingClient) GetDynamicInterface() dynamic.Interface {
	return c.dynamicClient
}

// ListResource can fail a parent's list request while other endpoints remain available.
func (c subresourceLoadingClient) ListResource(ctx context.Context, apiVersion, kind, namespace string, selector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	if c.checkContext != nil {
		c.checkContext(ctx)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if kind == c.failListKind {
		return nil, errors.New("parent list failed")
	}
	return c.groupCollisionClient.ListResource(ctx, apiVersion, kind, namespace, selector)
}

// GetResource can fail one subresource while preserving other inputs.
func (c subresourceLoadingClient) GetResource(ctx context.Context, apiVersion, kind, namespace, name string, subresources ...string) (*unstructured.Unstructured, error) {
	if c.checkContext != nil {
		c.checkContext(ctx)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.Join(subresources, "/") == c.failGetName {
		return nil, errors.New("subresource get failed")
	}
	return c.groupCollisionClient.GetResource(ctx, apiVersion, kind, namespace, name, subresources...)
}

// TestGetFromClusterPreservesSubresourceEndpoints exercises discovery and loading at sequential and concurrent settings.
func TestGetFromClusterPreservesSubresourceEndpoints(t *testing.T) {
	widget := metav1.APIResource{Name: "widgets", Kind: "Widget", Namespaced: true}
	status := metav1.APIResource{Name: "widgets/status", Kind: "Widget", Namespaced: true}
	resize := metav1.APIResource{Name: "widgets/resize", Kind: "Widget", Namespaced: true}
	widgetGVK := schema.GroupVersionKind{Group: "alpha.example.com", Version: "v1", Kind: "Widget"}
	scaleGVK := schema.GroupVersionKind{Group: "autoscaling", Version: "v1", Kind: "Scale"}
	tests := []struct {
		name         string
		failListKind string
		failGetName  string
		endpoints    []metav1.APIResource
		matches      []string
		want         map[string]schema.GroupVersionKind
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
			name: "multiple parents alongside a subresource",
			endpoints: []metav1.APIResource{widget, status,
				{Name: "gadgets", Kind: "Gadget", Namespaced: true}},
			matches: []string{"widgets", "gadgets", "widgets/status"},
			want: map[string]schema.GroupVersionKind{
				"Widget/": widgetGVK, "Widget/status": widgetGVK,
				"Gadget/": {Group: "alpha.example.com", Version: "v1", Kind: "Gadget"},
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
			name:         "failed parent list preserves other subresources",
			failListKind: "Gadget",
			endpoints: []metav1.APIResource{widget, status,
				{Name: "gadgets", Kind: "Gadget", Namespaced: true},
				{Name: "gadgets/status", Kind: "Gadget", Namespaced: true}},
			matches: []string{"widgets/status", "gadgets/status"},
			want:    map[string]schema.GroupVersionKind{"Widget/status": widgetGVK},
		},
		{
			name:        "failed subresource get preserves parent and other subresources",
			failGetName: "status",
			endpoints:   []metav1.APIResource{widget, status, resize},
			matches:     []string{"widgets", "widgets/status", "widgets/resize"},
			want:        map[string]schema.GroupVersionKind{"Widget/": widgetGVK, "Widget/resize": widgetGVK},
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
				listKinds := make(map[schema.GroupVersionResource]string)
				var objects []runtime.Object
				for _, endpoint := range tt.endpoints {
					if strings.Contains(endpoint.Name, "/") {
						continue
					}
					gvr := schema.GroupVersionResource{Group: "alpha.example.com", Version: "v1", Resource: endpoint.Name}
					listKinds[gvr] = endpoint.Kind + "List"
					parents, err := (groupCollisionClient{}).ListResource(context.Background(), "alpha.example.com/v1", endpoint.Kind, "default", nil)
					if err != nil {
						t.Fatal(err)
					}
					objects = append(objects, &parents.Items[0])
				}
				dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objects...)
				fetcher := &ResourceFetcher{
					Client: subresourceLoadingClient{
						discoveryGroupCollisionClient: discoveryGroupCollisionClient{discovery: dclient.NewServerResourcesDiscovery(delegate)},
						dynamicClient:                 dynamicClient,
						failListKind:                  tt.failListKind,
						failGetName:                   tt.failGetName,
						checkContext: func(ctx context.Context) {
							deadline, ok := ctx.Deadline()
							if !ok || time.Until(deadline) > 5*time.Minute {
								t.Error("sequential request has no five-minute deadline")
							}
						},
					},
					Namespace: "default", Out: io.Discard,
					ResourceOptions: loader.ResourceOptions{Concurrency: concurrency, BatchSize: 100, Namespace: "default"},
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
				// Ordinary resources must still use the concurrent loader when subresources match.
				wantConcurrentLists := 0
				if concurrency > 1 {
					for endpoint := range tt.want {
						if strings.HasSuffix(endpoint, "/") {
							wantConcurrentLists++
						}
					}
				}
				if got := len(dynamicClient.Actions()); got != wantConcurrentLists {
					t.Errorf("got %d concurrent resource lists, want %d", got, wantConcurrentLists)
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

// TestListResourcesCancellation verifies list and get requests receive a cancellable context and cancellation is returned.
func TestListResourcesCancellation(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "alpha.example.com", Version: "v1", Kind: "Widget"}
	for _, operation := range []string{"list", "get"} {
		t.Run(operation, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			calls := 0
			fetcher := &ResourceFetcher{Namespace: "default", Client: subresourceLoadingClient{
				checkContext: func(ctx context.Context) {
					calls++
					if operation == "list" || calls == 2 {
						cancel()
						if ctx.Err() == nil {
							t.Error("request did not receive the loading context")
						}
					}
				},
			}}
			info := &resourceTypeInfo{subresourceMap: map[subresourceKey]v1alpha1.Subresource{
				{GroupVersionKind: gvk, name: "status"}: {
					ParentResource: metav1.APIResource{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind, Name: "widgets"},
					Subresource:    metav1.APIResource{Name: "widgets/status", Kind: gvk.Kind},
				},
			}}
			_, err := fetcher.listResources(ctx, info)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("got error %v, want cancellation", err)
			}
			wantCalls := 1
			if operation == "get" {
				wantCalls = 2
			}
			if calls != wantCalls {
				t.Errorf("got %d API calls, want %d", calls, wantCalls)
			}
		})
	}
}
