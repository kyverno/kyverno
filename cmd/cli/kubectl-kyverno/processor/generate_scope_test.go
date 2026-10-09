package processor

import (
	"io"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestMockControllerCloneListResourceScope(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name             string
		kind             string
		fixture          bool
		fixtureNamespace string
		wantError        string
	}{
		{name: "empty namespaced list", kind: "Secret"},
		{name: "namespaced fixture omits namespace", kind: "Secret", fixture: true},
		{name: "cluster scoped kind", kind: "Namespace", wantError: "must be namespaced"},
		{name: "cluster scoped fixture has namespace", kind: "Namespace", fixture: true, fixtureNamespace: "target", wantError: "must be namespaced"},
		{name: "unknown kind", kind: "Unknown", wantError: "target scope"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			trigger := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "ConfigMap",
				"metadata": map[string]interface{}{"name": "trigger", "namespace": "target"},
			}}
			objects := []runtime.Object{trigger}
			if test.fixture {
				fixture := &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "v1", "kind": test.kind,
					"metadata": map[string]interface{}{"name": "fixture"},
				}}
				fixture.SetNamespace(test.fixtureNamespace)
				objects = append(objects, fixture)
			}
			policy := &kyvernov1.Policy{
				TypeMeta:   metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "Policy"},
				ObjectMeta: metav1.ObjectMeta{Name: "clone-list", Namespace: "target"},
				Spec: kyvernov1.Spec{Rules: []kyvernov1.Rule{{
					Name: "clone-list",
					MatchResources: kyvernov1.MatchResources{Any: kyvernov1.ResourceFilters{{
						ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"ConfigMap"}},
					}}},
					Generation: &kyvernov1.Generation{GeneratePattern: kyvernov1.GeneratePattern{
						ResourceSpec: kyvernov1.ResourceSpec{Namespace: "target"},
						CloneList:    kyvernov1.CloneList{Namespace: "target", Kinds: []string{"v1/" + test.kind}},
					}},
				}}},
			}
			listKinds := map[schema.GroupVersionResource]string{
				{Version: "v1", Resource: strings.ToLower(test.kind) + "s"}: test.kind + "List",
			}
			controller, err := initializeMockController(io.Discard, &store.Store{}, listKinds, objects)
			require.NoError(t, err)
			cfg := config.NewDefaultConfiguration(false)
			policyContext, err := engine.NewPolicyContext(jmespath.New(cfg), *trigger, kyvernov1.Create, nil, cfg)
			require.NoError(t, err)
			policyContext = policyContext.WithPolicy(policy)
			generated, err := controller.ApplyGeneratePolicy(logr.Discard(), policyContext, []string{"clone-list"})
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
				return
			}
			require.NoError(t, err)
			require.Contains(t, generated, "clone-list", "the empty list must still execute the rule")
			require.Empty(t, generated["clone-list"])
		})
	}
}

func TestMockControllerDataTargetScope(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, apiVersion, kind string
		data                   map[string]any
		fixtureNamespaces      map[string]string
		storageAPIVersion      string
		wantError              string
	}{
		{
			name: "builtin target without fixture", apiVersion: "policy/v1", kind: "PodDisruptionBudget",
			data: map[string]any{"spec": map[string]any{"maxUnavailable": 1, "selector": map[string]any{"matchLabels": map[string]any{"app": "test"}}}},
		},
		{
			name: "irregular plural without fixture", apiVersion: "networking.k8s.io/v1", kind: "NetworkPolicy",
			data: map[string]any{"spec": map[string]any{"podSelector": map[string]any{}, "policyTypes": []any{"Ingress"}}},
		},
		{name: "cluster scoped target", apiVersion: "v1", kind: "Namespace", wantError: "must be namespaced"},
		{name: "unknown target", apiVersion: "v1", kind: "Unknown", wantError: "target scope"},
		{name: "unknown builtin version", apiVersion: "policy/v99", kind: "PodDisruptionBudget", wantError: "target scope"},
		{name: "builtin in unknown group", apiVersion: "unknown.example/v1", kind: "PodDisruptionBudget", wantError: "target scope"},
		{
			name: "omitted builtin Event version", kind: "Event", storageAPIVersion: "v1",
			data: map[string]any{"reason": "Generated", "message": "generated event"},
		},
		{
			name: "explicit Event group", apiVersion: "events.k8s.io/v1", kind: "Event", storageAPIVersion: "events.k8s.io/v1",
			data: map[string]any{"reason": "Generated", "note": "generated event"},
		},
		{
			name: "exact version with mixed scopes", apiVersion: "alpha.example/v1", kind: "Widget",
			data:              map[string]any{"spec": map[string]any{"value": "generated"}},
			fixtureNamespaces: map[string]string{"alpha.example/v1": "target", "alpha.example/v2": ""},
		},
		{
			name: "exact group with mixed scopes", apiVersion: "alpha.example/v1", kind: "Widget",
			data:              map[string]any{"spec": map[string]any{"value": "generated"}},
			fixtureNamespaces: map[string]string{"alpha.example/v1": "target", "beta.example/v1": ""},
		},
		{
			name: "omitted version with mixed scopes", kind: "Widget", wantError: "must be namespaced",
			fixtureNamespaces: map[string]string{"alpha.example/v1": "target", "alpha.example/v2": ""},
		},
		{
			name: "omitted group with mixed scopes", kind: "Widget", wantError: "must be namespaced",
			fixtureNamespaces: map[string]string{"alpha.example/v1": "target", "beta.example/v1": ""},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			trigger := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "v1", "kind": "ConfigMap",
				"metadata": map[string]any{"name": "trigger", "namespace": "target"},
			}}
			pattern := kyvernov1.GeneratePattern{ResourceSpec: kyvernov1.ResourceSpec{
				APIVersion: test.apiVersion, Kind: test.kind, Namespace: "target", Name: "generated",
			}}
			pattern.SetData(test.data)
			policy := &kyvernov1.Policy{
				TypeMeta:   metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "Policy"},
				ObjectMeta: metav1.ObjectMeta{Name: "data-target", Namespace: "target"},
				Spec: kyvernov1.Spec{Rules: []kyvernov1.Rule{{
					Name: "data-target",
					MatchResources: kyvernov1.MatchResources{Any: kyvernov1.ResourceFilters{{
						ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"ConfigMap"}},
					}}},
					Generation: &kyvernov1.Generation{GeneratePattern: pattern},
				}}},
			}
			objects := []runtime.Object{trigger}
			for apiVersion, namespace := range test.fixtureNamespaces {
				objects = append(objects, &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": apiVersion, "kind": test.kind,
					"metadata": map[string]any{"name": "fixture", "namespace": namespace},
				}})
			}
			controller, err := initializeMockController(io.Discard, &store.Store{}, nil, objects)
			require.NoError(t, err)
			cfg := config.NewDefaultConfiguration(false)
			policyContext, err := engine.NewPolicyContext(jmespath.New(cfg), *trigger, kyvernov1.Create, nil, cfg)
			require.NoError(t, err)
			generated, err := controller.ApplyGeneratePolicy(logr.Discard(), policyContext.WithPolicy(policy), []string{"data-target"})
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
				return
			}
			require.NoError(t, err)
			require.Len(t, generated["data-target"], 1)
			resources, err := controller.GetUnstrResources(generated["data-target"])
			require.NoError(t, err)
			require.Len(t, resources, 1)
			require.Equal(t, test.apiVersion, resources[0].GetAPIVersion())
			require.Equal(t, test.kind, resources[0].GetKind())
			require.Equal(t, "target", resources[0].GetNamespace())
			require.Equal(t, "generated", resources[0].GetName())
			if test.storageAPIVersion != "" {
				spec := generated["data-target"][0]
				spec.APIVersion = test.storageAPIVersion
				stored, err := controller.GetUnstrResources([]kyvernov1.ResourceSpec{spec})
				require.NoError(t, err)
				require.Len(t, stored, 1, "generation must use the preferred API resource")
				require.Equal(t, "generated", stored[0].GetName())
			}
		})
	}
}
