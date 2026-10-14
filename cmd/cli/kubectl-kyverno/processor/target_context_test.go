package processor

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/libs"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// liveContext stands in for the client-backed context used with --cluster: it is not a
// *libs.FakeContextProvider, so targetResourceContext treats it as a live cluster.
type liveContext struct {
	*libs.FakeContextProvider
}

func (c liveContext) Clone() libs.Context { return c }

func configMap(name string, labels map[string]string) *unstructured.Unstructured {
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": name, "namespace": "test-ns"},
	}}
	object.SetLabels(labels)
	return object
}

func testRESTMapper() meta.RESTMapper {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	mapper.Add(schema.GroupVersionKind{Version: "v1", Kind: "Secret"}, meta.RESTScopeNamespace)
	return mapper
}

// newLiveContext returns a stand-in live context holding the given ConfigMaps.
func newLiveContext(t *testing.T, objects ...*unstructured.Unstructured) liveContext {
	t.Helper()
	live := liveContext{libs.NewFakeContextProvider()}
	for _, object := range objects {
		require.NoError(t, live.AddResource(schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}, object))
	}
	return live
}

// TestTargetResourceContext_WithoutCluster checks that without a cluster the targets are registered in the
// processor's own fake context, which is returned as is.
func TestTargetResourceContext_WithoutCluster(t *testing.T) {
	fake := libs.NewFakeContextProvider()
	got := targetResourceContext(fake, []*unstructured.Unstructured{configMap("supplied", nil)}, testRESTMapper())
	assert.Same(t, fake, got)
	object, err := fake.GetResource("v1", "configmaps", "test-ns", "supplied")
	require.NoError(t, err)
	assert.Equal(t, "supplied", object.GetName())
}

// TestTargetResourceContext_WithCluster checks the overlay on a live context: supplied targets win over live
// objects with the same identity, live objects stay visible, lists merge both, and cloning keeps the targets.
func TestTargetResourceContext_WithCluster(t *testing.T) {
	live := newLiveContext(t, configMap("shared", map[string]string{"from": "live"}), configMap("live-only", map[string]string{"from": "live"}))
	targets := []*unstructured.Unstructured{
		configMap("shared", map[string]string{"from": "supplied"}),
		configMap("supplied-only", map[string]string{"from": "supplied"}),
	}
	ctx := targetResourceContext(live, targets, testRESTMapper())
	require.IsType(t, &targetOverlayContext{}, ctx)

	for name, from := range map[string]string{"shared": "supplied", "live-only": "live", "supplied-only": "supplied"} {
		object, err := ctx.GetResource("v1", "configmaps", "test-ns", name)
		require.NoError(t, err, name)
		assert.Equal(t, from, object.GetLabels()["from"], name)
	}
	_, err := ctx.GetResource("v1", "configmaps", "test-ns", "missing")
	assert.Error(t, err)

	listed := map[string]string{}
	list, err := ctx.ListResources("v1", "configmaps", "test-ns", nil)
	require.NoError(t, err)
	for _, item := range list.Items {
		_, duplicate := listed[item.GetName()]
		assert.False(t, duplicate, "%s listed twice", item.GetName())
		listed[item.GetName()] = item.GetLabels()["from"]
	}
	assert.Equal(t, map[string]string{"shared": "supplied", "live-only": "live", "supplied-only": "supplied"}, listed)

	// the label selector applies to the supplied targets as well
	filtered, err := ctx.ListResources("v1", "configmaps", "test-ns", map[string]string{"from": "supplied"})
	require.NoError(t, err)
	var names []string
	for _, item := range filtered.Items {
		if item.GetLabels()["from"] == "supplied" {
			names = append(names, item.GetName())
		}
	}
	assert.ElementsMatch(t, []string{"shared", "supplied-only"}, names)

	// a kind the cluster has no objects of still lists the supplied targets
	secret := configMap("supplied-secret", nil)
	secret.SetKind("Secret")
	withSecret := targetResourceContext(newLiveContext(t), []*unstructured.Unstructured{secret}, testRESTMapper())
	secrets, err := withSecret.ListResources("v1", "secrets", "test-ns", nil)
	require.NoError(t, err)
	require.Len(t, secrets.Items, 1)
	assert.Equal(t, "supplied-secret", secrets.Items[0].GetName())

	clone := ctx.Clone()
	require.IsType(t, &targetOverlayContext{}, clone)
	object, err := clone.GetResource("v1", "configmaps", "test-ns", "supplied-only")
	require.NoError(t, err)
	assert.Equal(t, "supplied-only", object.GetName())
}

// TestDiscoverCELTargets_WithCluster checks that in cluster mode a CEL target expression selects a supplied
// target that does not exist in the cluster.
func TestDiscoverCELTargets_WithCluster(t *testing.T) {
	loaded := loadMutatingPolicies(t, celTargetPolicy("", "supplied-only", "cel"))
	require.Len(t, loaded.MutatingPolicies, 1)
	live := newLiveContext(t)
	secret := readUnstructured(t, "../../../../test/cli/test-mutating-policy/mutate-existing-secret-trigger-deployment-target/trigger.yaml")
	triggered := []string{mpolengine.PolicyKey(loaded.MutatingPolicies[0])}

	discover := func(ctx libs.Context) (map[string]map[string]bool, error) {
		provider, err := mpolengine.NewProvider(mpolcompiler.NewCompiler(), loaded.MutatingPolicies, nil, ctx)
		require.NoError(t, err)
		return discoverCELTargets(provider, ctx, secret, triggered)
	}

	// the live context alone cannot see the supplied target
	_, err := discover(live)
	require.Error(t, err)

	ctx := targetResourceContext(live, []*unstructured.Unstructured{configMap("supplied-only", nil)}, testRESTMapper())
	selected, err := discover(ctx)
	require.NoError(t, err)
	assert.True(t, selected[triggered[0]][celTargetKey(schema.GroupKind{Kind: "ConfigMap"}, "test-ns", "supplied-only")])
}
