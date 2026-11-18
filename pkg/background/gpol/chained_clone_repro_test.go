package gpol

import (
	"context"
	"fmt"
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
		for _, scenario := range []string{"single-hop-control", "two-hop-chain", "tamper-control", "deferred-accepted", "deferred-tampered"} {
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
				changed := seed.DeepCopy()
				value := "v2"
				if scenario == "single-hop-control" {
					changed = hub.DeepCopy()
				} else if scenario == "tamper-control" {
					changed, value = hub.DeepCopy(), "tampered"
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
