package gpol

import (
	"errors"
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

// An incomplete evaluation may add resources of a new kind, but must not
// discard previously tracked kinds until a complete evaluation succeeds.
func TestAddWatchersPreservesPriorGVRUntilSuccessfulRetry(t *testing.T) {
	policyName := "partial-policy"
	oldGVR := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	newGVR := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	labels := map[string]string{
		common.GeneratePolicyLabel:     policyName,
		common.GenerateTriggerUIDLabel: "trigger-uid",
	}
	old := makeUnstructured("1", "", "v1", "Secret", "old", "ns", types.UID("old-uid"), labels)
	newer := makeUnstructured("1", "", "v1", "ConfigMap", "new", "ns", types.UID("new-uid"), labels)

	setup := func() (*WatchManager, *MockClient) {
		client := &MockClient{}
		wm := &WatchManager{
			log:    logging.WithName("partial-generation"),
			client: client,
			restMapper: &mockRESTMapper{fn: func(gk schema.GroupKind, version string) (*meta.RESTMapping, error) {
				if gk.Kind == "ConfigMap" {
					return &meta.RESTMapping{Resource: newGVR}, nil
				}
				return &meta.RESTMapping{Resource: oldGVR}, nil
			}},
			dynamicWatchers: map[schema.GroupVersionResource]*watcher{
				oldGVR: {
					watcher: watch.MockWatcher{StopFunc: func() {}},
					metadataCache: map[types.UID]Resource{
						old.GetUID(): {Name: old.GetName(), Namespace: old.GetNamespace(), Labels: old.GetLabels(), Hash: "old-hash", Data: old},
					},
				},
				newGVR: {watcher: watch.MockWatcher{StopFunc: func() {}}, metadataCache: map[types.UID]Resource{}},
			},
			policyRefs: map[string][]schema.GroupVersionResource{policyName: {oldGVR}},
			refCount:   map[schema.GroupVersionResource]int{oldGVR: 1},
		}
		return wm, client
	}

	t.Run("partial failure retains both for policy deletion", func(t *testing.T) {
		wm, client := setup()
		for i := 0; i < 2; i++ {
			require.NoError(t, wm.AddWatchers(policyName, nil, []*unstructured.Unstructured{newer}, false))
		}
		require.Equal(t, 1, wm.refCount[oldGVR])
		require.Equal(t, 1, wm.refCount[newGVR])
		require.Contains(t, wm.dynamicWatchers[oldGVR].metadataCache, old.GetUID())
		require.Contains(t, wm.dynamicWatchers[newGVR].metadataCache, newer.GetUID())
		require.Empty(t, client.deleted)

		wm.RemoveWatchersForPolicy(policyName, true)
		require.ElementsMatch(t, []string{"Secret/ns/old", "ConfigMap/ns/new"}, client.deleted)
	})

	t.Run("successful retry may prune obsolete kind", func(t *testing.T) {
		wm, client := setup()
		require.NoError(t, wm.AddWatchers(policyName, nil, []*unstructured.Unstructured{newer}, false))
		require.Empty(t, client.deleted)
		require.NoError(t, wm.SyncWatchers(policyName, nil, []*unstructured.Unstructured{newer}, false))
		require.Equal(t, []string{"Secret/ns/old"}, client.deleted)
		require.Contains(t, wm.dynamicWatchers[newGVR].metadataCache, newer.GetUID())
	})
}

// NamespacedGeneratingPolicy UpdateRequests use namespace/name, while CEL
// downstreams intentionally store only the bare policy name and the trigger
// namespace in labels. A partial write must remain owned by exactly one policy.
func TestAddWatchersNamespacedPolicyKeyKeepsNamespacesIsolated(t *testing.T) {
	const policyName = "generate-config"
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	client := &MockClient{}
	wm := &WatchManager{
		log:    logging.WithName("namespaced-partial-generation"),
		client: client,
		restMapper: &mockRESTMapper{fn: func(schema.GroupKind, string) (*meta.RESTMapping, error) {
			return &meta.RESTMapping{Resource: gvr}, nil
		}},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			gvr: {watcher: watch.MockWatcher{StopFunc: func() {}}, metadataCache: map[types.UID]Resource{}},
		},
		policyRefs: map[string][]schema.GroupVersionResource{},
		refCount:   map[schema.GroupVersionResource]int{},
	}
	defer wm.StopWatchers()

	newDownstream := func(namespace string) *unstructured.Unstructured {
		return makeUnstructured("1", "", "v1", "ConfigMap", "generated", namespace,
			types.UID("generated-"+namespace), map[string]string{
				common.GeneratePolicyLabel:      policyName,
				common.GeneratePolicyScopeLabel: common.GeneratePolicyScopeNamespaced,
				common.GenerateTriggerNSLabel:   namespace,
				common.GenerateTriggerUIDLabel:  "trigger-" + namespace,
			})
	}
	first, second := newDownstream("team-a"), newDownstream("team-b")
	require.NoError(t, wm.AddWatchers("team-a/"+policyName, nil, []*unstructured.Unstructured{first}, false))
	require.NoError(t, wm.AddWatchers("team-b/"+policyName, nil, []*unstructured.Unstructured{second}, false))
	require.Equal(t, 2, wm.refCount[gvr])
	require.Equal(t, []*unstructured.Unstructured{first}, wm.GetDownstreams("team-a/"+policyName))
	require.Equal(t, []*unstructured.Unstructured{second}, wm.GetDownstreams("team-b/"+policyName))

	wm.RemoveWatchersForPolicy("team-a/"+policyName, true)
	require.Equal(t, []string{"ConfigMap/team-a/generated"}, client.deleted)
	require.Equal(t, 1, wm.refCount[gvr])
	require.Contains(t, wm.dynamicWatchers[gvr].metadataCache, second.GetUID())

	wm.RemoveWatchersForPolicy("team-b/"+policyName, true)
	require.ElementsMatch(t, []string{"ConfigMap/team-a/generated", "ConfigMap/team-b/generated"}, client.deleted)
}

func TestAddWatchersPreservesSuccessfulRegistrationOnLaterMappingError(t *testing.T) {
	const policyName = "partial-policy"
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	labels := map[string]string{
		common.GeneratePolicyLabel:     policyName,
		common.GenerateTriggerUIDLabel: "trigger-uid",
	}
	first := makeUnstructured("1", "", "v1", "ConfigMap", "first", "ns", "uid-first", labels)
	second := makeUnstructured("1", "", "v1", "Secret", "second", "ns", "uid-second", labels)
	client := &MockClient{}
	wm := &WatchManager{
		log:    logging.WithName("registration-error"),
		client: client,
		restMapper: &mockRESTMapper{fn: func(gk schema.GroupKind, _ string) (*meta.RESTMapping, error) {
			if gk.Kind == "Secret" {
				return nil, errors.New("second GVR mapping failed")
			}
			return &meta.RESTMapping{Resource: gvr}, nil
		}},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			gvr: {watcher: watch.MockWatcher{StopFunc: func() {}}, metadataCache: map[types.UID]Resource{}},
		},
		policyRefs: map[string][]schema.GroupVersionResource{},
		refCount:   map[schema.GroupVersionResource]int{},
	}
	defer wm.StopWatchers()
	err := wm.AddWatchers(policyName, nil, []*unstructured.Unstructured{first, second}, false)
	require.ErrorContains(t, err, "second GVR mapping failed")
	require.Equal(t, 1, wm.refCount[gvr])
	require.Equal(t, []schema.GroupVersionResource{gvr}, wm.policyRefs[policyName])
	require.Equal(t, []*unstructured.Unstructured{first}, wm.GetDownstreams(policyName))
	wm.RemoveWatchersForPolicy(policyName, true)
	require.Equal(t, []string{"ConfigMap/ns/first"}, client.deleted)
}

func TestPartialRetryPreservesSiblingTrigger(t *testing.T) {
	const policyName = "shared-policy"
	oldGVR := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	newGVR := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	labelsFor := func(triggerUID string) map[string]string {
		return map[string]string{
			common.GeneratePolicyLabel:     policyName,
			common.GenerateTriggerUIDLabel: triggerUID,
		}
	}
	old := makeUnstructured("1", "", "v1", "Secret", "old", "ns", "old-uid", labelsFor("first"))
	sibling := makeUnstructured("1", "", "v1", "Secret", "sibling", "ns", "sibling-uid", labelsFor("sibling"))
	newer := makeUnstructured("1", "", "v1", "ConfigMap", "new", "ns", "new-uid", labelsFor("first"))
	client := &MockClient{}
	wm := &WatchManager{
		log:    logging.WithName("partial-sibling"),
		client: client,
		restMapper: &mockRESTMapper{fn: func(gk schema.GroupKind, _ string) (*meta.RESTMapping, error) {
			if gk.Kind == "ConfigMap" {
				return &meta.RESTMapping{Resource: newGVR}, nil
			}
			return &meta.RESTMapping{Resource: oldGVR}, nil
		}},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			oldGVR: {
				watcher: watch.MockWatcher{StopFunc: func() {}},
				metadataCache: map[types.UID]Resource{
					old.GetUID():     {Name: old.GetName(), Namespace: old.GetNamespace(), Labels: old.GetLabels(), Hash: "old", Data: old},
					sibling.GetUID(): {Name: sibling.GetName(), Namespace: sibling.GetNamespace(), Labels: sibling.GetLabels(), Hash: "sibling", Data: sibling},
				},
			},
			newGVR: {watcher: watch.MockWatcher{StopFunc: func() {}}, metadataCache: map[types.UID]Resource{}},
		},
		policyRefs: map[string][]schema.GroupVersionResource{policyName: {oldGVR}},
		refCount:   map[schema.GroupVersionResource]int{oldGVR: 1},
	}
	defer wm.StopWatchers()

	trigger := &kyverno.ResourceSpec{UID: "first"}
	require.NoError(t, wm.AddWatchers(policyName, trigger, []*unstructured.Unstructured{newer}, false))
	require.NoError(t, wm.SyncWatchers(policyName, trigger, []*unstructured.Unstructured{newer}, false))
	wm.CleanupStaleDownstreams(policyName, trigger, []*unstructured.Unstructured{newer})

	require.Equal(t, []string{"Secret/ns/old"}, client.deleted)
	require.Contains(t, wm.dynamicWatchers[oldGVR].metadataCache, sibling.GetUID())
	require.Contains(t, wm.dynamicWatchers[newGVR].metadataCache, newer.GetUID())
	require.Equal(t, 1, wm.refCount[oldGVR])
	require.Equal(t, 1, wm.refCount[newGVR])
	require.ElementsMatch(t, []schema.GroupVersionResource{oldGVR, newGVR}, wm.policyRefs[policyName])
}

// TestAddWatchersDifferentPolicyScopesWithSameName ensures cleanup does not
// cross policy boundaries for same-name, same-trigger-namespace resources.
func TestAddWatchersDifferentPolicyScopesWithSameName(t *testing.T) {
	const policyName = "same-name"
	const namespace = "team-a"
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	labelsFor := func(scope string) map[string]string {
		return map[string]string{
			common.GeneratePolicyLabel:      policyName,
			common.GeneratePolicyScopeLabel: scope,
			common.GenerateTriggerNSLabel:   namespace,
			common.GenerateTriggerUIDLabel:  "trigger-uid",
		}
	}
	cluster := makeUnstructured("1", "", "v1", "ConfigMap", "cluster-result", namespace, "cluster-uid", labelsFor(common.GeneratePolicyScopeCluster))
	namespaced := makeUnstructured("1", "", "v1", "ConfigMap", "namespaced-result", namespace, "namespaced-uid", labelsFor(common.GeneratePolicyScopeNamespaced))
	client := &MockClient{}
	wm := &WatchManager{
		log:    logging.WithName("scope-isolation"),
		client: client,
		restMapper: &mockRESTMapper{fn: func(schema.GroupKind, string) (*meta.RESTMapping, error) {
			return &meta.RESTMapping{Resource: gvr}, nil
		}},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			gvr: {watcher: watch.MockWatcher{StopFunc: func() {}}, metadataCache: map[types.UID]Resource{}},
		},
		policyRefs: map[string][]schema.GroupVersionResource{},
		refCount:   map[schema.GroupVersionResource]int{},
	}
	defer wm.StopWatchers()
	require.NoError(t, wm.AddWatchers(policyName, nil, []*unstructured.Unstructured{cluster}, false))
	require.NoError(t, wm.AddWatchers(namespace+"/"+policyName, nil, []*unstructured.Unstructured{namespaced}, false))
	require.Equal(t, 2, wm.refCount[gvr])
	require.Equal(t, []*unstructured.Unstructured{cluster}, wm.GetDownstreams(policyName))
	require.Equal(t, []*unstructured.Unstructured{namespaced}, wm.GetDownstreams(namespace+"/"+policyName))

	wm.RemoveWatchersForPolicy(namespace+"/"+policyName, true)
	require.Equal(t, []string{"ConfigMap/team-a/namespaced-result"}, client.deleted)
	require.Contains(t, wm.dynamicWatchers[gvr].metadataCache, cluster.GetUID())
	require.Equal(t, 1, wm.refCount[gvr])
	require.Equal(t, []*unstructured.Unstructured{cluster}, wm.GetDownstreams(policyName))

	wm.RemoveWatchersForPolicy(policyName, true)
	require.ElementsMatch(t, []string{"ConfigMap/team-a/namespaced-result", "ConfigMap/team-a/cluster-result"}, client.deleted)
}

// TestUnmarkedLegacyPolicyCannotBeInferredFromTriggerNamespace proves
// that ambiguous old objects are never selected for destructive cleanup.
func TestUnmarkedLegacyPolicyCannotBeInferredFromTriggerNamespace(t *testing.T) {
	labels := map[string]string{
		common.GeneratePolicyLabel:    "same-name",
		common.GenerateTriggerNSLabel: "team-a",
	}
	require.False(t, matchesPolicyKey(labels, "same-name"))
	require.False(t, matchesPolicyKey(labels, "team-a/same-name"))
}
