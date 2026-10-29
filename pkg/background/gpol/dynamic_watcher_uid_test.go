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
	"k8s.io/apimachinery/pkg/api/meta"
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
		{name: "same-named replacement survives", cacheUID: oldUID, dataUID: oldUID, liveUID: replacementUID, wantDeletes: 1, wantGets: 1, wantLiveUID: replacementUID},
		{name: "conflict then NotFound retires entry", cacheUID: oldUID, dataUID: oldUID, deleteErr: conflict, wantDeletes: 1, wantGets: 1},
		{name: "conflict with original UID retains entry", cacheUID: oldUID, dataUID: oldUID, liveUID: oldUID, deleteErr: conflict, wantDeletes: 1, wantGets: 1, wantCached: true, wantLiveUID: oldUID},
		{name: "unverifiable conflict retains entry", cacheUID: oldUID, dataUID: oldUID, liveUID: replacementUID, deleteErr: conflict, getErr: unavailable, wantDeletes: 1, wantGets: 1, wantCached: true, wantLiveUID: replacementUID},
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
			assert.Equal(t, 1, gets)
			assert.Equal(t, newUID, liveUID, "replacement must survive cleanup")
			assert.NotContains(t, w.metadataCache, oldUID)
		})
	}
}

// A nil-Data cache entry must be recovered or retired, without touching a
// same-named replacement or forgetting an identity after a transient failure.
func TestCleanupStaleDownstreamsNilData(t *testing.T) {
	const uid types.UID = "cached-uid"
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	notFound := apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "settings")
	unavailable := apierrors.NewServiceUnavailable("API temporarily unavailable")
	tests := []struct {
		name, resourceName string
		liveUID            types.UID
		getErr             error
		desired, noMapper  bool
		wantGets, wantDels int
		wantCache, wantData bool
	}{
		{name: "stale original deleted with UID", resourceName: "settings", liveUID: uid, wantGets: 1, wantDels: 1},
		{name: "desired object safely recovered", resourceName: "settings", liveUID: uid, desired: true, wantGets: 1, wantCache: true, wantData: true},
		{name: "replacement preserved and stale cache retired", resourceName: "settings", liveUID: "replacement", wantGets: 1},
		{name: "NotFound retires cache", resourceName: "settings", getErr: notFound, wantGets: 1},
		{name: "transient GET failure retains cache", resourceName: "settings", getErr: unavailable, wantGets: 1, wantCache: true},
		{name: "empty live UID keeps cache for retry", resourceName: "settings", wantGets: 1, wantCache: true},
		{name: "missing mapper keeps cache for retry", resourceName: "settings", noMapper: true, wantCache: true},
		{name: "missing name retires invalid cache", resourceName: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := map[string]string{
				common.GeneratePolicyLabel:     "policy",
				common.GenerateTriggerUIDLabel: "trigger",
			}
			entry := Resource{Name: tt.resourceName, Namespace: "app", Labels: labels, Hash: "old-desired-hash"}
			w := &watcher{metadataCache: map[types.UID]Resource{uid: entry}}
			var desired []*unstructured.Unstructured
			if tt.desired {
				desired = []*unstructured.Unstructured{makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", uid, labels)}
			}
			gets, deletes := 0, 0
			client := &MockClient{
				getFn: func(_ context.Context, apiVersion, kind, namespace, name string) (*unstructured.Unstructured, error) {
					gets++
					assert.Equal(t, "v1", apiVersion)
					assert.Equal(t, "ConfigMap", kind)
					assert.Equal(t, "app", namespace)
					assert.Equal(t, "settings", name)
					if tt.getErr != nil {
						return nil, tt.getErr
					}
					return makeUnstructured("", "", "v1", "ConfigMap", "settings", "app", tt.liveUID, labels), nil
				},
				deleteFn: func(_ context.Context, _, _, _, _ string, _ bool, opts metav1.DeleteOptions) error {
					deletes++
					require.NotNil(t, opts.Preconditions, "DELETE must use UID preconditions")
					require.NotNil(t, opts.Preconditions.UID)
					assert.Equal(t, uid, *opts.Preconditions.UID)
					return nil
				},
			}
			wm := &WatchManager{
				client:          client,
				log:             logging.WithName("test"),
				policyRefs:      map[string][]schema.GroupVersionResource{"policy": {gvr}},
				dynamicWatchers: map[schema.GroupVersionResource]*watcher{gvr: w},
			}
			if !tt.noMapper {
				mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})
				mapper.Add(gvk, meta.RESTScopeNamespace)
				wm.restMapper = mapper
			}
			wm.CleanupStaleDownstreams("policy", &kyverno.ResourceSpec{UID: "trigger"}, desired)
			assert.Equal(t, tt.wantGets, gets)
			assert.Equal(t, tt.wantDels, deletes)
			got, ok := w.metadataCache[uid]
			assert.Equal(t, tt.wantCache, ok)
			if tt.wantData {
				require.NotNil(t, got.Data)
				assert.Equal(t, uid, got.Data.GetUID())
				assert.Empty(t, got.Hash, "a live snapshot must not masquerade as desired policy state")
			} else if tt.wantCache {
				assert.Nil(t, got.Data)
			}
		})
	}
}
