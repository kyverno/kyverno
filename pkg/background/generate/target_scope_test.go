package generate

import (
	"fmt"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type targetScopeDiscovery struct {
	dclient.IDiscovery
	empty bool
	mixed bool
}

func (d targetScopeDiscovery) FindResources(group, version, kind, subresource string) (map[dclient.TopLevelApiDescription]metav1.APIResource, error) {
	if d.empty {
		return nil, nil
	}
	if subresource != "" || (group != "" && group != "*") || (version != "v1" && version != "*") ||
		(kind != "ConfigMap" && kind != "Secret" && kind != "Namespace") {
		return nil, fmt.Errorf("unknown resource %s/%s/%s/%s", group, version, kind, subresource)
	}
	resources := map[dclient.TopLevelApiDescription]metav1.APIResource{
		{GroupVersion: schema.GroupVersion{Version: "v1"}, Kind: kind}: {Namespaced: kind != "Namespace"},
	}
	if d.mixed {
		resources[dclient.TopLevelApiDescription{GroupVersion: schema.GroupVersion{Group: "example.com", Version: "v1"}, Kind: kind}] = metav1.APIResource{Namespaced: false}
	}
	return resources, nil
}

func targetScopePattern(mode string) kyvernov1.GeneratePattern {
	pattern := kyvernov1.GeneratePattern{
		ResourceSpec: kyvernov1.ResourceSpec{APIVersion: "v1", Kind: "ConfigMap", Namespace: "tenant", Name: "copy"},
	}
	switch mode {
	case "data":
		pattern.SetData(map[string]any{"data": map[string]any{"value": "generated"}})
	case "clone":
		pattern.Clone = kyvernov1.CloneFrom{Namespace: "tenant", Name: "source"}
	case "cloneList":
		pattern.ResourceSpec = kyvernov1.ResourceSpec{Namespace: "tenant"}
		pattern.CloneList = kyvernov1.CloneList{Namespace: "tenant", Kinds: []string{"v1/ConfigMap", "v1/Secret"}}
	}
	return pattern
}

// Exercise stored Policy objects directly, without the newer admission checks.
// Rejections must precede target writes and even clone source reads/updates.
func TestGenerateResolvedTargetScope(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"data", "clone", "cloneList"} {
		for _, tc := range []struct {
			name             string
			change           func(*kyvernov1.GeneratePattern)
			empty, mixed     bool
			clusterPolicy    bool
			missingNamespace bool
			wantError        string
		}{
			{name: "same namespace"},
			{name: "foreign namespace", change: func(p *kyvernov1.GeneratePattern) { p.Namespace = "other" }, wantError: "must equal policy namespace"},
			{name: "empty namespace", change: func(p *kyvernov1.GeneratePattern) { p.Namespace = "" }, wantError: "must equal policy namespace"},
			{name: "missing policy namespace", missingNamespace: true, wantError: "requires a policy namespace"},
			{name: "resolved same namespace", change: func(p *kyvernov1.GeneratePattern) { p.Namespace = "{{ request.object.metadata.namespace }}" }},
			{name: "resolved foreign namespace", change: func(p *kyvernov1.GeneratePattern) { p.Namespace = "{{ request.object.metadata.labels.source }}" }, wantError: "must equal policy namespace"},
			{name: "cluster scoped kind", change: func(p *kyvernov1.GeneratePattern) {
				p.Kind = "{{ 'Namespace' }}"
				if len(p.CloneList.Kinds) > 0 {
					p.CloneList.Kinds[1] = "{{ 'v1/Namespace' }}"
				}
			}, wantError: "must be namespaced"},
			{name: "unknown apiVersion", change: func(p *kyvernov1.GeneratePattern) {
				p.APIVersion = "{{ 'v9' }}"
				if len(p.CloneList.Kinds) > 0 {
					p.CloneList.Kinds[1] = "v9/ConfigMap"
				}
			}, wantError: "target scope"},
			{name: "unknown kind", change: func(p *kyvernov1.GeneratePattern) {
				p.Kind = "Missing"
				if len(p.CloneList.Kinds) > 0 {
					p.CloneList.Kinds[1] = "v1/Missing"
				}
			}, wantError: "target scope"},
			{name: "subresource", change: func(p *kyvernov1.GeneratePattern) {
				p.Kind = "ConfigMap/status"
				if len(p.CloneList.Kinds) > 0 {
					p.CloneList.Kinds[1] = "v1/ConfigMap/status"
				}
			}, wantError: "top-level target kind"},
			{name: "wildcard", change: func(p *kyvernov1.GeneratePattern) {
				p.Kind = "*"
				if len(p.CloneList.Kinds) > 0 {
					p.CloneList.Kinds[1] = "v1/*"
				}
			}, wantError: "top-level target kind"},
			{name: "empty discovery", empty: true, wantError: "cannot determine target scope"},
			{name: "omitted apiVersion", change: func(p *kyvernov1.GeneratePattern) {
				p.APIVersion = ""
				if len(p.CloneList.Kinds) > 0 {
					p.CloneList.Kinds = []string{"ConfigMap", "Secret"}
				}
			}},
			{name: "mixed discovery scopes", mixed: true, change: func(p *kyvernov1.GeneratePattern) {
				p.APIVersion = ""
				if len(p.CloneList.Kinds) > 0 {
					p.CloneList.Kinds = []string{"ConfigMap", "Secret"}
				}
			}, wantError: "must be namespaced"},
			{name: "ClusterPolicy cross namespace", clusterPolicy: true, change: func(p *kyvernov1.GeneratePattern) { p.Namespace = "other" }},
			{name: "ClusterPolicy cluster target", clusterPolicy: true, change: func(p *kyvernov1.GeneratePattern) {
				p.Namespace = ""
				p.Kind = "Namespace"
				if len(p.CloneList.Kinds) > 0 {
					p.CloneList.Kinds = []string{"v1/Namespace"}
				}
			}},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				pattern := targetScopePattern(mode)
				if tc.change != nil {
					tc.change(&pattern)
				}
				gen, client := newCloneSourceGenerator(t, pattern, !tc.clusterPolicy, "other")
				client.SetDiscovery(targetScopeDiscovery{empty: tc.empty, mixed: tc.mixed})
				if tc.missingNamespace {
					gen.policy.SetNamespace("")
				}
				_, err := gen.generate()
				if tc.wantError != "" {
					require.ErrorContains(t, err, tc.wantError)
					require.Empty(t, client.calls, "reject the entire substituted target before any source or target operation")
				} else {
					require.NoError(t, err)
					require.NotEmpty(t, client.calls, "valid stored policies must still execute")
				}
				require.Equal(t, pattern, gen.pattern, "validation must not rewrite the cached template")
			})
		}
	}
}

func TestGenerateTargetScopeReferencesAndClonePrecedence(t *testing.T) {
	t.Parallel()
	for _, namespace := range []string{"tenant", "other"} {
		t.Run("reference/"+namespace, func(t *testing.T) {
			t.Parallel()
			pattern := targetScopePattern("data")
			pattern.Namespace = "$(./../data/targetNamespace)"
			pattern.Name = "$(./../data/targetName)"
			pattern.SetData(map[string]any{"targetNamespace": namespace, "targetName": "dynamic-name"})
			gen, client := newCloneSourceGenerator(t, pattern, true, "")
			client.SetDiscovery(targetScopeDiscovery{})
			_, err := gen.generate()
			if namespace != "tenant" {
				require.ErrorContains(t, err, "must equal policy namespace")
				require.Empty(t, client.calls)
			} else {
				require.NoError(t, err)
				require.Equal(t, []string{"get:tenant/dynamic-name", "create:tenant"}, client.calls)
			}
		})
	}
	t.Run("single clone precedes cloneList", func(t *testing.T) {
		t.Parallel()
		pattern := targetScopePattern("clone")
		pattern.Kind = "Namespace"
		pattern.CloneList = kyvernov1.CloneList{Namespace: "tenant", Kinds: []string{"v1/ConfigMap"}}
		gen, client := newCloneSourceGenerator(t, pattern, true, "")
		client.SetDiscovery(targetScopeDiscovery{})
		_, err := gen.generate()
		require.ErrorContains(t, err, "target v1/Namespace must be namespaced")
		require.Empty(t, client.calls)
	})
}

func TestGenerateForeachResolvedTargetScope(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"data", "clone", "cloneList"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			pattern := targetScopePattern(mode)
			pattern.Namespace = "{{ element.namespace }}"
			gen, client := newCloneSourceGenerator(t, pattern, true, "")
			client.SetDiscovery(targetScopeDiscovery{})
			require.NoError(t, gen.policyContext.JSONContext().AddVariable("targets", []any{
				map[string]any{"namespace": "tenant"}, map[string]any{"namespace": "other"},
			}))
			gen.forEach = []kyvernov1.ForEachGeneration{{List: "targets", GeneratePattern: pattern}}
			_, err := gen.generateForeach()
			require.ErrorContains(t, err, "must equal policy namespace")
			switch mode {
			case "data":
				require.Equal(t, []string{"get:tenant/copy", "create:tenant"}, client.calls)
			case "clone":
				require.Equal(t, []string{"get:tenant/source", "update:tenant", "get:tenant/copy", "create:tenant"}, client.calls)
			case "cloneList":
				require.Equal(t, []string{"list:tenant", "list:tenant"}, client.calls)
			}
		})
	}
}
