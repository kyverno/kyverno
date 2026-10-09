package generate

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type cloneSourceDiscovery struct {
	dclient.IDiscovery
	empty bool
}

func (d cloneSourceDiscovery) FindResources(group, version, kind, _ string) (map[dclient.TopLevelApiDescription]metav1.APIResource, error) {
	if d.empty {
		return nil, nil
	}
	if (group != "" && group != "*") || (version != "v1" && version != "*") {
		return nil, fmt.Errorf("unknown apiVersion %s/%s", group, version)
	}
	if kind != "ConfigMap" && kind != "Namespace" && kind != "Secret" {
		return nil, fmt.Errorf("unknown kind %s", kind)
	}
	return map[dclient.TopLevelApiDescription]metav1.APIResource{
		{GroupVersion: schema.GroupVersion{Group: group, Version: version}, Kind: kind}: {Namespaced: kind != "Namespace"},
	}, nil
}

// Record actual client operations, not just validation results. A rejected source
// must never be fetched, listed, or updated by the background controller.
type cloneSourceClient struct {
	dclient.Interface
	calls []string
}

func (c *cloneSourceClient) GetResource(_ context.Context, apiVersion, kind, namespace, name string, _ ...string) (*unstructured.Unstructured, error) {
	c.calls = append(c.calls, "get:"+namespace+"/"+name)
	if name != "source" {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: kind}, name)
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion, "kind": kind,
		"metadata": map[string]any{"namespace": namespace, "name": name, "uid": "source-uid"},
		"data":     map[string]any{"key": "value"},
	}}, nil
}

func (c *cloneSourceClient) ListResource(_ context.Context, _, _, namespace string, _ *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	c.calls = append(c.calls, "list:"+namespace)
	return &unstructured.UnstructuredList{}, nil
}

func (c *cloneSourceClient) UpdateResource(_ context.Context, _, _, namespace string, obj interface{}, _ bool, _ ...string) (*unstructured.Unstructured, error) {
	c.calls = append(c.calls, "update:"+namespace)
	return obj.(*unstructured.Unstructured), nil
}

func (c *cloneSourceClient) CreateResource(_ context.Context, _, _, namespace string, obj interface{}, _ bool) (*unstructured.Unstructured, error) {
	c.calls = append(c.calls, "create:"+namespace)
	return obj.(*unstructured.Unstructured), nil
}

func newCloneSourceGenerator(t *testing.T, pattern kyvernov1.GeneratePattern, namespaced bool, resolvedNamespace string) (*generator, *cloneSourceClient) {
	t.Helper()
	base := dclient.NewEmptyFakeClient()
	base.SetDiscovery(cloneSourceDiscovery{IDiscovery: base.Discovery()})
	client := &cloneSourceClient{Interface: base}
	trigger := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{"name": "trigger", "namespace": "tenant", "labels": map[string]any{"source": resolvedNamespace}},
	}}
	configuration := config.NewDefaultConfiguration(false)
	policyContext, err := engine.NewPolicyContext(jmespath.New(configuration), trigger, kyvernov1.Create, nil, configuration)
	require.NoError(t, err)
	var policy kyvernov1.PolicyInterface = &kyvernov1.ClusterPolicy{}
	if namespaced {
		policy = &kyvernov1.Policy{ObjectMeta: metav1.ObjectMeta{Namespace: "tenant"}}
	}
	rule := kyvernov1.Rule{Name: "clone", Generation: &kyvernov1.Generation{GeneratePattern: pattern}}
	policy.GetSpec().Rules = []kyvernov1.Rule{rule}
	gen := newGenerator(client, logr.Discard(), policyContext.WithPolicy(policy), policy, rule, nil, nil, trigger, pattern,
		func(context.Context, []kyvernov1.ContextEntry, enginecontext.Interface) error { return nil })
	return gen, client
}

func TestGenerateCloneSourceNamespaces(t *testing.T) {
	t.Parallel()
	for _, cloneList := range []bool{false, true} {
		for _, tt := range []struct {
			name       string
			namespace  string
			resolved   string
			namespaced bool
			wantErr    bool
			wantSource string
		}{
			{"same namespace", "tenant", "", true, false, "tenant"},
			{"foreign namespace", "other", "", true, true, ""},
			{"omitted namespace", "", "", true, true, ""},
			{"resolved same namespace", "{{request.object.metadata.labels.source}}", "tenant", true, false, "tenant"},
			{"resolved foreign namespace", "{{request.object.metadata.labels.source}}", "other", true, true, ""},
			{"resolved empty namespace", "{{request.object.metadata.labels.source}}", "", true, true, ""},
			{"cluster policy cross namespace", "other", "", false, false, "other"},
			{"cluster policy resolved namespace", "{{request.object.metadata.labels.source}}", "other", false, false, "other"},
		} {
			t.Run(fmt.Sprintf("cloneList=%t/%s", cloneList, tt.name), func(t *testing.T) {
				t.Parallel()
				pattern := kyvernov1.GeneratePattern{
					ResourceSpec: kyvernov1.ResourceSpec{APIVersion: "v1", Kind: "ConfigMap", Name: "copy", Namespace: "tenant"},
					Clone:        kyvernov1.CloneFrom{Namespace: tt.namespace, Name: "source"},
				}
				if cloneList {
					pattern.Clone = kyvernov1.CloneFrom{}
					pattern.CloneList = kyvernov1.CloneList{Namespace: tt.namespace, Kinds: []string{"v1/ConfigMap"}}
				}
				gen, client := newCloneSourceGenerator(t, pattern, tt.namespaced, tt.resolved)
				_, err := gen.generate()
				if tt.wantErr {
					require.ErrorContains(t, err, "must equal policy namespace")
					require.Empty(t, client.calls)
				} else {
					require.NoError(t, err)
					if cloneList {
						require.Equal(t, []string{"list:" + tt.wantSource}, client.calls)
					} else {
						require.Equal(t, []string{"get:" + tt.wantSource + "/source", "update:" + tt.wantSource, "get:tenant/copy", "create:tenant"}, client.calls)
					}
				}
				// Substitution must not mutate the cached policy pattern.
				require.Equal(t, pattern, gen.pattern)
			})
		}
	}
}

func TestGenerateCloneSourceScope(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name        string
		kind        string
		cloneKinds  []string
		empty       bool
		omitVersion bool
		wantErr     string
	}{
		{name: "cluster source", kind: "Namespace", wantErr: "must be namespaced"},
		{name: "resolved cluster source", kind: "{{'Namespace'}}", wantErr: "must be namespaced"},
		{name: "unknown scope", kind: "Missing", wantErr: "target scope"},
		{name: "empty discovery", kind: "ConfigMap", empty: true, wantErr: "cannot determine target scope"},
		{name: "subresource", kind: "ConfigMap/status", wantErr: "top-level target kind"},
		{name: "wildcard", kind: "*", wantErr: "top-level target kind"},
		{name: "later cluster kind", cloneKinds: []string{"v1/ConfigMap", "v1/Namespace"}, wantErr: "must be namespaced"},
		{name: "later unknown kind", cloneKinds: []string{"v1/ConfigMap", "v1/Missing"}, wantErr: "target scope"},
		{name: "all kinds permitted", cloneKinds: []string{"v1/ConfigMap", "v1/Secret"}},
		{name: "omitted clone apiVersion", kind: "ConfigMap", omitVersion: true},
		{name: "omitted cloneList apiVersion", cloneKinds: []string{"ConfigMap", "Secret"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pattern := kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{APIVersion: "v1", Kind: tt.kind, Name: "copy", Namespace: "tenant"},
				Clone:        kyvernov1.CloneFrom{Namespace: "tenant", Name: "source"},
			}
			if tt.omitVersion {
				pattern.APIVersion = ""
			}
			if tt.cloneKinds != nil {
				pattern.Clone = kyvernov1.CloneFrom{}
				pattern.CloneList = kyvernov1.CloneList{Namespace: "tenant", Kinds: tt.cloneKinds}
			}
			gen, client := newCloneSourceGenerator(t, pattern, true, "")
			if tt.empty {
				client.SetDiscovery(cloneSourceDiscovery{empty: true})
			}
			_, err := gen.generate()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Empty(t, client.calls, "validate every kind before reading any source")
			} else {
				require.NoError(t, err)
				if tt.cloneKinds != nil {
					require.Equal(t, []string{"list:tenant", "list:tenant"}, client.calls)
				} else {
					require.Equal(t, []string{"get:tenant/source", "update:tenant", "get:tenant/copy", "create:tenant"}, client.calls)
				}
			}
		})
	}
}

func TestGenerateForeachCloneSourceNamespaces(t *testing.T) {
	t.Parallel()
	for _, cloneList := range []bool{false, true} {
		t.Run(fmt.Sprintf("cloneList=%t", cloneList), func(t *testing.T) {
			t.Parallel()
			pattern := kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{APIVersion: "v1", Kind: "ConfigMap", Name: "copy", Namespace: "tenant"},
				Clone:        kyvernov1.CloneFrom{Namespace: "{{element.namespace}}", Name: "source"},
			}
			if cloneList {
				pattern.Clone = kyvernov1.CloneFrom{}
				pattern.CloneList = kyvernov1.CloneList{Namespace: "{{element.namespace}}", Kinds: []string{"v1/ConfigMap"}}
			}
			gen, client := newCloneSourceGenerator(t, pattern, true, "")
			require.NoError(t, gen.policyContext.JSONContext().AddVariable("sources", []any{
				map[string]any{"namespace": "tenant"}, map[string]any{"namespace": "other"},
			}))
			gen.forEach = []kyvernov1.ForEachGeneration{{List: "sources", GeneratePattern: pattern}}
			_, err := gen.generateForeach()
			require.ErrorContains(t, err, "must equal policy namespace")
			if cloneList {
				require.Equal(t, []string{"list:tenant"}, client.calls)
			} else {
				require.Equal(t, []string{"get:tenant/source", "update:tenant", "get:tenant/copy", "create:tenant"}, client.calls)
			}
		})
	}
}
