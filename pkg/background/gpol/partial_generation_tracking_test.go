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
		require.Equal(t, []schema.GroupVersionResource{newGVR}, wm.policyRefs[policyName])
		require.NotContains(t, wm.refCount, oldGVR)
		require.NotContains(t, wm.dynamicWatchers, oldGVR)
		require.Equal(t, 1, wm.refCount[newGVR], "a full retry must not double-count a partial registration")
		require.NoError(t, wm.SyncWatchers(policyName, nil, []*unstructured.Unstructured{newer}, false))
		require.Equal(t, []string{"Secret/ns/old"}, client.deleted, "repeated reconciliation must not delete twice")
		require.Equal(t, 1, wm.refCount[newGVR])
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
	// The mapped resource and the later unmapped acknowledged write are both
	// UID-qualified cleanup targets. Deletion order is not significant.
	require.ElementsMatch(t, []string{"ConfigMap/ns/first", "Secret/ns/second"}, client.deleted)
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

// TestRestoredLegacyDownstreamUsesVerifiedCacheOwner confirms that an old
// unmarked downstream can be tracked under the exact canonical policy key
// without permitting a same-name cluster policy to claim it.
func TestRestoredLegacyDownstreamUsesVerifiedCacheOwner(t *testing.T) {
	const namespace = "team-a"
	const policyName = "shared-name"
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	legacy := makeUnstructured("1", "", "v1", "ConfigMap", "old", namespace, "legacy-uid", map[string]string{
		common.GeneratePolicyLabel:     policyName,
		common.GenerateTriggerNSLabel:  namespace,
		common.GenerateTriggerUIDLabel: "trigger-uid",
	})
	client := &MockClient{}
	wm := &WatchManager{
		log:    logging.WithName("legacy-owner"),
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
	policyKey := namespace + "/" + policyName
	clusterLegacy := makeUnstructured("1", "", "v1", "ConfigMap", "cluster-old", namespace, "cluster-uid", legacy.GetLabels())
	require.NoError(t, wm.AddWatchers(policyKey, nil, []*unstructured.Unstructured{legacy}, false))
	require.NoError(t, wm.AddWatchers(policyName, nil, []*unstructured.Unstructured{clusterLegacy}, false))
	require.Equal(t, 2, wm.refCount[gvr])
	require.Equal(t, []*unstructured.Unstructured{legacy}, wm.GetDownstreams(policyKey))
	require.Equal(t, []*unstructured.Unstructured{clusterLegacy}, wm.GetDownstreams(policyName))
	wm.RemoveWatchersForPolicy(policyKey, true)
	require.Equal(t, []string{"ConfigMap/team-a/old"}, client.deleted)
	require.Contains(t, wm.dynamicWatchers[gvr].metadataCache, clusterLegacy.GetUID())
	require.Equal(t, 1, wm.refCount[gvr])
	wm.RemoveWatchersForPolicy(policyName, true)
	require.ElementsMatch(t, []string{"ConfigMap/team-a/old", "ConfigMap/team-a/cluster-old"}, client.deleted)
}

// The Namespace trigger is cluster-scoped but may belong to a policy scoped
// to that Namespace. Unmarked historical resources must not be inferred as
// belonging to a same-name cluster policy without explicit verification.
func TestMatchesPolicyKeyNamespaceTriggerRequiresScope(t *testing.T) {
	labels := map[string]string{
		common.GeneratePolicyLabel:       "shared",
		common.GeneratePolicyScopeLabel:  common.GeneratePolicyScopeNamespaced,
		common.GenerateTriggerNSLabel:    "",
		common.GenerateTriggerNameLabel:  "team-a",
		common.GenerateTriggerKindLabel:  "Namespace",
		common.GenerateTriggerGroupLabel: "",
	}
	require.True(t, matchesPolicyKey(labels, "team-a/shared"))
	require.False(t, matchesPolicyKey(labels, "team-b/shared"))
	require.False(t, matchesPolicyKey(labels, "shared"))
	delete(labels, common.GeneratePolicyScopeLabel)
	require.False(t, matchesPolicyKey(labels, "shared"))
	require.False(t, matchesPolicyKey(labels, "team-a/shared"))
}

func TestDownstreamKeyIgnoresServedVersion(t *testing.T) {
	v1 := schema.GroupVersionKind{Group: "example.io", Version: "v1alpha1", Kind: "Widget"}
	v2 := schema.GroupVersionKind{Group: "example.io", Version: "v1beta1", Kind: "Widget"}
	require.Equal(t, downstreamKey(v1, "team-a", "same"), downstreamKey(v2, "team-a", "same"))
}

// A CRD can serve the same object through two API versions. The old GVR's
// cache entry is obsolete, but the object itself is still desired.
func TestServedVersionWatcherReconciliationDoesNotDeleteDesiredUID(t *testing.T) {
	const policyKey = "served-version-policy"
	oldGVR := schema.GroupVersionResource{Group: "example.io", Version: "v1alpha1", Resource: "widgets"}
	newGVR := schema.GroupVersionResource{Group: "example.io", Version: "v1beta1", Resource: "widgets"}
	old := makeUnstructured("1", "example.io", "v1alpha1", "Widget", "widget", "team-a", "widget-uid", map[string]string{
		common.GeneratePolicyLabel: policyKey,
	})
	newer := old.DeepCopy()
	newer.SetAPIVersion("example.io/v1beta1")
	client := &MockClient{}
	wm := &WatchManager{
		log:    logging.WithName("served-version-reconcile"),
		client: client,
		restMapper: &mockRESTMapper{fn: func(gk schema.GroupKind, version string) (*meta.RESTMapping, error) {
			if version == "v1beta1" {
				return &meta.RESTMapping{Resource: newGVR}, nil
			}
			return &meta.RESTMapping{Resource: oldGVR}, nil
		}},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			oldGVR: {watcher: watch.MockWatcher{StopFunc: func() {}}, metadataCache: map[types.UID]Resource{
				old.GetUID(): {Name: old.GetName(), Namespace: old.GetNamespace(), Data: old, Labels: old.GetLabels(), PolicyKey: policyKey, Hash: "old-hash"},
			}},
			newGVR: {watcher: watch.MockWatcher{StopFunc: func() {}}, metadataCache: map[types.UID]Resource{}},
		},
		policyRefs: map[string][]schema.GroupVersionResource{policyKey: {oldGVR}},
		refCount:   map[schema.GroupVersionResource]int{oldGVR: 1},
	}
	defer wm.StopWatchers()
	require.NoError(t, wm.AddWatchers(policyKey, nil, []*unstructured.Unstructured{newer}, false))
	require.Empty(t, client.deleted)
	require.NoError(t, wm.SyncWatchers(policyKey, nil, []*unstructured.Unstructured{newer}, false))
	require.Empty(t, client.deleted, "old API-version cache pruning must not delete the same desired UID")
	require.NotContains(t, wm.dynamicWatchers, oldGVR)
	require.Contains(t, wm.dynamicWatchers[newGVR].metadataCache, newer.GetUID())
	require.Equal(t, 1, wm.refCount[newGVR])
}

// Even a failed watch setup must not discard an already acknowledged write
// before policy deletion can clean it up in the same controller process.
func TestPartialWatcherStartupFailureRetainsCleanupIdentity(t *testing.T) {
	const policyKey = "watch-failure-policy"
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	obj := makeUnstructured("", "", "v1", "ConfigMap", "generated", "team-a", "generated-uid", map[string]string{
		common.GeneratePolicyLabel: policyKey,
	})
	client := &MockClient{}
	wm := &WatchManager{
		log:    logging.WithName("watch-failure"),
		client: client,
		restMapper: &mockRESTMapper{fn: func(schema.GroupKind, string) (*meta.RESTMapping, error) {
			return &meta.RESTMapping{Resource: gvr}, nil
		}},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
		policyRefs:      map[string][]schema.GroupVersionResource{},
		refCount:        map[schema.GroupVersionResource]int{},
	}
	defer wm.StopWatchers()
	require.Error(t, wm.AddWatchers(policyKey, nil, []*unstructured.Unstructured{obj}, false), "empty resourceVersion must fail watch initialization")
	require.Equal(t, []schema.GroupVersionResource{gvr}, wm.policyRefs[policyKey])
	require.Contains(t, wm.dynamicWatchers[gvr].metadataCache, obj.GetUID())
	wm.RemoveWatchersForPolicy(policyKey, true)
	require.Equal(t, []string{"ConfigMap/team-a/generated"}, client.deleted)
}

// Mapping a generated kind can fail before any watcher exists. The identity
// must remain available to policy deletion without consulting the GVR cache.
func TestUnmappedAcknowledgedResourceSurvivesDiscoveryFailure(t *testing.T) {
	const policyKey = "unmapped-policy"
	resource := makeUnstructured("5", "", "v1", "ConfigMap", "old", "team-a", "old-uid",
		map[string]string{common.GeneratePolicyLabel: policyKey})
	client := &MockClient{}
	wm := &WatchManager{
		log:    logging.WithName("unmapped"),
		client: client,
		restMapper: &mockRESTMapper{fn: func(schema.GroupKind, string) (*meta.RESTMapping, error) {
			return nil, errors.New("discovery unavailable")
		}},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
		policyRefs:      map[string][]schema.GroupVersionResource{},
		refCount:        map[schema.GroupVersionResource]int{},
	}
	defer wm.StopWatchers()
	require.ErrorContains(t, wm.AddWatchers(policyKey, nil, []*unstructured.Unstructured{resource}, false), "discovery unavailable")
	require.Contains(t, wm.unmappedAcknowledged[policyKey], resource.GetUID())
	require.Empty(t, wm.policyRefs[policyKey])
	wm.RemoveWatchersForPolicy(policyKey, true)
	require.Equal(t, []string{"ConfigMap/team-a/old"}, client.deleted)
	require.NotContains(t, wm.unmappedAcknowledged, policyKey)
}

func TestForgetDownstreamUIDReleasesOnlyUnusedWatcherReference(t *testing.T) {
	const policyKey = "release-policy"
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	first := makeUnstructured("1", "", "v1", "ConfigMap", "first", "team-a", "uid-first",
		map[string]string{common.GeneratePolicyLabel: policyKey})
	second := makeUnstructured("2", "", "v1", "ConfigMap", "second", "team-a", "uid-second",
		map[string]string{common.GeneratePolicyLabel: policyKey})
	wm := &WatchManager{
		log: logging.WithName("release-ref"),
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			gvr: {watcher: watch.MockWatcher{StopFunc: func() {}}, metadataCache: map[types.UID]Resource{
				first.GetUID():  {Name: first.GetName(), Data: first, Labels: first.GetLabels(), PolicyKey: policyKey},
				second.GetUID(): {Name: second.GetName(), Data: second, Labels: second.GetLabels(), PolicyKey: policyKey},
			}},
		},
		policyRefs: map[string][]schema.GroupVersionResource{policyKey: {gvr}},
		refCount:   map[schema.GroupVersionResource]int{gvr: 1},
	}
	wm.ForgetDownstreamUID(policyKey, first.GetUID())
	require.Equal(t, 1, wm.refCount[gvr])
	require.Contains(t, wm.dynamicWatchers[gvr].metadataCache, second.GetUID())
	wm.ForgetDownstreamUID(policyKey, second.GetUID())
	require.NotContains(t, wm.refCount, gvr)
	require.NotContains(t, wm.dynamicWatchers, gvr)
	require.NotContains(t, wm.policyRefs, policyKey)
}

// TestUnmappedAcknowledgementsAreTriggerScopedAndRetryable checks that
// discovery failures cannot orphan acknowledged writes when one trigger is
// deleted, and that transient and UID-conflict deletes preserve retry state.
func TestUnmappedAcknowledgementsAreTriggerScopedAndRetryable(t *testing.T) {
	const policyKey = "unmapped-trigger-policy"
	first := makeUnstructured("1", "", "v1", "ConfigMap", "first", "team-a", "first-uid", map[string]string{
		common.GeneratePolicyLabel:     policyKey,
		common.GenerateTriggerUIDLabel: "first-trigger",
	})
	sibling := makeUnstructured("1", "", "v1", "ConfigMap", "sibling", "team-a", "sibling-uid", map[string]string{
		common.GeneratePolicyLabel:     policyKey,
		common.GenerateTriggerUIDLabel: "sibling-trigger",
	})
	firstKey := "ConfigMap/team-a/first"
	siblingKey := "ConfigMap/team-a/sibling"
	client := &partialReproClient{objects: map[string]*unstructured.Unstructured{
		firstKey: first, siblingKey: sibling,
	}}
	wm := &WatchManager{
		log:    logging.WithName("unmapped-trigger-cleanup"),
		client: client,
		restMapper: &mockRESTMapper{fn: func(schema.GroupKind, string) (*meta.RESTMapping, error) {
			return nil, errors.New("temporary discovery outage")
		}},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
		policyRefs:      map[string][]schema.GroupVersionResource{},
		refCount:        map[schema.GroupVersionResource]int{},
	}
	require.ErrorContains(t, wm.AddWatchers(policyKey, nil, []*unstructured.Unstructured{first, sibling}, false), "temporary discovery outage")
	require.Len(t, wm.unmappedAcknowledged[policyKey], 2)
	require.Empty(t, wm.policyRefs[policyKey])

	client.deleteErr = errors.New("temporary delete outage")
	wm.DeleteDownstreams(policyKey, &kyverno.ResourceSpec{UID: "first-trigger"})
	require.Contains(t, wm.unmappedAcknowledged[policyKey], first.GetUID(), "failed deletion must retain retry identity")
	require.Contains(t, wm.unmappedAcknowledged[policyKey], sibling.GetUID(), "sibling trigger must remain untouched")
	require.Zero(t, client.deletes)

	client.deleteErr = nil
	replacement := first.DeepCopy()
	replacement.SetUID("replacement-uid")
	client.objects[firstKey] = replacement
	wm.DeleteDownstreams(policyKey, &kyverno.ResourceSpec{UID: "first-trigger"})
	require.Contains(t, wm.unmappedAcknowledged[policyKey], first.GetUID(), "UID conflict must preserve the recorded identity")
	require.Equal(t, replacement.GetUID(), client.objects[firstKey].GetUID())
	require.Zero(t, client.deletes, "a same-name replacement must survive")

	client.objects[firstKey] = first
	wm.DeleteDownstreams(policyKey, &kyverno.ResourceSpec{UID: "first-trigger"})
	require.NotContains(t, wm.unmappedAcknowledged[policyKey], first.GetUID())
	require.Contains(t, wm.unmappedAcknowledged[policyKey], sibling.GetUID())
	require.Equal(t, 1, client.deletes)
	require.Equal(t, first.GetUID(), *client.deleteOptions[len(client.deleteOptions)-1].Preconditions.UID)
	require.Contains(t, client.objects, siblingKey)

	wm.DeleteDownstreams(policyKey, &kyverno.ResourceSpec{UID: "sibling-trigger"})
	require.NotContains(t, wm.unmappedAcknowledged, policyKey)
	require.Equal(t, 2, client.deletes)
	require.NotContains(t, client.objects, siblingKey)
}
