package gpol

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	gpolvalidation "github.com/kyverno/kyverno/pkg/cel/policies/gpol"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

// This client models acknowledged writes and queues their Modified events.
// It is not an API server or an implementation of server-side field ownership.
type chainReproClient struct {
	dclient.Interface
	objects map[string]*unstructured.Unstructured
	events  []*unstructured.Unstructured
	writes  []recordedWrite
	nextRV  int
	// listErr, when set, makes ListResource fail the way the real client can:
	// a nil list with an error.
	listErr error
	// listFailCount, when positive, makes the next N ListResource calls fail
	// with a transient error then succeed. Used by list-error-retry.
	listFailCount int
}

// recordedWrite is one acknowledged write: its key and the data.version it carried.
type recordedWrite struct {
	key     string
	version string
}

func (c *chainReproClient) GetResource(_ context.Context, _, _kind, namespace, name string, _ ...string) (*unstructured.Unstructured, error) {
	if obj := c.objects[namespace+"/"+name]; obj != nil {
		return obj.DeepCopy(), nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, name)
}

func (c *chainReproClient) ListResource(_ context.Context, _, _, namespace string, selector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	if c.listFailCount > 0 {
		c.listFailCount--
		return nil, fmt.Errorf("transient list failure")
	}
	if c.listErr != nil {
		return nil, c.listErr
	}
	sel, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}
	list := &unstructured.UnstructuredList{}
	for _, obj := range c.objects {
		if (namespace == "" || namespace == obj.GetNamespace()) && sel.Matches(labels.Set(obj.GetLabels())) {
			list.Items = append(list.Items, *obj.DeepCopy())
		}
	}
	sort.Slice(list.Items, func(i, j int) bool {
		if list.Items[i].GetNamespace() != list.Items[j].GetNamespace() {
			return list.Items[i].GetNamespace() < list.Items[j].GetNamespace()
		}
		return list.Items[i].GetName() < list.Items[j].GetName()
	})
	return list, nil
}

func (c *chainReproClient) write(obj interface{}, update bool) (*unstructured.Unstructured, error) {
	out := obj.(*unstructured.Unstructured).DeepCopy()
	key := out.GetNamespace() + "/" + out.GetName()
	prior := c.objects[key]
	if update && prior == nil {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, out.GetName())
	}
	if update && out.GetResourceVersion() != prior.GetResourceVersion() {
		return nil, apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, out.GetName(), fmt.Errorf("stale resourceVersion"))
	}
	if prior != nil {
		out.SetUID(prior.GetUID())
	} else {
		out.SetUID(types.UID("uid-" + out.GetNamespace()))
	}
	c.nextRV++
	out.SetResourceVersion(fmt.Sprint(c.nextRV))
	c.objects[key] = out.DeepCopy()
	c.events = append(c.events, out.DeepCopy())
	version, _, _ := unstructured.NestedString(out.Object, "data", "version")
	c.writes = append(c.writes, recordedWrite{key: key, version: version})
	return out.DeepCopy(), nil
}

func (c *chainReproClient) CreateResource(_ context.Context, _, _, _ string, obj interface{}, _ bool) (*unstructured.Unstructured, error) {
	return c.write(obj, false)
}

func (c *chainReproClient) UpdateResource(_ context.Context, _, _, _ string, obj interface{}, _ bool, _ ...string) (*unstructured.Unstructured, error) {
	return c.write(obj, true)
}

func (c *chainReproClient) ApplyResource(_ context.Context, _, _, _, _ string, obj interface{}, _ bool, _ string, _ ...string) (*unstructured.Unstructured, error) {
	return c.write(obj, false)
}

func TestChainedCloneSynchronization(t *testing.T) {
	previous := libs.LibraryContext
	t.Cleanup(func() { libs.LibraryContext = previous })
	for _, ssa := range []bool{false, true} {
		for _, scenario := range []string{"single-hop-control", "two-hop-chain", "tamper-control", "deferred-accepted", "deferred-tampered", "deferred-invalidated", "deferred-removed", "no-op-convergence", "invalidated-entry", "invalidated-downstream", "list-error", "list-error-recovery", "list-error-retry", "aliased-clone-data", "downstream-metadata-preserved"} {
			t.Run(fmt.Sprintf("%s/ssa=%t", scenario, ssa), func(t *testing.T) {
				ctx := context.Background()
				gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
				mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})
				mapper.Add(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
				seed := &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "v1", "kind": "ConfigMap",
					"metadata": map[string]interface{}{"name": "settings", "namespace": "seed", "uid": "uid-seed", "resourceVersion": "10"},
					"data":     map[string]interface{}{"version": "v1"},
				}}
				client := &chainReproClient{objects: map[string]*unstructured.Unstructured{"seed/settings": seed}, nextRV: 100}
				library, err := libs.NewContextProvider(client, nil, nil, mapper, false)
				require.NoError(t, err)
				clone := func(policy, target string, source *unstructured.Unstructured) *unstructured.Unstructured {
					library.ClearGeneratedResources()
					library.SetGenerateContext(policy, "", target, "", "v1", "", "Namespace", "trigger-"+target, false, ssa)
					require.NoError(t, library.GenerateResources(target, []map[string]interface{}{source.DeepCopy().Object}))
					require.Len(t, library.GetGeneratedResources(), 1)
					return library.GetGeneratedResources()[0].DeepCopy()
				}
				hub := clone("seed-to-hub", "hub", seed)
				leaf := clone("hub-to-leaf", "leaf", hub)
				require.Equal(t, string(seed.GetUID()), hub.GetLabels()[common.GenerateSourceUIDLabel])
				require.Equal(t, string(hub.GetUID()), leaf.GetLabels()[common.GenerateSourceUIDLabel])
				require.Equal(t, kyverno.ValueKyvernoApp, hub.GetLabels()[kyverno.LabelAppManagedBy])
				cached := func(obj *unstructured.Unstructured) Resource {
					return Resource{Name: obj.GetName(), Namespace: obj.GetNamespace(), Hash: reportutils.CalculateResourceHash(*obj), Labels: obj.GetLabels(), Data: obj.DeepCopy(), ServerSideApply: ssa}
				}
				w := &watcher{metadataCache: map[types.UID]Resource{leaf.GetUID(): cached(leaf)}}
				if scenario != "single-hop-control" {
					w.metadataCache[hub.GetUID()] = cached(hub)
				}
				wm := &WatchManager{log: logr.Discard(), client: client, dynamicWatchers: map[schema.GroupVersionResource]*watcher{gvr: w}}
				client.events, client.writes = nil, nil // initial generation is complete
				version := func(namespace string) string {
					v, found, err := unstructured.NestedString(client.objects[namespace+"/settings"].Object, "data", "version")
					require.NoError(t, err)
					require.True(t, found)
					return v
				}
				drain := func() {
					for delivered := 0; len(client.events) > 0; delivered++ {
						require.Less(t, delivered, 10, "watch handling must converge")
						event := client.events[0]
						client.events = client.events[1:]
						wm.handleUpdate(event, gvr)
					}
				}
				deliverOne := func() {
					require.NotEmpty(t, client.events, "expected a pending event")
					event := client.events[0]
					client.events = client.events[1:]
					wm.handleUpdate(event, gvr)
				}
				if scenario == "deferred-accepted" || scenario == "deferred-tampered" {
					// a hub update lands while the generate window for its policy/trigger
					// is open: the mismatch is skipped and re-checked when the window closes
					closer := wm.BeginGenerate("seed-to-hub", types.UID("trigger-hub"))
					value := "v2"
					if scenario == "deferred-tampered" {
						value = "tampered"
					}
					changed := hub.DeepCopy()
					require.NoError(t, unstructured.SetNestedField(changed.Object, value, "data", "version"))
					_, err = client.UpdateResource(ctx, "v1", "ConfigMap", changed.GetNamespace(), changed, false)
					require.NoError(t, err)
					drain()
					// the generate completes and the cache is refreshed; the refreshed hash
					// decides whether the skipped update is forwarded or reverted
					if scenario == "deferred-accepted" {
						w.metadataCache[hub.GetUID()] = cached(changed)
					}
					closer()
					drain()
					t.Logf("hub=%s leaf=%s watcherWrites=%v", version("hub"), version("leaf"), client.writes)
					if scenario == "deferred-accepted" {
						require.Equal(t, "v2", version("leaf"), "an accepted deferred hub update must propagate to the second policy's clone")
						for _, wr := range client.writes {
							require.NotEqual(t, "tampered", wr.version, "no write payload may carry tampered content")
						}
					} else {
						require.Equal(t, "v1", version("hub"), "a deferred tampered hub edit must be reverted")
						require.Equal(t, "v1", version("leaf"), "tampered content must never reach P2's leaf")
						for _, wr := range client.writes {
							if strings.HasPrefix(wr.key, "leaf/") {
								require.Equal(t, "v1", wr.version, "a write to P2's leaf must carry the trusted v1 value")
							}
						}
					}
					return
				}
				// deferred-invalidated check: after drain() and before closer(), invalidate
				// the hub cache entry (Hash == ""); the closer must not forward the
				// skipped update or cause any writes.
				if scenario == "deferred-invalidated" {
					closer := wm.BeginGenerate("seed-to-hub", types.UID("trigger-hub"))
					changed := hub.DeepCopy()
					require.NoError(t, unstructured.SetNestedField(changed.Object, "v2", "data", "version"))
					_, err = client.UpdateResource(ctx, "v1", "ConfigMap", changed.GetNamespace(), changed, false)
					require.NoError(t, err)
					drain()
					// invalidate the hub entry before closer()
					cachedHub := w.metadataCache[hub.GetUID()]
					cachedHub.Hash = ""
					w.metadataCache[hub.GetUID()] = cachedHub
					client.writes = nil // count only closer writes
					closer()
					drain()
					require.Empty(t, client.writes, "closer must not write when hub entry is invalidated")
					require.Equal(t, "v1", version("leaf"), "leaf must remain at v1")
					t.Logf("hub=%s leaf=%s watcherWrites=%v", version("hub"), version("leaf"), client.writes)
					return
				}
				// deferred-removed check: after drain() and before closer(), delete the
				// hub cache entry entirely; the closer must not forward the skipped
				// update or cause any writes.
				if scenario == "deferred-removed" {
					closer := wm.BeginGenerate("seed-to-hub", types.UID("trigger-hub"))
					changed := hub.DeepCopy()
					require.NoError(t, unstructured.SetNestedField(changed.Object, "v2", "data", "version"))
					_, err = client.UpdateResource(ctx, "v1", "ConfigMap", changed.GetNamespace(), changed, false)
					require.NoError(t, err)
					drain()
					// remove the hub entry before closer()
					delete(w.metadataCache, hub.GetUID())
					client.writes = nil // count only closer writes
					closer()
					drain()
					require.Empty(t, client.writes, "closer must not write when hub entry is removed")
					require.Equal(t, "v1", version("leaf"), "leaf must remain at v1")
					t.Logf("hub=%s leaf=%s watcherWrites=%v", version("hub"), version("leaf"), client.writes)
					return
				}
				// no-op/convergence check: after the chain converges, re-delivering the
				// hub's current accepted state must cause zero additional watcher writes
				if scenario == "no-op-convergence" {
					changed := seed.DeepCopy()
					require.NoError(t, unstructured.SetNestedField(changed.Object, "v2", "data", "version"))
					_, err = client.UpdateResource(ctx, "v1", "ConfigMap", changed.GetNamespace(), changed, false)
					require.NoError(t, err)
					drain()
					require.Equal(t, "v2", version("hub"), "the first hop must succeed")
					require.Equal(t, "v2", version("leaf"), "an accepted hub update must propagate to the second policy's clone")
					client.writes = nil // convergence complete; reset the write counter
					// re-deliver the hub's current accepted state as a watch event
					wm.handleUpdate(client.objects["hub/settings"].DeepCopy(), gvr)
					require.Empty(t, client.writes, "re-delivering converged hub state must cause no watcher writes")
					t.Logf("hub=%s leaf=%s watcherWrites=%v", version("hub"), version("leaf"), client.writes)
					require.Equal(t, "v2", version("hub"))
					require.Equal(t, "v2", version("leaf"))
					return
				}
				// invalidated-entry check: an invalidated hub cache entry (Hash == "",
				// as InvalidateDownstreams does) must not distribute an updated hub to
				// the leaf that clones from it
				if scenario == "invalidated-entry" {
					cachedHub := w.metadataCache[hub.GetUID()]
					cachedHub.Hash = "" // invalidate, as InvalidateDownstreams does
					w.metadataCache[hub.GetUID()] = cachedHub
					changed := hub.DeepCopy()
					require.NoError(t, unstructured.SetNestedField(changed.Object, "v2", "data", "version"))
					_, err = client.UpdateResource(ctx, "v1", "ConfigMap", changed.GetNamespace(), changed, false)
					require.NoError(t, err)
					client.writes = nil // count only watcher writes
					drain()
					require.Empty(t, client.writes, "an invalidated entry must not trigger any watcher writes")
					require.Equal(t, "v2", version("hub"), "the hub update itself is applied")
					require.Equal(t, "v1", version("leaf"), "an invalidated hub entry must not distribute to P2's leaf")
					t.Logf("hub=%s leaf=%s watcherWrites=%v", version("hub"), version("leaf"), client.writes)
					return
				}
				// invalidated-downstream check: an invalidated leaf cache entry (Hash == "",
				// as InvalidateDownstreams does) must not be resurrected when the hub updates.
				// The seed update should propagate: seed -> hub (accepted) -> leaf (skipped because invalidated)
				// while an active sibling leaf in namespace "sibling" still reaches v2.
				// The fake's ListResource returns items sorted by namespace/name so the
				// invalidated leaf is genuinely first, distinguishing continue from return.
				if scenario == "invalidated-downstream" {
					sibling := clone("hub-to-leaf", "sibling", hub)
					require.Equal(t, string(hub.GetUID()), sibling.GetLabels()[common.GenerateSourceUIDLabel])
					w.metadataCache[sibling.GetUID()] = cached(sibling)
					cachedLeaf := w.metadataCache[leaf.GetUID()]
					cachedLeaf.Hash = "" // invalidate, as InvalidateDownstreams does
					w.metadataCache[leaf.GetUID()] = cachedLeaf
					changed := seed.DeepCopy()
					require.NoError(t, unstructured.SetNestedField(changed.Object, "v2", "data", "version"))
					_, err = client.UpdateResource(ctx, "v1", "ConfigMap", changed.GetNamespace(), changed, false)
					require.NoError(t, err)
					client.writes = nil // count only watcher writes
					drain()
					// Hub write is expected (seed -> hub) and sibling must be updated, but no leaf write should occur
					for _, wr := range client.writes {
						require.False(t, strings.HasPrefix(wr.key, "leaf/"), "an invalidated leaf entry must not trigger any watcher writes")
					}
					require.Equal(t, "v2", version("hub"), "the hub update is applied")
					require.Equal(t, "v2", version("sibling"), "the active sibling leaf must still reach v2")
					require.Equal(t, "v1", version("leaf"), "an invalidated leaf entry must not be resurrected by hub update")
					t.Logf("hub=%s leaf=%s sibling=%s watcherWrites=%v", version("hub"), version("leaf"), version("sibling"), client.writes)
					return
				}
				// list-error check: a transient downstream-list failure must not panic
				// the watcher or write anything; the next watch event retries and the
				// chain still converges
				if scenario == "list-error" {
					client.listErr = fmt.Errorf("transient list failure")
					changed := seed.DeepCopy()
					require.NoError(t, unstructured.SetNestedField(changed.Object, "v2", "data", "version"))
					_, err = client.UpdateResource(ctx, "v1", "ConfigMap", changed.GetNamespace(), changed, false)
					require.NoError(t, err)
					client.writes = nil // count only watcher writes
					drain()
					require.Empty(t, client.writes, "a failed downstream list must not cause watcher writes")
					require.Equal(t, "v1", version("hub"), "a failed downstream list must leave the hub unchanged")
					require.Equal(t, "v1", version("leaf"), "a failed downstream list must leave the leaf unchanged")
					// the list recovers; re-delivering the source's current state retries
					// the propagation and the chain converges
					client.listErr = nil
					wm.handleUpdate(client.objects["seed/settings"].DeepCopy(), gvr)
					drain()
					require.Equal(t, "v2", version("hub"), "the first hop must succeed once the list recovers")
					require.Equal(t, "v2", version("leaf"), "an accepted hub update must propagate to the second policy's clone")
					t.Logf("hub=%s leaf=%s watcherWrites=%v", version("hub"), version("leaf"), client.writes)
					return
				}
				// list-error-recovery: seed propagation updates the hub to v2, then
				// listing the hub's downstreams fails. Clearing the error and replaying
				// the hub event covers recovery through the accepted cached-hub path.
				if scenario == "list-error-recovery" {
					// Phase 1: deliver only the seed event so the hub reaches v2 while
					// the hub event is still pending and the leaf has not been updated.
					changed := seed.DeepCopy()
					require.NoError(t, unstructured.SetNestedField(changed.Object, "v2", "data", "version"))
					_, err = client.UpdateResource(ctx, "v1", "ConfigMap", changed.GetNamespace(), changed, false)
					require.NoError(t, err)
					client.writes = nil // count only watcher writes
					deliverOne()
					require.Equal(t, "v2", version("hub"), "phase 1: seed propagation must update the hub to v2")
					require.Equal(t, "v1", version("leaf"), "phase 1: leaf must remain at v1 before the hub event is delivered")
					require.Len(t, client.events, 1, "phase 1: exactly one hub event must be pending")
					// Phase 2: fail listing the hub's downstreams while draining the pending hub event.
					client.listErr = fmt.Errorf("transient list failure")
					client.writes = nil
					drain()
					require.Empty(t, client.writes, "a failed downstream list must not cause watcher writes")
					require.Equal(t, "v2", version("hub"), "the hub must retain the accepted state")
					require.Equal(t, "v1", version("leaf"), "the leaf must remain at v1 after the failed list")
					// Phase 3: clear the error and replay the hub event through the accepted cached-hub path.
					client.listErr = nil
					client.writes = nil
					wm.handleUpdate(client.objects["hub/settings"].DeepCopy(), gvr)
					drain()
					require.Equal(t, "v2", version("leaf"), "replaying the hub event must propagate the accepted hub state to the leaf")
					t.Logf("hub=%s leaf=%s watcherWrites=%v", version("hub"), version("leaf"), client.writes)
					return
				}
				// list-error-retry: transient downstream-list failures are retried inline.
				// The first 2 list attempts fail, the 3rd succeeds; with no hand-delivered
				// replay event the bounded retry must converge the chain to v2.
				if scenario == "list-error-retry" {
					client.listFailCount = 2
					changed := seed.DeepCopy()
					require.NoError(t, unstructured.SetNestedField(changed.Object, "v2", "data", "version"))
					_, err = client.UpdateResource(ctx, "v1", "ConfigMap", changed.GetNamespace(), changed, false)
					require.NoError(t, err)
					client.writes = nil // count only watcher writes
					drain()
					require.Equal(t, "v2", version("hub"), "the first hop must succeed after transient list retries")
					require.Equal(t, "v2", version("leaf"), "an accepted hub update must propagate to the second policy's clone after transient list retries")
					require.Zero(t, client.listFailCount, "all injected list failures must be consumed by retries")
					t.Logf("hub=%s leaf=%s watcherWrites=%v", version("hub"), version("leaf"), client.writes)
					return
				}
				if scenario == "aliased-clone-data" {
					// two leaves clone from the same hub (same object name, different namespaces)
					sibling := clone("hub-to-leaf", "sibling", hub)
					require.Equal(t, string(hub.GetUID()), sibling.GetLabels()[common.GenerateSourceUIDLabel])
					w.metadataCache[sibling.GetUID()] = cached(sibling)
					changed := seed.DeepCopy()
					require.NoError(t, unstructured.SetNestedField(changed.Object, "v2", "data", "version"))
					_, err = client.UpdateResource(ctx, "v1", "ConfigMap", changed.GetNamespace(), changed, false)
					require.NoError(t, err)
					drain()
					// each cached Data must retain its own namespace (not aliased by the loop)
					require.Equal(t, "leaf", w.metadataCache[leaf.GetUID()].Data.GetNamespace(), "cached Data for first leaf must keep its namespace")
					require.Equal(t, "sibling", w.metadataCache[sibling.GetUID()].Data.GetNamespace(), "cached Data for second leaf must keep its namespace")
					// tamper the first-listed leaf; revert must target the right object
					target := leaf
					if listed, lerr := client.ListResource(ctx, "v1", "ConfigMap", "", &metav1.LabelSelector{MatchLabels: map[string]string{common.GenerateSourceUIDLabel: string(hub.GetUID())}}); lerr == nil && len(listed.Items) > 0 && listed.Items[0].GetNamespace() == "sibling" {
						target = sibling
					}
					tampered := client.objects[target.GetNamespace()+"/settings"].DeepCopy()
					require.NoError(t, unstructured.SetNestedField(tampered.Object, "tampered", "data", "version"))
					_, err = client.UpdateResource(ctx, "v1", "ConfigMap", tampered.GetNamespace(), tampered, false)
					require.NoError(t, err)
					drain()
					require.Equal(t, "v2", version("leaf"), "leaf must be restored to v2 after tamper")
					require.Equal(t, "v2", version("sibling"), "sibling must remain at v2 after tamper")
					t.Logf("hub=%s leaf=%s sibling=%s watcherWrites=%v", version("hub"), version("leaf"), version("sibling"), client.writes)
					return
				}
				changed := seed.DeepCopy()
				value := "v2"
				if scenario == "single-hop-control" {
					changed = hub.DeepCopy()
				} else if scenario == "tamper-control" {
					changed, value = hub.DeepCopy(), "tampered"
				}
				// downstream-metadata-preserved: an accepted hub update must not
				// overwrite the leaf's own finalizers and owner references with the
				// source's. The leaf keeps exactly its own metadata and never
				// carries the hub's. Metadata is set on the stored objects (the ones
				// the production code reads from ListResource/watch events), not on
				// the detached DeepCopy() variables `hub`/`leaf`.
				if scenario == "downstream-metadata-preserved" {
					hubObj := client.objects["hub/settings"]
					leafObj := client.objects["leaf/settings"]
					hubObj.SetFinalizers([]string{"hub.example.com/keep"})
					hubObj.SetOwnerReferences([]metav1.OwnerReference{{Name: "hub-owner", APIVersion: "v1", Kind: "ConfigMap", UID: "uid-hub-owner"}})
					leafObj.SetFinalizers([]string{"leaf.example.com/keep"})
					leafObj.SetOwnerReferences([]metav1.OwnerReference{{Name: "leaf-owner", APIVersion: "v1", Kind: "ConfigMap", UID: "uid-leaf-owner"}})
					changed := seed.DeepCopy()
					require.NoError(t, unstructured.SetNestedField(changed.Object, "v2", "data", "version"))
					_, err = client.UpdateResource(ctx, "v1", "ConfigMap", changed.GetNamespace(), changed, false)
					require.NoError(t, err)
					client.writes = nil // count only watcher writes
					drain()
					require.Equal(t, "v2", version("hub"), "the first hop must succeed")
					require.Equal(t, "v2", version("leaf"), "an accepted hub update must propagate to the second policy's clone")
					if !ssa {
						// The fake's ApplyResource replaces the whole stored object with the
						// apply config (applyConfiguration drops metadata outside name,
						// namespace, labels and annotations), so it does not model SSA field
						// ownership: finalizers/ownerReferences would not survive a real SSA
						// write here. Assert metadata preservation only in the non-SSA path,
						// where the production code copies the live downstream's finalizers
						// and owner references onto the update.
						require.Equal(t, []string{"leaf.example.com/keep"}, client.objects["leaf/settings"].GetFinalizers(), "the leaf must keep its own finalizers")
						require.Equal(t, []metav1.OwnerReference{{Name: "leaf-owner", APIVersion: "v1", Kind: "ConfigMap", UID: "uid-leaf-owner"}}, client.objects["leaf/settings"].GetOwnerReferences(), "the leaf must keep its own owner references")
						for _, f := range client.objects["leaf/settings"].GetFinalizers() {
							require.NotEqual(t, "hub.example.com/keep", f, "the leaf must not carry the hub's finalizer")
						}
						for _, o := range client.objects["leaf/settings"].GetOwnerReferences() {
							require.NotEqual(t, "hub-owner", o.Name, "the leaf must not carry the hub's owner reference")
						}
					}
					t.Logf("hub=%s leaf=%s watcherWrites=%v", version("hub"), version("leaf"), client.writes)
					return
				}
				require.NoError(t, unstructured.SetNestedField(changed.Object, value, "data", "version"))
				_, err = client.UpdateResource(ctx, "v1", "ConfigMap", changed.GetNamespace(), changed, false)
				require.NoError(t, err)
				client.writes = nil // count only watcher writes
				drain()
				t.Logf("hub=%s leaf=%s watcherWrites=%v", version("hub"), version("leaf"), client.writes)
				if scenario == "tamper-control" {
					require.Equal(t, "v1", version("hub"), "P1 must continue reverting unauthorized hub edits")
					require.Equal(t, "v1", version("leaf"), "tampered content must not reach P2's leaf")
					for _, wr := range client.writes {
						if strings.HasPrefix(wr.key, "leaf/") {
							require.Equal(t, "v1", wr.version, "a write to P2's leaf must carry the trusted v1 value")
						}
					}
					return
				}
				require.Equal(t, "v2", version("hub"), "the first hop must succeed")
				require.Equal(t, "v2", version("leaf"), "an accepted hub update must propagate to the second policy's clone")
			})
		}
	}
}

func TestChainedClonePolicyManifests(t *testing.T) {
	for _, stage := range []struct{ name, source, target string }{
		{"seed-to-hub", "seed", "hub"},
		{"hub-to-leaf", "hub", "leaf"},
	} {
		t.Run(stage.name, func(t *testing.T) {
			manifest := fmt.Sprintf(`apiVersion: policies.kyverno.io/v1
kind: GeneratingPolicy
metadata:
  name: %s
spec:
  evaluation:
    synchronize:
      enabled: true
  matchConstraints:
    resourceRules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["namespaces"]
    objectSelector:
      matchLabels:
        clone-stage: %s
  variables:
    - name: source
      expression: resource.Get("v1", "configmaps", "%s", "settings")
  generate:
    - expression: generator.Apply(object.metadata.name, [variables.source])
`, stage.name, stage.target, stage.source)
			var policy policiesv1beta1.GeneratingPolicy
			require.NoError(t, yaml.Unmarshal([]byte(manifest), &policy))
			_, err := gpolvalidation.Validate(&policy)
			require.NoError(t, err, "the reproduction manifests must pass Kyverno CEL policy validation")
		})
	}
}
