package processor

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/libs"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// liveContext stands in for the client-backed context used with --cluster: it is not a
// *libs.FakeContextProvider, so targetResourceContext treats it as a live cluster. An empty namespace
// lists every namespace the stand-in holds objects in, as a cluster-wide list does.
type liveContext struct {
	*libs.FakeContextProvider
	namespaces []string
	listErr    error // returned by every list when set
}

func (c liveContext) Clone() libs.Context { return c }

// ListResources answers like an API server: NotFound for a kind it does not serve.
func (c liveContext) ListResources(apiVersion, resource, namespace string, labels map[string]string) (*unstructured.UnstructuredList, error) {
	if c.listErr != nil {
		return nil, c.listErr
	}
	list, err := c.list(apiVersion, resource, namespace, labels)
	if kerrors.IsBadRequest(err) {
		gv, _ := schema.ParseGroupVersion(apiVersion)
		return nil, kerrors.NewNotFound(gv.WithResource(resource).GroupResource(), "")
	}
	return list, err
}

func (c liveContext) list(apiVersion, resource, namespace string, labels map[string]string) (*unstructured.UnstructuredList, error) {
	if namespace != "" {
		return c.FakeContextProvider.ListResources(apiVersion, resource, namespace, labels)
	}
	out := &unstructured.UnstructuredList{}
	for _, ns := range c.namespaces {
		list, err := c.FakeContextProvider.ListResources(apiVersion, resource, ns, labels)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, list.Items...)
	}
	return out, nil
}

func configMap(name string, labels map[string]string) *unstructured.Unstructured {
	return configMapIn("test-ns", name, labels)
}

func configMapIn(namespace, name string, labels map[string]string) *unstructured.Unstructured {
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
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
	live := liveContext{FakeContextProvider: libs.NewFakeContextProvider()}
	seen := map[string]bool{}
	for _, object := range objects {
		require.NoError(t, live.AddResource(schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}, object))
		if !seen[object.GetNamespace()] {
			seen[object.GetNamespace()] = true
			live.namespaces = append(live.namespaces, object.GetNamespace())
		}
	}
	return live
}

// TestTargetResourceContext_WithoutCluster checks that without a cluster the targets are registered in the
// processor's own fake context, and that lists honour the label selector, which that context ignores.
func TestTargetResourceContext_WithoutCluster(t *testing.T) {
	fake := libs.NewFakeContextProvider()
	targets := []*unstructured.Unstructured{
		configMap("selected", map[string]string{"app": "x"}),
		configMap("other", map[string]string{"app": "y"}),
	}
	got := targetResourceContext(fake, targets, testRESTMapper())
	require.IsType(t, &labelFilterContext{}, got)
	object, err := fake.GetResource("v1", "configmaps", "test-ns", "selected")
	require.NoError(t, err)
	assert.Equal(t, "selected", object.GetName())

	all, err := got.ListResources("v1", "configmaps", "test-ns", nil)
	require.NoError(t, err)
	assert.Len(t, all.Items, 2)
	filtered, err := got.ListResources("v1", "configmaps", "test-ns", map[string]string{"app": "x"})
	require.NoError(t, err)
	require.Len(t, filtered.Items, 1)
	assert.Equal(t, "selected", filtered.Items[0].GetName())

	require.IsType(t, &labelFilterContext{}, got.Clone())
}

// TestTargetResourceContext_WithoutClusterListsEveryNamespace checks that without a cluster a cluster-wide
// list includes the supplied targets of every namespace, as an API server does, although the fake context
// only returns the objects stored under the empty namespace, and that a namespaced list holds each once.
func TestTargetResourceContext_WithoutClusterListsEveryNamespace(t *testing.T) {
	ctx := targetResourceContext(libs.NewFakeContextProvider(), []*unstructured.Unstructured{
		configMapIn("team-a", "selected", map[string]string{"app": "x"}),
		configMapIn("team-b", "other", map[string]string{"app": "y"}),
	}, testRESTMapper())
	listed := func(namespace string, labels map[string]string) []string {
		list, err := ctx.ListResources("v1", "configmaps", namespace, labels)
		require.NoError(t, err)
		var names []string
		for _, item := range list.Items {
			names = append(names, item.GetNamespace()+"/"+item.GetName())
		}
		return names
	}
	assert.ElementsMatch(t, []string{"team-a/selected", "team-b/other"}, listed("", nil))
	assert.Equal(t, []string{"team-a/selected"}, listed("", map[string]string{"app": "x"}))
	assert.Equal(t, []string{"team-a/selected"}, listed("team-a", nil))
}

// TestTargetResourceContext_PropagatesLiveListErrors checks that only a kind the cluster does not serve falls
// back to the supplied targets; any other list error, such as a rejected selector, is returned.
func TestTargetResourceContext_PropagatesLiveListErrors(t *testing.T) {
	live := newLiveContext(t)
	live.listErr = kerrors.NewBadRequest("invalid label selector")
	ctx := targetResourceContext(live, []*unstructured.Unstructured{configMap("supplied", nil)}, testRESTMapper())
	_, err := ctx.ListResources("v1", "configmaps", "test-ns", map[string]string{"bad key": "x"})
	require.Error(t, err)
	assert.True(t, kerrors.IsBadRequest(err), err.Error())
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

// TestTargetEngineContext checks that with a cluster the target policies read the live objects, not the
// supplied targets laid over them, while without a cluster they see the supplied targets.
func TestTargetEngineContext(t *testing.T) {
	live := newLiveContext(t, configMap("settings", map[string]string{"source": "live"}))
	targetContext := targetResourceContext(live, []*unstructured.Unstructured{configMap("settings", map[string]string{"source": "supplied"})}, testRESTMapper())

	for _, tt := range []struct {
		cluster bool
		want    string
	}{
		{cluster: true, want: "live"},
		{cluster: false, want: "supplied"},
	} {
		object, err := targetEngineContext(tt.cluster, live, targetContext).GetResource("v1", "configmaps", "test-ns", "settings")
		require.NoError(t, err)
		assert.Equal(t, tt.want, object.GetLabels()["source"], "cluster=%t", tt.cluster)
	}
}

// TestTargetResourceContext_SuppliedTargetReplacesStaleLiveCopy checks that a supplied target replaces the
// live object with the same identity in a list even when only the live copy still matches the selector.
func TestTargetResourceContext_SuppliedTargetReplacesStaleLiveCopy(t *testing.T) {
	live := newLiveContext(t, configMap("stale", map[string]string{"team": "a"}), configMap("other", map[string]string{"team": "a"}))
	// the supplied version of stale no longer carries the label
	ctx := targetResourceContext(live, []*unstructured.Unstructured{configMap("stale", nil)}, testRESTMapper())

	list, err := ctx.ListResources("v1", "configmaps", "test-ns", map[string]string{"team": "a"})
	require.NoError(t, err)
	var names []string
	for _, item := range list.Items {
		names = append(names, item.GetName())
	}
	assert.Equal(t, []string{"other"}, names, "the stale live copy must not stand in for the supplied target")
}

// TestTargetResourceContext_AllNamespaces checks that a cluster-wide list keeps objects that only share a
// name in different namespaces, and includes supplied targets from every namespace.
func TestTargetResourceContext_AllNamespaces(t *testing.T) {
	live := newLiveContext(t, configMapIn("ns-a", "cm", map[string]string{"from": "live"}))
	ctx := targetResourceContext(live, []*unstructured.Unstructured{configMapIn("ns-b", "cm", map[string]string{"from": "supplied"})}, testRESTMapper())

	list, err := ctx.ListResources("v1", "configmaps", "", nil)
	require.NoError(t, err)
	listed := map[string]string{}
	for _, item := range list.Items {
		listed[item.GetNamespace()+"/"+item.GetName()] = item.GetLabels()["from"]
	}
	assert.Equal(t, map[string]string{"ns-a/cm": "live", "ns-b/cm": "supplied"}, listed)

	for ns, from := range map[string]string{"ns-a": "live", "ns-b": "supplied"} {
		object, err := ctx.GetResource("v1", "configmaps", ns, "cm")
		require.NoError(t, err, ns)
		assert.Equal(t, from, object.GetLabels()["from"], ns)
	}
}

// TestTargetResourceContext_EdgeCases checks that an unmappable target is skipped, that an invalid
// apiVersion falls through to the live context, and that a kind without supplied targets lists the live
// objects unchanged.
func TestTargetResourceContext_EdgeCases(t *testing.T) {
	unmappable := configMap("unmappable", nil)
	unmappable.SetKind("NotARealKind")
	live := newLiveContext(t, configMap("live-only", nil))
	ctx := targetResourceContext(live, []*unstructured.Unstructured{unmappable}, testRESTMapper())
	overlay, ok := ctx.(*targetOverlayContext)
	require.True(t, ok)
	assert.Empty(t, overlay.targets, "a target the REST mapper cannot map must be skipped")

	// an invalid apiVersion matches no supplied target and is left to the live context to reject
	_, err := ctx.GetResource("not/a/valid/version", "configmaps", "test-ns", "live-only")
	require.Error(t, err)

	// no supplied ConfigMaps, so the live list comes back as it is
	list, err := ctx.ListResources("v1", "configmaps", "test-ns", nil)
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	assert.Equal(t, "live-only", list.Items[0].GetName())
}

// TestMatchesLabels checks that a label selector map is matched as the API server matches it.
func TestMatchesLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		selector map[string]string
		want     bool
	}{
		{name: "empty selector matches everything", labels: map[string]string{"app": "web"}, want: true},
		{name: "every key and value present", labels: map[string]string{"app": "web", "tier": "front"}, selector: map[string]string{"app": "web"}, want: true},
		{name: "other value", labels: map[string]string{"app": "db"}, selector: map[string]string{"app": "web"}},
		{name: "empty value needs the key", labels: map[string]string{"app": "web"}, selector: map[string]string{"select": ""}},
		{name: "empty value with the key", labels: map[string]string{"select": ""}, selector: map[string]string{"select": ""}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, matchesLabels(tt.labels, tt.selector))
		})
	}
}
