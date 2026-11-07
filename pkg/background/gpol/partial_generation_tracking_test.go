package gpol

import (
	"errors"
	"testing"

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
				common.GeneratePolicyLabel:     policyName,
				common.GenerateTriggerNSLabel:  namespace,
				common.GenerateTriggerUIDLabel: "trigger-" + namespace,
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
