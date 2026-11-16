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
		{name: "same-named replacement survives", cacheUID: oldUID, dataUID: oldUID, liveUID: replacementUID, wantDeletes: 1, wantCached: true, wantLiveUID: replacementUID},
		{name: "UID conflict retains retryable cache without GET", cacheUID: oldUID, dataUID: oldUID, deleteErr: conflict, wantDeletes: 1, wantCached: true},
		{name: "UID conflict does not require live verification", cacheUID: oldUID, dataUID: oldUID, liveUID: replacementUID, deleteErr: conflict, getErr: unavailable, wantDeletes: 1, wantCached: true, wantLiveUID: replacementUID},
		{name: "webhook conflict with original UID keeps retry state", cacheUID: oldUID, dataUID: oldUID, liveUID: oldUID, deleteErr: conflict, wantDeletes: 1, wantCached: true, wantLiveUID: oldUID},
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

// A webhook can return Conflict despite the original UID still being live.
// Keep the cache entry and retry the same UID-protected deletion later.
func TestDeleteCachedDownstreamRetryAfterWebhookConflict(t *testing.T) {
	const uid types.UID = "original-uid"
	res := Resource{
		Name:      "settings",
		Namespace: "app",
		Data:      makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", uid, nil),
	}
	w := &watcher{metadataCache: map[types.UID]Resource{uid: res}}
	attempts := 0
	client := &MockClient{
		deleteFn: func(_ context.Context, _, _, _, _ string, _ bool, opts metav1.DeleteOptions) error {
			attempts++
			require.NotNil(t, opts.Preconditions)
			require.NotNil(t, opts.Preconditions.UID)
			assert.Equal(t, uid, *opts.Preconditions.UID)
			if attempts == 1 {
				return apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, res.Name, errors.New("admission webhook denied delete"))
			}
			return nil
		},
		getFn: func(_ context.Context, _, _, _, _ string) (*unstructured.Unstructured, error) {
			t.Error("no live GET is needed to retry a protected deletion")
			return nil, nil
		},
	}
	wm := &WatchManager{log: logging.WithName("test"), client: client}

	wm.deleteCachedDownstream(wm.log, w, uid, res)
	require.Equal(t, 1, attempts)
	require.Contains(t, w.metadataCache, uid, "Conflict must not lose the original cleanup target")

	wm.deleteCachedDownstream(wm.log, w, uid, w.metadataCache[uid])
	require.Equal(t, 2, attempts)
	assert.NotContains(t, w.metadataCache, uid, "successful protected delete must retire the entry")
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
			if path == "policy removal" {
				// Policy teardown intentionally discards watcher state.
				assert.Equal(t, 0, gets)
			} else {
				assert.Equal(t, 1, gets, "conflict verification must occur after the DELETE")
			}
			assert.Equal(t, newUID, liveUID, "replacement must survive cleanup")
			assert.NotContains(t, w.metadataCache, oldUID, "confirmed replacement must retire the stale cache identity")
			if path == "old GVR" {
				assert.NotContains(t, wm.dynamicWatchers, gvr, "last stale entry must release its old watcher")
				assert.NotContains(t, wm.refCount, gvr, "last stale entry must release its old ref count")
				assert.Empty(t, wm.policyRefs["policy"], "old GVR must no longer be referenced")
			}
		})
	}
}

// A conflict may mean a replaced UID, a webhook denial, or a transient failure.
// Verify only outside the global lock and retire state only when identity changed.
func TestCleanupStaleDownstreamsVerifyConflictOutsideLock(t *testing.T) {
	const cachedUID types.UID = "original-uid"
	const replacementUID types.UID = "new-uid"
	gr := schema.GroupResource{Resource: "configmaps"}
	for _, tt := range []struct {
		name       string
		liveUID    types.UID
		getErr     error
		nilLive    bool
		wantCached bool
	}{
		{name: "replacement confirmed", liveUID: replacementUID},
		{name: "original no longer exists", getErr: apierrors.NewNotFound(gr, "settings")},
		{name: "webhook conflict with original live", liveUID: cachedUID, wantCached: true},
		{name: "GET service unavailable", getErr: apierrors.NewServiceUnavailable("temporary"), wantCached: true},
		{name: "GET returned no object", nilLive: true, wantCached: true},
		{name: "GET returned unknown UID", wantCached: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
			labels := map[string]string{
				common.GeneratePolicyLabel:     "policy",
				common.GenerateTriggerUIDLabel: "trigger",
			}
			res := Resource{
				Name:      "settings",
				Namespace: "app",
				Labels:    labels,
				Data:      makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", cachedUID, labels),
			}
			w := &watcher{metadataCache: map[types.UID]Resource{cachedUID: res}}
			var wm *WatchManager
			gets := 0
			client := &MockClient{
				deleteFn: func(_ context.Context, _, _, _, _ string, _ bool, opts metav1.DeleteOptions) error {
					require.NotNil(t, opts.Preconditions)
					require.NotNil(t, opts.Preconditions.UID)
					assert.Equal(t, cachedUID, *opts.Preconditions.UID)
					return apierrors.NewConflict(gr, "settings", errors.New("conflicting delete"))
				},
				getFn: func(_ context.Context, apiVersion, kind, namespace, name string) (*unstructured.Unstructured, error) {
					gets++
					if wm.lock.TryLock() {
						wm.lock.Unlock()
					} else {
						t.Error("live GET must execute outside the global watch manager lock")
					}
					assert.Equal(t, "v1", apiVersion)
					assert.Equal(t, "ConfigMap", kind)
					assert.Equal(t, "app", namespace)
					assert.Equal(t, "settings", name)
					if tt.getErr != nil {
						return nil, tt.getErr
					}
					if tt.nilLive {
						return nil, nil
					}
					return makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", tt.liveUID, nil), nil
				},
			}
			wm = &WatchManager{
				log:             logging.WithName("test"),
				client:          client,
				policyRefs:      map[string][]schema.GroupVersionResource{"policy": {gvr}},
				dynamicWatchers: map[schema.GroupVersionResource]*watcher{gvr: w},
			}
			wm.CleanupStaleDownstreams("policy", &kyverno.ResourceSpec{UID: "trigger"}, nil)
			assert.Equal(t, 1, gets)
			_, exists := w.metadataCache[cachedUID]
			assert.Equal(t, tt.wantCached, exists, "only verified stale UIDs should be evicted")
		})
	}
}

// If a watcher event updates the cached identity during an unlocked GET, a
// delayed conflict verification must not erase the newer cache entry.
func TestCleanupStaleDownstreamsConflictVerificationRechecksCache(t *testing.T) {
	const uid types.UID = "original-uid"
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	labels := map[string]string{
		common.GeneratePolicyLabel:     "policy",
		common.GenerateTriggerUIDLabel: "trigger",
	}
	res := Resource{
		Name: "settings", Namespace: "app", Labels: labels,
		Data: makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", uid, labels),
	}
	w := &watcher{metadataCache: map[types.UID]Resource{uid: res}}
	var wm *WatchManager
	client := &MockClient{
		deleteFn: func(_ context.Context, _, _, _, _ string, _ bool, _ metav1.DeleteOptions) error {
			return apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, "settings", errors.New("UID mismatch"))
		},
		getFn: func(_ context.Context, _, _, _, _ string) (*unstructured.Unstructured, error) {
			wm.lock.Lock()
			w.metadataCache[uid] = Resource{
				Name: "new-settings", Namespace: "app", Labels: labels,
				Data: makeUnstructured("", "", "v1", "ConfigMap", "new-settings", "app", uid, labels),
			}
			wm.lock.Unlock()
			return makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", "replacement-uid", nil), nil
		},
	}
	wm = &WatchManager{
		log: logging.WithName("test"), client: client,
		policyRefs:      map[string][]schema.GroupVersionResource{"policy": {gvr}},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{gvr: w},
	}
	wm.CleanupStaleDownstreams("policy", &kyverno.ResourceSpec{UID: "trigger"}, nil)
	require.Contains(t, w.metadataCache, uid)
	assert.Equal(t, "new-settings", w.metadataCache[uid].Name)
}

// An old-GVR conflict must release only this policy's reference after UID
// verification, without stopping a watcher used by another policy.
func TestSyncWatchersOldGVRConflictPreservesSharedWatcher(t *testing.T) {
	const oldUID types.UID = "old-uid"
	const otherUID types.UID = "other-uid"
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	labels := map[string]string{
		common.GeneratePolicyLabel:     "policy",
		common.GenerateTriggerUIDLabel: "trigger",
	}
	otherLabels := map[string]string{
		common.GeneratePolicyLabel:     "other-policy",
		common.GenerateTriggerUIDLabel: "other-trigger",
	}
	w := &watcher{metadataCache: map[types.UID]Resource{
		oldUID: {
			Name: "settings", Namespace: "app", Labels: labels,
			Data: makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", oldUID, labels),
		},
		otherUID: {
			Name: "other-settings", Namespace: "app", Labels: otherLabels,
			Data: makeUnstructured("", "", "v1", "ConfigMap", "other-settings", "app", otherUID, otherLabels),
		},
	}}
	client := &MockClient{
		deleteFn: func(_ context.Context, _, _, _, _ string, _ bool, opts metav1.DeleteOptions) error {
			require.NotNil(t, opts.Preconditions)
			require.NotNil(t, opts.Preconditions.UID)
			assert.Equal(t, oldUID, *opts.Preconditions.UID)
			return apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, "settings", errors.New("UID mismatch"))
		},
		getFn: func(_ context.Context, _, _, _, _ string) (*unstructured.Unstructured, error) {
			return makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", "replacement-uid", nil), nil
		},
	}
	wm := &WatchManager{
		client:          client,
		log:             logging.WithName("test"),
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{gvr: w},
		policyRefs:      map[string][]schema.GroupVersionResource{"policy": {gvr}, "other-policy": {gvr}},
		refCount:        map[schema.GroupVersionResource]int{gvr: 2},
	}
	require.NoError(t, wm.SyncWatchers("policy", &kyverno.ResourceSpec{UID: "trigger"}, nil, false))
	assert.NotContains(t, w.metadataCache, oldUID)
	assert.Contains(t, w.metadataCache, otherUID)
	assert.Empty(t, wm.policyRefs["policy"])
	assert.Equal(t, []schema.GroupVersionResource{gvr}, wm.policyRefs["other-policy"])
	assert.Equal(t, 1, wm.refCount[gvr], "shared watcher must retain the other policy's reference")
	assert.Same(t, w, wm.dynamicWatchers[gvr])
}

// Source deletion is also a cleanup entry point. An ambiguous 409 must be
// checked outside the global lock so stale UID entries are not stranded.
func TestHandleDeleteVerifiesDownstreamConflictsOutsideLock(t *testing.T) {
	const sourceUID types.UID = "source-uid"
	const oldUID types.UID = "old-uid"
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	for _, tt := range []struct {
		name       string
		liveUID    types.UID
		wantCached bool
	}{
		{name: "source deletion with replacement", liveUID: "replacement-uid"},
		{name: "webhook conflict with original resource", liveUID: oldUID, wantCached: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			downstream := makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", oldUID, nil)
			cached := Resource{Name: "settings", Namespace: "app", Data: downstream}
			w := &watcher{metadataCache: map[types.UID]Resource{oldUID: cached}}
			var wm *WatchManager
			gets := 0
			client := &MockClient{
				listItems: []unstructured.Unstructured{*downstream},
				deleteFn: func(_ context.Context, _, _, _, _ string, _ bool, opts metav1.DeleteOptions) error {
					require.NotNil(t, opts.Preconditions)
					require.NotNil(t, opts.Preconditions.UID)
					assert.Equal(t, oldUID, *opts.Preconditions.UID)
					return apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, "settings", errors.New("conflict"))
				},
				getFn: func(_ context.Context, _, _, _, _ string) (*unstructured.Unstructured, error) {
					gets++
					if wm.lock.TryLock() {
						wm.lock.Unlock()
					} else {
						t.Error("source-deletion verification must run outside wm.lock")
					}
					return makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", tt.liveUID, nil), nil
				},
			}
			wm = &WatchManager{
				log:             logging.WithName("test"),
				client:          client,
				dynamicWatchers: map[schema.GroupVersionResource]*watcher{gvr: w},
			}
			source := makeUnstructured("", "", "v1", "ConfigMap", "source", "app", sourceUID, nil)
			wm.handleDelete(source, gvr)
			assert.Equal(t, 1, gets)
			_, exists := w.metadataCache[oldUID]
			assert.Equal(t, tt.wantCached, exists)
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
