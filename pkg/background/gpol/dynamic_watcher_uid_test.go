package gpol

import (
	"context"
	"errors"
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// The mocked API server rejects a stale UID even when the replacement has the
// same namespace/name. Without preconditions, it instead deletes that object.
func TestDeleteCachedDownstreamUIDPreconditions(t *testing.T) {
	const oldUID types.UID = "old-uid"
	const replacementUID types.UID = "replacement-uid"
	gr := schema.GroupResource{Resource: "configmaps"}
	conflict := apierrors.NewConflict(gr, "settings", errors.New("UID precondition failed"))
	notFound := apierrors.NewNotFound(gr, "settings")
	unavailable := apierrors.NewServiceUnavailable("API unavailable")

	tests := []struct {
		name        string
		cacheUID    types.UID
		dataUID     types.UID
		liveUID     types.UID
		nilData     bool
		deleteErr   error
		getErr      error
		wantDeletes int
		wantGets    int
		wantCached  bool
		wantLiveUID types.UID
	}{
		{name: "deletes original UID", cacheUID: oldUID, dataUID: oldUID, liveUID: oldUID, wantDeletes: 1},
		{name: "NotFound retires entry", cacheUID: oldUID, dataUID: oldUID, deleteErr: notFound, wantDeletes: 1},
		{name: "same-named replacement survives", cacheUID: oldUID, dataUID: oldUID, liveUID: replacementUID, wantDeletes: 1, wantLiveUID: replacementUID},
		{name: "UID conflict retires stale cache without GET", cacheUID: oldUID, dataUID: oldUID, deleteErr: conflict, wantDeletes: 1},
		{name: "UID conflict does not require live verification", cacheUID: oldUID, dataUID: oldUID, liveUID: replacementUID, deleteErr: conflict, getErr: unavailable, wantDeletes: 1, wantLiveUID: replacementUID},
		{name: "transient delete failure retains entry", cacheUID: oldUID, dataUID: oldUID, liveUID: oldUID, deleteErr: unavailable, wantDeletes: 1, wantCached: true, wantLiveUID: oldUID},
		{name: "empty cached UID cannot delete", cacheUID: "", dataUID: oldUID, liveUID: replacementUID, wantCached: true, wantLiveUID: replacementUID},
		{name: "nil cached resource cannot delete", cacheUID: oldUID, nilData: true, liveUID: replacementUID, wantCached: true, wantLiveUID: replacementUID},
		{name: "desired data has cleared UID", cacheUID: oldUID, dataUID: "", liveUID: oldUID, wantDeletes: 1},
		{name: "rekeyed entry uses cache UID", cacheUID: replacementUID, dataUID: oldUID, liveUID: replacementUID, wantDeletes: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", tt.dataUID, nil)
			if tt.nilData {
				data = nil
			}
			res := Resource{Name: "settings", Namespace: "app", Data: data}
			cache := map[types.UID]Resource{tt.cacheUID: res}
			liveUID := tt.liveUID
			deletes, gets := 0, 0
			var requestedUID types.UID
			client := &MockClient{
				deleteFn: func(_ context.Context, apiVersion, kind, namespace, name string, dryRun bool, options metav1.DeleteOptions) error {
					deletes++
					assert.Equal(t, "v1", apiVersion)
					assert.Equal(t, "ConfigMap", kind)
					assert.Equal(t, "app", namespace)
					assert.Equal(t, "settings", name)
					assert.False(t, dryRun)
					if options.Preconditions == nil || options.Preconditions.UID == nil {
						// Demonstrate the original wrong-object deletion.
						liveUID = ""
						return nil
					}
					requestedUID = *options.Preconditions.UID
					if tt.deleteErr != nil {
						return tt.deleteErr
					}
					if liveUID != "" && liveUID != requestedUID {
						return apierrors.NewConflict(gr, name, errors.New("UID precondition failed"))
					}
					liveUID = ""
					return nil
				},
				getFn: func(_ context.Context, _, _, _, _ string) (*unstructured.Unstructured, error) {
					gets++
					if tt.getErr != nil {
						return nil, tt.getErr
					}
					if liveUID == "" {
						return nil, notFound
					}
					return makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", liveUID, nil), nil
				},
			}
			wm := &WatchManager{log: logging.WithName("test"), client: client}
			w := &watcher{metadataCache: cache}
			wm.deleteCachedDownstream(wm.log, w, tt.cacheUID, res)

			assert.Equal(t, tt.wantDeletes, deletes)
			assert.Equal(t, tt.wantGets, gets)
			if tt.wantDeletes != 0 {
				assert.Equal(t, tt.cacheUID, requestedUID, "DELETE must include observed cached UID")
			}
			_, exists := cache[tt.cacheUID]
			assert.Equal(t, tt.wantCached, exists)
			assert.Equal(t, tt.wantLiveUID, liveUID, "wrong-object deletion must never occur")
		})
	}
}

// Verify all cleanup entry points use the same guarded DELETE, including a
// removed GVR and the optional policy-removal path.
func TestDownstreamCleanupPathsPreserveSameNamedReplacement(t *testing.T) {
	for _, path := range []string{"trigger deletion", "stale desired set", "old GVR", "policy removal"} {
		t.Run(path, func(t *testing.T) {
			const oldUID types.UID = "old-uid"
			const newUID types.UID = "replacement-uid"
			gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
			labels := map[string]string{
				common.GeneratePolicyLabel:     "policy",
				common.GenerateTriggerUIDLabel: "trigger",
			}
			obj := makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", oldUID, labels)
			entry := Resource{Name: "settings", Namespace: "app", Labels: labels, Data: obj}
			w := &watcher{metadataCache: map[types.UID]Resource{oldUID: entry}}
			liveUID := newUID
			deletes, gets := 0, 0
			client := &MockClient{
				deleteFn: func(_ context.Context, _, _, _, _ string, _ bool, opts metav1.DeleteOptions) error {
					deletes++
					if opts.Preconditions == nil || opts.Preconditions.UID == nil {
						liveUID = ""
						return nil
					}
					assert.Equal(t, oldUID, *opts.Preconditions.UID)
					if liveUID != *opts.Preconditions.UID {
						return apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, "settings", errors.New("UID mismatch"))
					}
					liveUID = ""
					return nil
				},
				getFn: func(_ context.Context, _, _, _, _ string) (*unstructured.Unstructured, error) {
					gets++
					return makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", liveUID, nil), nil
				},
			}
			wm := &WatchManager{
				client:          client,
				log:             logging.WithName("test"),
				policyRefs:      map[string][]schema.GroupVersionResource{"policy": {gvr}},
				dynamicWatchers: map[schema.GroupVersionResource]*watcher{gvr: w},
				refCount:        map[schema.GroupVersionResource]int{gvr: 1},
			}
			trigger := &kyverno.ResourceSpec{UID: "trigger"}
			switch path {
			case "trigger deletion":
				wm.DeleteDownstreams("policy", trigger)
			case "stale desired set":
				wm.CleanupStaleDownstreams("policy", trigger, nil)
			case "old GVR":
				require.NoError(t, wm.SyncWatchers("policy", trigger, nil, false))
			case "policy removal":
				wm.RemoveWatchersForPolicy("policy", true)
			}
			assert.Equal(t, 1, deletes)
			assert.Equal(t, 0, gets, "UID-precondition conflicts must not trigger GET")
			assert.Equal(t, newUID, liveUID, "replacement must survive cleanup")
			assert.NotContains(t, w.metadataCache, oldUID)
		})
	}
}

// An incomplete cache entry cannot be classified safely without its GVK.
// Skip it without network reads, while still removing other stale resources.
// This also guards against mutating the cache during map iteration.
func TestCleanupStaleDownstreamsNilData(t *testing.T) {
	const incompleteUID types.UID = "incomplete-uid"
	const staleUID1 types.UID = "stale-uid-1"
	const staleUID2 types.UID = "stale-uid-2"
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	labels := map[string]string{
		common.GeneratePolicyLabel:     "policy",
		common.GenerateTriggerUIDLabel: "trigger",
	}

	for _, tt := range []struct {
		name    string
		desired bool
	}{
		{name: "incomplete resource is stale"},
		{name: "incomplete resource is still desired", desired: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cache := map[types.UID]Resource{
				incompleteUID: {
					Name:      "settings",
					Namespace: "app",
					Labels:    labels,
				},
				staleUID1: {
					Name:      "old-settings-1",
					Namespace: "app",
					Labels:    labels,
					Data:      makeUnstructured("", "", "v1", "ConfigMap", "old-settings-1", "app", staleUID1, labels),
				},
				staleUID2: {
					Name:      "old-settings-2",
					Namespace: "app",
					Labels:    labels,
					Data:      makeUnstructured("", "", "v1", "ConfigMap", "old-settings-2", "app", staleUID2, labels),
				},
			}
			deletes := 0
			client := &MockClient{
				getFn: func(_ context.Context, _, _, _, _ string) (*unstructured.Unstructured, error) {
					t.Error("stale cleanup must not issue a GET while holding wm.lock")
					return nil, nil
				},
				deleteFn: func(_ context.Context, apiVersion, kind, namespace, name string, _ bool, opts metav1.DeleteOptions) error {
					deletes++
					assert.Equal(t, "v1", apiVersion)
					assert.Equal(t, "ConfigMap", kind)
					assert.Equal(t, "app", namespace)
					assert.Contains(t, []string{"old-settings-1", "old-settings-2"}, name)
					require.NotNil(t, opts.Preconditions)
					require.NotNil(t, opts.Preconditions.UID)
					assert.Contains(t, []types.UID{staleUID1, staleUID2}, *opts.Preconditions.UID)
					return nil
				},
			}
			w := &watcher{metadataCache: cache}
			wm := &WatchManager{
				client:          client,
				log:             logging.WithName("test"),
				policyRefs:      map[string][]schema.GroupVersionResource{"policy": {gvr}},
				dynamicWatchers: map[schema.GroupVersionResource]*watcher{gvr: w},
			}
			var desired []*unstructured.Unstructured
			if tt.desired {
				desired = []*unstructured.Unstructured{
					makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", incompleteUID, labels),
				}
			}
			wm.CleanupStaleDownstreams("policy", &kyverno.ResourceSpec{UID: "trigger"}, desired)
			assert.Equal(t, 2, deletes, "valid stale entries must not be skipped")
			require.Contains(t, cache, incompleteUID, "incomplete entry must remain retryable")
			assert.Nil(t, cache[incompleteUID].Data)
			assert.NotContains(t, cache, staleUID1)
			assert.NotContains(t, cache, staleUID2)
		})
	}
}
