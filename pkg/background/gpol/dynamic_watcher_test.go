package gpol

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/kyverno/kyverno/api/kyverno"

	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/logging"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	eventsv1 "k8s.io/client-go/kubernetes/typed/events/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
)

type mockRESTMapper struct {
	fn func(gk schema.GroupKind, version string) (*meta.RESTMapping, error)
}

func (m *mockRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	if m.fn != nil {
		return m.fn(gk, versions[0])
	}
	return nil, errors.New("rest-mapping-error")
}
func (m *mockRESTMapper) KindFor(res schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}
func (m *mockRESTMapper) KindsFor(res schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return nil, nil
}
func (m *mockRESTMapper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{}, nil
}
func (m *mockRESTMapper) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return []schema.GroupVersionResource{}, nil
}
func (m *mockRESTMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	return nil, nil
}
func (m *mockRESTMapper) ResourceSingularizer(resource string) (string, error) {
	return "", nil
}

type MockClient struct {
	deleted  []string
	created  []string
	updated  []string
	err      error
	deleteFn func(ctx context.Context, apiVersion, kind, namespace, name string, dryRun bool, options metav1.DeleteOptions) error
	createFn func(ctx context.Context, apiVersion, kind, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error)
	getFn    func(ctx context.Context, apiVersion, kind, namespace, name string) (*unstructured.Unstructured, error)
	updateFn func(obj interface{})
	applied  []string
	applyFn  func(fieldManager string, obj interface{})
	// applyReturns, when set, is what ApplyResource returns.
	applyReturns *unstructured.Unstructured
	// listItems, when set, is returned by ListResource instead of the default single empty item.
	listItems []unstructured.Unstructured
}

func (m *MockClient) GetKubeClient() kubernetes.Interface {
	return &kubernetes.Clientset{}
}
func (m *MockClient) GetEventsInterface() eventsv1.EventsV1Interface {
	return nil
}
func (m *MockClient) GetDynamicInterface() dynamic.Interface {
	return dynamic.New(&rest.RESTClient{})
}
func (m *MockClient) Discovery() dclient.IDiscovery {
	return dclient.NewEmptyFakeClient().Discovery()
}
func (m *MockClient) SetDiscovery(discoverClient dclient.IDiscovery) {
}
func (m *MockClient) RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error) {
	return nil, nil
}
func (m *MockClient) GetResource(ctx context.Context, apiVersion string, kind string, namespace, name string, subresources ...string) (*unstructured.Unstructured, error) {
	if m.getFn != nil {
		return m.getFn(ctx, apiVersion, kind, namespace, name)
	}
	return nil, nil
}
func (m *MockClient) PatchResource(ctx context.Context, apiVersion string, kind string, namespace, name string, path []byte) (*unstructured.Unstructured, error) {
	return nil, nil
}
func (m *MockClient) ListResource(ctx context.Context, apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.listItems != nil {
		return &unstructured.UnstructuredList{Items: m.listItems}, nil
	}
	item := makeUnstructured("", "", "", "", "", "", "", nil)
	return &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{
			*item,
		},
	}, nil
}
func (m *MockClient) DeleteResource(ctx context.Context, apiVersion string, kind string, namespace, name string, dryRun bool, options metav1.DeleteOptions) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, apiVersion, kind, namespace, name, dryRun, options)
	}
	m.deleted = append(m.deleted, fmt.Sprintf("%s/%s/%s", kind, namespace, name))
	return m.err
}
func (m *MockClient) CreateResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	m.created = append(m.created, fmt.Sprintf("%s/%s", kind, namespace))
	if m.createFn != nil {
		return m.createFn(ctx, apiVersion, kind, namespace, obj, dryRun)
	}
	return nil, nil
}
func (m *MockClient) UpdateResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool, subresource ...string) (*unstructured.Unstructured, error) {
	if m.updateFn != nil {
		m.updateFn(obj)
	}
	if m.err != nil {
		return nil, m.err
	}
	m.updated = append(m.updated, fmt.Sprintf("%s/%s", kind, namespace))
	return makeUnstructured("", "", "", "", "", "", "", nil), nil
}
func (m *MockClient) UpdateStatusResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	return nil, nil
}
func (m *MockClient) ApplyResource(ctx context.Context, apiVersion string, kind string, namespace, name string, obj interface{}, dryRun bool, fieldManager string, subresources ...string) (*unstructured.Unstructured, error) {
	m.applied = append(m.applied, fmt.Sprintf("%s/%s/%s", kind, namespace, name))
	if m.applyFn != nil {
		m.applyFn(fieldManager, obj)
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.applyReturns, nil
}
func (m *MockClient) ApplyStatusResource(ctx context.Context, apiVersion string, kind string, namespace, name string, obj interface{}, dryRun bool, fieldManager string) (*unstructured.Unstructured, error) {
	return nil, nil
}

var (
	gvr  = schema.GroupVersionResource{Group: "g", Version: "v1", Resource: "res"}
	gvr1 = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "pods"}
)

func makeUnstructured(res, group, version, kind, name, ns string, uid types.UID, labels map[string]string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: version, Kind: kind})
	u.SetName(name)
	u.SetNamespace(ns)
	if res != "" {
		u.SetResourceVersion(res)
	}
	u.SetUID(uid)
	u.SetLabels(labels)
	return u
}

func TestNewWatchManager(t *testing.T) {
	client := dclient.NewEmptyFakeClient()
	log := logging.WithName("test-logging")
	wm := NewWatchManager(log, client)
	assert.NotNil(t, &wm)
}

func TestSyncWatchers(t *testing.T) {
	tests := []struct {
		name               string
		polName            string
		setupWM            func() *WatchManager
		generatedResources []*unstructured.Unstructured
		wantErr            bool
		assert             func(*testing.T, *WatchManager)
	}{
		{
			name:    "RESTMapping error",
			polName: "p1",
			setupWM: func() *WatchManager {
				return &WatchManager{
					log:    logging.WithName("test"),
					client: &MockClient{},
					restMapper: &mockRESTMapper{fn: func(gk schema.GroupKind, version string) (*meta.RESTMapping, error) {
						return nil, errors.New("map err")
					}},
				}
			},
			generatedResources: []*unstructured.Unstructured{makeUnstructured("", "g", "v1", "Kind", "n", "ns", "uid1", nil)},
			wantErr:            true,
		},
		{
			name: "Watcher already exist path",
			setupWM: func() *WatchManager {
				existing := &watcher{
					watcher:       watch.MockWatcher{StopFunc: func() {}},
					metadataCache: map[types.UID]Resource{},
				}
				return &WatchManager{
					log:    logging.WithName("test"),
					client: &MockClient{},
					restMapper: &mockRESTMapper{fn: func(_ schema.GroupKind, _ string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{Resource: gvr}, nil
					}},
					dynamicWatchers: map[schema.GroupVersionResource]*watcher{
						gvr: existing,
					},
					policyRefs: make(map[string][]schema.GroupVersionResource),
					refCount:   make(map[schema.GroupVersionResource]int),
				}
			},
			generatedResources: []*unstructured.Unstructured{makeUnstructured("", "g", "v1", "Kind", "n", "ns", "uid1", nil)},
			wantErr:            false,
		},
		{
			name:    "refreshes invalidated cache entry",
			polName: "p1",
			setupWM: func() *WatchManager {
				resource := makeUnstructured("1", "g", "v1", "Kind", "n", "ns", "uid1", nil)
				return &WatchManager{
					log:    logging.WithName("test"),
					client: &MockClient{},
					restMapper: &mockRESTMapper{fn: func(_ schema.GroupKind, _ string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{Resource: gvr}, nil
					}},
					dynamicWatchers: map[schema.GroupVersionResource]*watcher{
						gvr: {
							watcher: watch.MockWatcher{StopFunc: func() {}},
							metadataCache: map[types.UID]Resource{
								resource.GetUID(): {
									Name:      resource.GetName(),
									Namespace: resource.GetNamespace(),
									Hash:      "",
									Data:      resource,
								},
							},
						},
					},
					policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr}},
					refCount:   map[schema.GroupVersionResource]int{gvr: 1},
				}
			},
			generatedResources: []*unstructured.Unstructured{makeUnstructured("2", "g", "v1", "Kind", "n", "ns", "uid1", nil)},
			wantErr:            false,
			assert: func(t *testing.T, wm *WatchManager) {
				cached := wm.dynamicWatchers[gvr].metadataCache["uid1"]
				assert.NotEmpty(t, cached.Hash)
			},
		},
		{
			name: "startWatcher error",
			setupWM: func() *WatchManager {
				wm := &WatchManager{
					log:    logging.WithName("test"),
					client: &MockClient{},
					restMapper: &mockRESTMapper{fn: func(_ schema.GroupKind, _ string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{Resource: gvr}, nil
					}},
					dynamicWatchers: make(map[schema.GroupVersionResource]*watcher),
					policyRefs:      make(map[string][]schema.GroupVersionResource),
					refCount:        make(map[schema.GroupVersionResource]int),
				}
				return wm
			},
			generatedResources: []*unstructured.Unstructured{makeUnstructured("", "g", "v1", "Kind", "n", "ns", "uid1", nil)},
			wantErr:            true,
		},
		{
			name:    "startWatcher success",
			polName: "pol1",
			setupWM: func() *WatchManager {
				return &WatchManager{
					log:    logging.WithName("test"),
					client: dclient.NewEmptyFakeClient(),
					restMapper: &mockRESTMapper{fn: func(gk schema.GroupKind, version string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{Resource: gvr1}, nil
					}},
					dynamicWatchers: make(map[schema.GroupVersionResource]*watcher),
					policyRefs: map[string][]schema.GroupVersionResource{
						"pol1": {gvr1},
					},
					refCount: make(map[schema.GroupVersionResource]int),
				}
			},
			generatedResources: []*unstructured.Unstructured{makeUnstructured("1", "g", "v1", "Kind", "n", "ns", "uid1", nil)},
			wantErr:            false,
		},
		{
			name:    "remove old watcher and delete resources",
			polName: "pol1",
			setupWM: func() *WatchManager {
				existing := &watcher{
					watcher: watch.MockWatcher{
						StopFunc: func() {
						},
						ResultChanFunc: func() <-chan watch.Event {
							return nil
						},
					},
					metadataCache: map[types.UID]Resource{
						"uid": {
							Name:      "res-test",
							Namespace: "isolated-ns",
							Hash:      "something",
							Labels:    map[string]string{common.GeneratePolicyLabel: "pol1"},
							Data:      makeUnstructured("1", "", "v1", "Pod", "res-test", "isolated-ns", "uid1", map[string]string{common.GeneratePolicyLabel: "pol1"}),
						},
					}}
				return &WatchManager{
					log: logging.WithName("test"),
					client: &MockClient{
						deleteFn: func(ctx context.Context, apiVersion, kind, namespace, name string, dryRun bool, options metav1.DeleteOptions) error {
							// Mock delete operation - no logging needed for test mock
							return nil
						},
					},
					restMapper: &mockRESTMapper{fn: func(gk schema.GroupKind, version string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{Resource: gvr1}, nil
					}},
					dynamicWatchers: map[schema.GroupVersionResource]*watcher{
						gvr:  existing,
						gvr1: existing,
					},
					policyRefs: map[string][]schema.GroupVersionResource{
						"pol1": {gvr},
					},
					refCount: map[schema.GroupVersionResource]int{
						gvr1: 1,
					},
				}
			},
			generatedResources: []*unstructured.Unstructured{makeUnstructured("1", "g", "v1", "Kind", "n", "ns", "uid1", nil)},
			wantErr:            false,
		},
		{
			name:    "error while removing old watcher and delete resources",
			polName: "pol1",
			setupWM: func() *WatchManager {
				existing := &watcher{
					watcher: watch.MockWatcher{
						StopFunc: func() {
						},
						ResultChanFunc: func() <-chan watch.Event {
							return nil
						},
					},
					metadataCache: map[types.UID]Resource{
						"uid": {
							Name:      "res-test",
							Namespace: "isolated-ns",
							Hash:      "something",
							Labels:    map[string]string{common.GeneratePolicyLabel: "pol1"},
							Data:      makeUnstructured("1", "", "v1", "Pod", "res-test", "isolated-ns", "uid1", map[string]string{common.GeneratePolicyLabel: "pol1"}),
						},
					}}
				return &WatchManager{
					log: logging.WithName("test"),
					client: &MockClient{
						err: errors.New("error while deleting old resources"),
					},
					restMapper: &mockRESTMapper{fn: func(gk schema.GroupKind, version string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{Resource: gvr1}, nil
					}},
					dynamicWatchers: map[schema.GroupVersionResource]*watcher{
						gvr:  existing,
						gvr1: existing,
					},
					policyRefs: map[string][]schema.GroupVersionResource{
						"pol1": {gvr},
					},
					refCount: map[schema.GroupVersionResource]int{
						gvr1: 1,
					},
				}
			},
			generatedResources: []*unstructured.Unstructured{makeUnstructured("1", "g", "v1", "Kind", "n", "ns", "uid1", nil)},
			wantErr:            false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wm := tc.setupWM()
			err := wm.SyncWatchers(tc.polName, nil, tc.generatedResources, false)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tc.assert != nil {
				tc.assert(t, wm)
			}
		})
	}
}

func TestSyncWatchers_StaleRefCountRemovedWhenWatcherMissing(t *testing.T) {
	oldGVR := schema.GroupVersionResource{Group: "old", Version: "v1", Resource: "res"}
	wm := &WatchManager{
		log:             logging.WithName("test"),
		client:          &MockClient{},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
		policyRefs:      map[string][]schema.GroupVersionResource{"pol1": {oldGVR}},
		refCount:        map[schema.GroupVersionResource]int{oldGVR: 1},
	}

	err := wm.SyncWatchers("pol1", nil, nil, false)
	require.NoError(t, err)
	_, exists := wm.refCount[oldGVR]
	assert.False(t, exists)
}

// TestSyncWatchers_GVRDropIsTriggerScoped is a regression test for #16759:
// when a trigger's evaluation no longer generates resources of a GVR, only
// the downstream resources generated for THAT trigger may be deleted.
// Resources generated by the same policy for other triggers must survive,
// along with their watcher, ref count and policy reference.
func TestSyncWatchers_GVRDropIsTriggerScoped(t *testing.T) {
	cmGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	secretGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	triggerA := &kyverno.ResourceSpec{UID: "uid-trigger-a"}

	cmA := makeUnstructured("1", "", "v1", "ConfigMap", "cm-a", "ns-a", "uid-cm-a", map[string]string{
		common.GeneratePolicyLabel:     "p1",
		common.GenerateTriggerUIDLabel: "uid-trigger-a",
	})
	cmB := makeUnstructured("1", "", "v1", "ConfigMap", "cm-b", "ns-b", "uid-cm-b", map[string]string{
		common.GeneratePolicyLabel:     "p1",
		common.GenerateTriggerUIDLabel: "uid-trigger-b",
	})
	client := &MockClient{}
	wm := &WatchManager{
		log:    logging.WithName("test"),
		client: client,
		restMapper: &mockRESTMapper{fn: func(gk schema.GroupKind, version string) (*meta.RESTMapping, error) {
			if gk.Kind == "Secret" {
				return &meta.RESTMapping{Resource: secretGVR}, nil
			}
			return &meta.RESTMapping{Resource: cmGVR}, nil
		}},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			cmGVR: {
				watcher: watch.NewFake(),
				metadataCache: map[types.UID]Resource{
					cmA.GetUID(): {Name: "cm-a", Namespace: "ns-a", Labels: cmA.GetLabels(), Data: cmA},
					cmB.GetUID(): {Name: "cm-b", Namespace: "ns-b", Labels: cmB.GetLabels(), Data: cmB},
				},
			},
			// pre-seeded so SyncWatchers reuses it instead of starting a real watcher
			secretGVR: {
				watcher:       watch.NewFake(),
				metadataCache: map[types.UID]Resource{},
			},
		},
		policyRefs: map[string][]schema.GroupVersionResource{"p1": {cmGVR}},
		refCount:   map[schema.GroupVersionResource]int{cmGVR: 1, secretGVR: 1},
	}

	// trigger A's evaluation now generates a Secret instead of a ConfigMap
	secretA := makeUnstructured("1", "", "v1", "Secret", "sec-a", "ns-a", "uid-sec-a", map[string]string{
		common.GeneratePolicyLabel:     "p1",
		common.GenerateTriggerUIDLabel: "uid-trigger-a",
	})
	err := wm.SyncWatchers("p1", triggerA, []*unstructured.Unstructured{secretA}, false)
	require.NoError(t, err)

	// only trigger A's stale ConfigMap is deleted
	assert.Equal(t, []string{"ConfigMap/ns-a/cm-a"}, client.deleted)
	// trigger B's ConfigMap survives, together with the watcher and its bookkeeping
	cmWatcher, exists := wm.dynamicWatchers[cmGVR]
	require.True(t, exists, "configmaps watcher must be kept while trigger B still owns resources")
	_, cmBCached := cmWatcher.metadataCache[cmB.GetUID()]
	assert.True(t, cmBCached, "trigger B's ConfigMap must remain cached")
	assert.Equal(t, 1, wm.refCount[cmGVR])
	assert.ElementsMatch(t, []schema.GroupVersionResource{cmGVR, secretGVR}, wm.policyRefs["p1"])

	// once trigger B's resources are gone too, dropping the GVR stops the watcher
	wm.DeleteDownstreams("p1", &kyverno.ResourceSpec{UID: "uid-trigger-b"})
	err = wm.SyncWatchers("p1", triggerA, []*unstructured.Unstructured{secretA}, false)
	require.NoError(t, err)
	_, exists = wm.dynamicWatchers[cmGVR]
	assert.False(t, exists, "configmaps watcher must be stopped once no resources of the policy remain")
	_, exists = wm.refCount[cmGVR]
	assert.False(t, exists)
	assert.ElementsMatch(t, []schema.GroupVersionResource{secretGVR}, wm.policyRefs["p1"])
}

func TestWatchManager_GetDownstreams(t *testing.T) {
	type mockWatcher struct {
		metadataCache map[types.UID]Resource
	}
	makeCached := func(obj *unstructured.Unstructured) Resource {
		return Resource{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
			Labels:    obj.GetLabels(),
			Hash:      "kept",
			Data:      obj,
		}
	}

	gvr1 := schema.GroupVersionResource{Group: "g", Version: "v1", Resource: "res1"}
	gvr2 := schema.GroupVersionResource{Group: "g", Version: "v1", Resource: "res2"}

	tests := []struct {
		name       string
		policyName string
		policyRefs map[string][]schema.GroupVersionResource
		dynamicW   map[schema.GroupVersionResource]*mockWatcher
		wantKinds  []string
	}{
		{
			name:       "no watchers for policy",
			policyName: "p1",
			policyRefs: map[string][]schema.GroupVersionResource{},
			dynamicW:   map[schema.GroupVersionResource]*mockWatcher{},
			wantKinds:  nil,
		},
		{
			name:       "watcher missing in dynamicWatchers",
			policyName: "p1",
			policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr1}},
			dynamicW:   map[schema.GroupVersionResource]*mockWatcher{},
			wantKinds:  nil,
		},
		{
			name:       "resource without matching label",
			policyName: "p1",
			policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr1}},
			dynamicW: map[schema.GroupVersionResource]*mockWatcher{
				gvr1: {metadataCache: map[types.UID]Resource{
					"uid1": makeCached(makeUnstructured("", "apps", "v1", "ConfigMap", "res1", "ns1", "uid1", map[string]string{"foo": "bar"})),
				}},
			},
			wantKinds: nil,
		},
		{
			name:       "single matching resource",
			policyName: "p1",
			policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr1}},
			dynamicW: map[schema.GroupVersionResource]*mockWatcher{
				gvr1: {metadataCache: map[types.UID]Resource{
					"uid1": makeCached(makeUnstructured("", "apps", "v1", "Deployment", "res1", "ns1", "uid1", map[string]string{common.GeneratePolicyLabel: "p1"})),
				}},
			},
			wantKinds: []string{"Deployment"},
		},
		{
			name:       "multiple matching resources from one watcher",
			policyName: "p1",
			policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr1}},
			dynamicW: map[schema.GroupVersionResource]*mockWatcher{
				gvr1: {metadataCache: map[types.UID]Resource{
					"uid1": makeCached(makeUnstructured("", "apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{common.GeneratePolicyLabel: "p1"})),
					"uid2": makeCached(makeUnstructured("", "apps", "v1", "Service", "res2", "ns1", "uid2", map[string]string{common.GeneratePolicyLabel: "p1"})),
				}},
			},
			wantKinds: []string{"Pod", "Service"},
		},
		{
			name:       "multiple watchers with matches in both",
			policyName: "p1",
			policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr1, gvr2}},
			dynamicW: map[schema.GroupVersionResource]*mockWatcher{
				gvr1: {metadataCache: map[types.UID]Resource{
					"uid1": makeCached(makeUnstructured("", "apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{common.GeneratePolicyLabel: "p1"})),
				}},
				gvr2: {metadataCache: map[types.UID]Resource{
					"uid2": makeCached(makeUnstructured("", "apps", "v1", "ConfigMap", "res2", "ns1", "uid1", map[string]string{common.GeneratePolicyLabel: "p1"})),
				}},
			},
			wantKinds: []string{"Pod", "ConfigMap"},
		},
		{
			name:       "mixed matches and non-matches",
			policyName: "p1",
			policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr1}},
			dynamicW: map[schema.GroupVersionResource]*mockWatcher{
				gvr1: {metadataCache: map[types.UID]Resource{
					"uid1": makeCached(makeUnstructured("", "apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{"foo": "bar"})),
					"uid2": makeCached(makeUnstructured("", "apps", "v1", "Service", "res2", "ns1", "uid1", map[string]string{common.GeneratePolicyLabel: "p1"})),
				}},
			},
			wantKinds: []string{"Service"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dynamicWatchers := map[schema.GroupVersionResource]*watcher{}
			for gvr, mw := range tt.dynamicW {
				dynamicWatchers[gvr] = &watcher{
					metadataCache: mw.metadataCache,
				}
			}

			wm := &WatchManager{
				policyRefs:      tt.policyRefs,
				dynamicWatchers: dynamicWatchers,
				log:             logging.WithName("test"),
			}

			got := wm.GetDownstreams(tt.policyName)
			var kinds []string
			for _, obj := range got {
				kinds = append(kinds, obj.GetKind())
			}

			assert.ElementsMatch(t, tt.wantKinds, kinds)
		})
	}
}

func TestDeleteDownstreams(t *testing.T) {
	makeCached := func(obj *unstructured.Unstructured, labels map[string]string) Resource {
		obj.SetLabels(labels)
		return Resource{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
			Labels:    labels,
			Data:      obj,
		}
	}

	gvr1 := schema.GroupVersionResource{Group: "g", Version: "v1", Resource: "res1"}
	triggerUID := types.UID("trigger-123")

	tests := []struct {
		name           string
		policyName     string
		policyRefs     map[string][]schema.GroupVersionResource
		dynamicW       map[schema.GroupVersionResource]*watcher
		trigger        *kyverno.ResourceSpec
		clientErr      error
		wantDeleted    []string
		wantCacheSizes map[schema.GroupVersionResource]int
	}{
		{
			name:           "no watchers for policy",
			policyName:     "p1",
			policyRefs:     map[string][]schema.GroupVersionResource{},
			dynamicW:       map[schema.GroupVersionResource]*watcher{},
			wantDeleted:    nil,
			wantCacheSizes: map[schema.GroupVersionResource]int{},
		},
		{
			name:           "watcher missing in dynamicWatchers",
			policyName:     "p1",
			policyRefs:     map[string][]schema.GroupVersionResource{"p1": {gvr1}},
			dynamicW:       map[schema.GroupVersionResource]*watcher{},
			wantDeleted:    nil,
			wantCacheSizes: map[schema.GroupVersionResource]int{},
		},
		{
			name:       "no matching label",
			policyName: "p1",
			policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr1}},
			dynamicW: map[schema.GroupVersionResource]*watcher{
				gvr1: {metadataCache: map[types.UID]Resource{
					"uid1": makeCached(makeUnstructured("", "apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{"foo": "bar"}), map[string]string{"foo": "bar"}),
				}},
			},
			wantDeleted:    nil,
			wantCacheSizes: map[schema.GroupVersionResource]int{gvr1: 1},
		},
		{
			name:       "trigger UID does not match",
			policyName: "p1",
			policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr1}},
			trigger:    &kyverno.ResourceSpec{UID: triggerUID},
			dynamicW: map[schema.GroupVersionResource]*watcher{
				gvr1: {metadataCache: map[types.UID]Resource{
					"uid1": makeCached(makeUnstructured("", "apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{
						common.GeneratePolicyLabel:     "p1",
						common.GenerateTriggerUIDLabel: "other-uid",
					}), map[string]string{
						common.GeneratePolicyLabel:     "p1",
						common.GenerateTriggerUIDLabel: "other-uid",
					}),
				}},
			},
			wantDeleted:    nil,
			wantCacheSizes: map[schema.GroupVersionResource]int{gvr1: 1},
		},
		{
			name:       "trigger is nil deletes all matching",
			policyName: "p1",
			policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr1}},
			dynamicW: map[schema.GroupVersionResource]*watcher{
				gvr1: {metadataCache: map[types.UID]Resource{
					"uid1": makeCached(makeUnstructured("", "apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{
						common.GeneratePolicyLabel: "p1",
					}), map[string]string{
						common.GeneratePolicyLabel: "p1",
					}),
				}},
			},
			wantDeleted:    []string{"Pod/ns1/res1"},
			wantCacheSizes: map[schema.GroupVersionResource]int{gvr1: 0},
		},
		{
			name:       "trigger UID matches deletes only those",
			policyName: "p1",
			policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr1}},
			trigger:    &kyverno.ResourceSpec{UID: triggerUID},
			dynamicW: map[schema.GroupVersionResource]*watcher{
				gvr1: {metadataCache: map[types.UID]Resource{
					"uid1": makeCached(makeUnstructured("", "apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{
						common.GeneratePolicyLabel:     "p1",
						common.GenerateTriggerUIDLabel: string(triggerUID),
					}), map[string]string{
						common.GeneratePolicyLabel:     "p1",
						common.GenerateTriggerUIDLabel: string(triggerUID),
					}),
					"uid2": makeCached(makeUnstructured("", "apps", "v1", "Service", "res2", "ns1", "uid2", map[string]string{
						common.GeneratePolicyLabel:     "p1",
						common.GenerateTriggerUIDLabel: "other-uid",
					}), map[string]string{
						common.GeneratePolicyLabel:     "p1",
						common.GenerateTriggerUIDLabel: "other-uid",
					}),
				}},
			},
			wantDeleted:    []string{"Pod/ns1/res1"},
			wantCacheSizes: map[schema.GroupVersionResource]int{gvr1: 1},
		},
		{
			name:       "delete error keeps resource in cache",
			policyName: "p1",
			policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr1}},
			clientErr:  fmt.Errorf("delete failed"),
			dynamicW: map[schema.GroupVersionResource]*watcher{
				gvr1: {metadataCache: map[types.UID]Resource{
					"uid1": makeCached(makeUnstructured("", "apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{
						common.GeneratePolicyLabel: "p1",
					}), map[string]string{
						common.GeneratePolicyLabel: "p1",
					}),
				}},
			},
			wantDeleted:    []string{"Pod/ns1/res1"},
			wantCacheSizes: map[schema.GroupVersionResource]int{gvr1: 1},
		},
		{
			name:       "not found drops the cache entry",
			policyName: "p1",
			policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr1}},
			clientErr:  apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, "res1"),
			dynamicW: map[schema.GroupVersionResource]*watcher{
				gvr1: {metadataCache: map[types.UID]Resource{
					"uid1": makeCached(makeUnstructured("", "apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{
						common.GeneratePolicyLabel: "p1",
					}), map[string]string{
						common.GeneratePolicyLabel: "p1",
					}),
				}},
			},
			wantDeleted:    []string{"Pod/ns1/res1"},
			wantCacheSizes: map[schema.GroupVersionResource]int{gvr1: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &MockClient{err: tt.clientErr}
			wm := &WatchManager{
				policyRefs:      tt.policyRefs,
				dynamicWatchers: tt.dynamicW,
				client:          client,
				log:             logging.WithName("test"),
			}

			wm.DeleteDownstreams(tt.policyName, tt.trigger)

			assert.ElementsMatch(t, tt.wantDeleted, client.deleted)
			for gvr, wantSize := range tt.wantCacheSizes {
				assert.Equal(t, wantSize, len(wm.dynamicWatchers[gvr].metadataCache))
			}
		})
	}
}

func TestCleanupStaleDownstreams(t *testing.T) {
	makeCached := func(obj *unstructured.Unstructured, labels map[string]string) Resource {
		obj.SetLabels(labels)
		return Resource{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
			Labels:    labels,
			Data:      obj,
		}
	}

	gvr1 := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	triggerUID := types.UID("trigger-123")
	managedLabels := map[string]string{
		common.GeneratePolicyLabel:     "p1",
		common.GenerateTriggerUIDLabel: string(triggerUID),
	}
	otherTriggerLabels := map[string]string{
		common.GeneratePolicyLabel:     "p1",
		common.GenerateTriggerUIDLabel: "other-uid",
	}

	tests := []struct {
		name           string
		desired        []*unstructured.Unstructured
		dynamicW       map[schema.GroupVersionResource]*watcher
		wantDeleted    []string
		wantCacheSizes map[schema.GroupVersionResource]int
	}{
		{
			name:    "still desired resource is kept",
			desired: []*unstructured.Unstructured{makeUnstructured("", "", "v1", "ConfigMap", "cm1", "ns1", "uid1", managedLabels)},
			dynamicW: map[schema.GroupVersionResource]*watcher{
				gvr1: {metadataCache: map[types.UID]Resource{
					"uid1": makeCached(makeUnstructured("", "", "v1", "ConfigMap", "cm1", "ns1", "uid1", managedLabels), managedLabels),
				}},
			},
			wantDeleted:    nil,
			wantCacheSizes: map[schema.GroupVersionResource]int{gvr1: 1},
		},
		{
			name:    "empty desired set deletes trigger downstreams only",
			desired: nil,
			dynamicW: map[schema.GroupVersionResource]*watcher{
				gvr1: {metadataCache: map[types.UID]Resource{
					"uid1": makeCached(makeUnstructured("", "", "v1", "ConfigMap", "cm1", "ns1", "uid1", managedLabels), managedLabels),
					"uid2": makeCached(makeUnstructured("", "", "v1", "ConfigMap", "cm2", "ns2", "uid2", otherTriggerLabels), otherTriggerLabels),
				}},
			},
			wantDeleted:    []string{"ConfigMap/ns1/cm1"},
			wantCacheSizes: map[schema.GroupVersionResource]int{gvr1: 1},
		},
		{
			name:    "renamed downstream deletes stale one",
			desired: []*unstructured.Unstructured{makeUnstructured("", "", "v1", "ConfigMap", "cm-new", "ns1", "uid-new", managedLabels)},
			dynamicW: map[schema.GroupVersionResource]*watcher{
				gvr1: {metadataCache: map[types.UID]Resource{
					"uid1": makeCached(makeUnstructured("", "", "v1", "ConfigMap", "cm-old", "ns1", "uid1", managedLabels), managedLabels),
				}},
			},
			wantDeleted:    []string{"ConfigMap/ns1/cm-old"},
			wantCacheSizes: map[schema.GroupVersionResource]int{gvr1: 0},
		},
		{
			name:    "unmanaged resource is not deleted",
			desired: nil,
			dynamicW: map[schema.GroupVersionResource]*watcher{
				gvr1: {metadataCache: map[types.UID]Resource{
					"uid1": makeCached(makeUnstructured("", "", "v1", "ConfigMap", "cm1", "ns1", "uid1", map[string]string{"foo": "bar"}), map[string]string{"foo": "bar"}),
				}},
			},
			wantDeleted:    nil,
			wantCacheSizes: map[schema.GroupVersionResource]int{gvr1: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &MockClient{}
			wm := &WatchManager{
				policyRefs:      map[string][]schema.GroupVersionResource{"p1": {gvr1}},
				dynamicWatchers: tt.dynamicW,
				client:          client,
				log:             logging.WithName("test"),
			}

			wm.CleanupStaleDownstreams("p1", &kyverno.ResourceSpec{UID: triggerUID}, tt.desired)

			assert.ElementsMatch(t, tt.wantDeleted, client.deleted)
			for gvr, wantSize := range tt.wantCacheSizes {
				assert.Equal(t, wantSize, len(wm.dynamicWatchers[gvr].metadataCache))
			}
		})
	}
}

func TestCleanupStaleDownstreams_NotFoundDropsCache(t *testing.T) {
	gvr1 := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	labels := map[string]string{
		common.GeneratePolicyLabel:     "p1",
		common.GenerateTriggerUIDLabel: "trigger-123",
	}
	obj := makeUnstructured("", "", "v1", "ConfigMap", "cm-old", "ns1", "uid1", labels)
	wm := &WatchManager{
		log: logging.WithName("test"),
		client: &MockClient{
			err: apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "cm-old"),
		},
		policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr1}},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			gvr1: {metadataCache: map[types.UID]Resource{
				"uid1": {
					Name:      obj.GetName(),
					Namespace: obj.GetNamespace(),
					Labels:    labels,
					Data:      obj,
				},
			}},
		},
	}

	wm.CleanupStaleDownstreams("p1", &kyverno.ResourceSpec{UID: "trigger-123"}, nil)

	assert.Empty(t, wm.dynamicWatchers[gvr1].metadataCache)
}

func TestGetDownstreamsSkipsEmptyHash(t *testing.T) {
	gvr1 := schema.GroupVersionResource{Group: "g", Version: "v1", Resource: "res1"}
	labels := map[string]string{common.GeneratePolicyLabel: "p1"}
	live := makeUnstructured("", "apps", "v1", "Pod", "live", "ns1", "uid1", labels)
	retired := makeUnstructured("", "apps", "v1", "Pod", "retired", "ns1", "uid2", labels)
	wm := &WatchManager{
		log:        logging.WithName("test"),
		policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr1}},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			gvr1: {metadataCache: map[types.UID]Resource{
				"uid1": {Name: live.GetName(), Namespace: live.GetNamespace(), Labels: labels, Hash: "kept", Data: live},
				"uid2": {Name: retired.GetName(), Namespace: retired.GetNamespace(), Labels: labels, Hash: "", Data: retired},
			}},
		},
	}

	got := wm.GetDownstreams("p1")
	require.Len(t, got, 1)
	assert.Equal(t, "live", got[0].GetName())
}

func TestRemoveWatchersForPolicy(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}

	type fields struct {
		client          *MockClient
		dynamicWatchers map[schema.GroupVersionResource]*watcher
		policyRefs      map[string][]schema.GroupVersionResource
		refCount        map[schema.GroupVersionResource]int
	}
	type args struct {
		policyName       string
		deleteDownstream bool
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		wantDeleted   []string
		wantCacheSize int
	}{
		{
			name: "policy not found",
			fields: fields{
				client:          &MockClient{},
				dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
				policyRefs:      map[string][]schema.GroupVersionResource{},
				refCount:        map[schema.GroupVersionResource]int{},
			},
			args: args{"pol1", true},
		},
		{
			name: "watcher missing",
			fields: fields{
				client:          &MockClient{},
				dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
				policyRefs:      map[string][]schema.GroupVersionResource{"pol1": {gvr}},
				refCount:        map[schema.GroupVersionResource]int{},
			},
			args: args{"pol1", true},
		},
		{
			name: "delete downstream when deleteDownstream = true",
			fields: fields{
				client: &MockClient{},
				dynamicWatchers: map[schema.GroupVersionResource]*watcher{
					gvr: {
						metadataCache: map[types.UID]Resource{
							"uid1": {
								Name:      "res-test",
								Namespace: "isolated-ns",
								Labels:    map[string]string{common.GeneratePolicyLabel: "pol1"},
								Data:      makeUnstructured("1", "", "v1", "Pod", "res-test", "isolated-ns", "uid1", map[string]string{common.GeneratePolicyLabel: "pol1"}),
							},
						},
						watcher: watch.MockWatcher{
							StopFunc: func() {},
						},
					},
				},
				policyRefs: map[string][]schema.GroupVersionResource{"pol1": {gvr}},
				refCount:   map[schema.GroupVersionResource]int{gvr: 1},
			},
			args:          args{"pol1", true},
			wantDeleted:   []string{"Pod/isolated-ns/res-test"},
			wantCacheSize: 0,
		},
		{
			name: "skip delete when deleteDownstream = false",
			fields: fields{
				client: &MockClient{},
				dynamicWatchers: map[schema.GroupVersionResource]*watcher{
					gvr: {
						metadataCache: map[types.UID]Resource{
							"uid1": {
								Name:      "res-test",
								Namespace: "isolated-ns",
								Labels:    map[string]string{common.GeneratePolicyLabel: "pol1"},
								Data:      makeUnstructured("1", "", "v1", "Pod", "res-test", "isolated-ns", "uid1", nil),
							},
						},
						watcher: watch.MockWatcher{
							StopFunc: func() {},
						},
					},
				},
				policyRefs: map[string][]schema.GroupVersionResource{"pol1": {gvr}},
				refCount:   map[schema.GroupVersionResource]int{gvr: 1},
			},
			args:          args{"pol1", false},
			wantDeleted:   nil,
			wantCacheSize: 0,
		},
		{
			name: "skip delete when GenerateSourceUIDLabel present",
			fields: fields{
				client: &MockClient{},
				dynamicWatchers: map[schema.GroupVersionResource]*watcher{
					gvr: {
						metadataCache: map[types.UID]Resource{
							"uid1": {
								Name:      "res-test",
								Namespace: "isolated-ns",
								Labels: map[string]string{
									common.GeneratePolicyLabel:    "pol1",
									common.GenerateSourceUIDLabel: "src-uid",
								},
								Data: makeUnstructured("1", "", "v1", "Pod", "res-test", "isolated-ns", "uid1", nil),
							},
						},
						watcher: watch.MockWatcher{
							StopFunc: func() {},
						},
					},
				},
				policyRefs: map[string][]schema.GroupVersionResource{"pol1": {gvr}},
				refCount:   map[schema.GroupVersionResource]int{gvr: 1},
			},
			args:          args{"pol1", true},
			wantDeleted:   nil,
			wantCacheSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wm := &WatchManager{
				log:             logging.WithName("test"),
				client:          tt.fields.client,
				dynamicWatchers: tt.fields.dynamicWatchers,
				policyRefs:      tt.fields.policyRefs,
				refCount:        tt.fields.refCount,
			}

			wm.RemoveWatchersForPolicy(tt.args.policyName, tt.args.deleteDownstream)

			assert.Equal(t, tt.wantDeleted, tt.fields.client.deleted, "deleted resources mismatch")

			for _, watcher := range wm.dynamicWatchers {
				assert.Equal(t, tt.wantCacheSize, len(watcher.metadataCache), "metadataCache size mismatch")
			}
		})
	}
}

func TestRemoveWatchersForPolicy_StaleRefCountRemovedWhenWatcherMissing(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	wm := &WatchManager{
		log:             logging.WithName("test"),
		client:          &MockClient{},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
		policyRefs:      map[string][]schema.GroupVersionResource{"pol1": {gvr}},
		refCount:        map[schema.GroupVersionResource]int{gvr: 1},
	}

	wm.RemoveWatchersForPolicy("pol1", true)

	_, refExists := wm.refCount[gvr]
	assert.False(t, refExists)
	_, policyRefExists := wm.policyRefs["pol1"]
	assert.False(t, policyRefExists)
}

type mockStopper struct {
	stopped bool
}

func (m *mockStopper) Stop() {
	m.stopped = true
}
func (m *mockStopper) ResultChan() <-chan watch.Event {
	return nil
}

func TestStopWatchers(t *testing.T) {
	wm := &WatchManager{
		lock:            sync.Mutex{},
		dynamicWatchers: make(map[schema.GroupVersionResource]*watcher),
		policyRefs:      make(map[string][]schema.GroupVersionResource),
		refCount:        make(map[schema.GroupVersionResource]int),
	}

	gvr1 := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	mock1 := &mockStopper{}
	wm.dynamicWatchers[gvr1] = &watcher{watcher: mock1}
	wm.policyRefs["policy1"] = []schema.GroupVersionResource{gvr1}
	wm.refCount[gvr1] = 1

	gvr2 := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	mock2 := &mockStopper{}
	wm.dynamicWatchers[gvr2] = &watcher{watcher: mock2}
	wm.policyRefs["policy2"] = []schema.GroupVersionResource{gvr2}
	wm.refCount[gvr2] = 2

	wm.StopWatchers()

	assert.True(t, mock1.stopped, "Expected watcher for %v to be stopped", gvr1)
	assert.True(t, mock2.stopped, "Expected watcher for %v to be stopped", gvr2)

	assert.Empty(t, wm.dynamicWatchers, "Expected dynamicWatchers to be empty")
	assert.Empty(t, wm.policyRefs, "Expected policyRefs to be empty")
	assert.Empty(t, wm.refCount, "Expected refCount to be empty")
}

func TestHandleDelete_SourceDeleted(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "networkpolicies"}

	makeSource := func(uid, name, ns string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetAPIVersion("v1")
		u.SetKind("Namespace")
		u.SetUID(types.UID(uid))
		u.SetName(name)
		u.SetNamespace(ns)
		// No kyverno managed-by label — this is a source resource.
		return u
	}

	makeDownstream := func(uid, name, ns, sourceUID string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetAPIVersion("networking.k8s.io/v1")
		u.SetKind("NetworkPolicy")
		u.SetUID(types.UID(uid))
		u.SetName(name)
		u.SetNamespace(ns)
		u.SetLabels(map[string]string{common.GenerateSourceUIDLabel: sourceUID})
		return u
	}

	tests := []struct {
		name          string
		deleteErr     error
		wantCacheSize int
	}{
		{
			name:          "delete succeeds: downstream removed from cache",
			deleteErr:     nil,
			wantCacheSize: 0,
		},
		{
			name:          "delete fails: downstream must remain in cache",
			deleteErr:     fmt.Errorf("permission denied"),
			wantCacheSize: 1,
		},
		{
			name:          "delete not found: downstream removed from cache",
			deleteErr:     apierrors.NewNotFound(schema.GroupResource{Resource: "networkpolicies"}, "np-default"),
			wantCacheSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			downstream := makeDownstream("down-uid", "np-default", "default", "src-uid")

			mockClient := &MockClient{
				err: tt.deleteErr,
				// ListResource returns the downstream so handleDelete can find it.
				// The mock ListResource always returns a fixed item; override here
				// to return our downstream.
			}
			// Override ListResource to return the downstream keyed by source UID.
			_ = mockClient

			// Build a WatchManager whose watcher for gvr tracks the downstream.
			w := &watcher{
				metadataCache: map[types.UID]Resource{
					"down-uid": {
						Name:      downstream.GetName(),
						Namespace: downstream.GetNamespace(),
						Labels:    downstream.GetLabels(),
						Data:      downstream,
					},
				},
			}

			// We need ListResource to return our downstream. The default MockClient
			// always returns a single blank item, which won't match "down-uid" in the
			// cache check. Use a custom deleteFn-style mock for the list instead by
			// constructing a specialised client inline.
			type listAndDeleteClient struct {
				MockClient
				listFn func() (*unstructured.UnstructuredList, error)
			}

			specialClient := &MockClient{
				deleteFn: func(ctx context.Context, apiVersion, kind, namespace, name string, dryRun bool, options metav1.DeleteOptions) error {
					return tt.deleteErr
				},
			}
			// Patch ListResource on the WatchManager directly using a closure client.
			_ = specialClient

			// Use a simpler approach: build the WatchManager and call handleDelete
			// with a pre-seeded metadataCache that already contains the downstream.
			// handleDelete's source path calls ListResource to find downstreams; we
			// need the list to return "down-uid". The default MockClient.ListResource
			// returns a blank Unstructured (empty UID), so the cache lookup at line
			// 490 returns !exists → the downstream is skipped.
			//
			// To exercise the delete path, use a MockClient whose ListResource returns
			// the real downstream object.
			realClient := &struct {
				MockClient
			}{}
			realClient.MockClient = MockClient{
				deleteFn: func(ctx context.Context, apiVersion, kind, namespace, name string, dryRun bool, options metav1.DeleteOptions) error {
					return tt.deleteErr
				},
			}

			wm := &WatchManager{
				log:    logging.WithName("test"),
				client: &realClient.MockClient,
				dynamicWatchers: map[schema.GroupVersionResource]*watcher{
					gvr: w,
				},
			}

			// Override ListResource to return the downstream so that handleDelete
			// finds it and attempts to delete it. We achieve this by seeding the
			// mock's return value via a custom listFn wrapper. Since MockClient
			// does not support per-call list overrides, we test handleDelete directly
			// with a MockClient that returns our downstream from ListResource by
			// temporarily replacing the client on the WatchManager.
			listClient := &fullMockClient{
				listResult: &unstructured.UnstructuredList{Items: []unstructured.Unstructured{*downstream}},
				deleteErr:  tt.deleteErr,
			}
			wm.client = listClient

			src := makeSource("src-uid", "prod-ns", "")
			wm.handleDelete(src, gvr)

			assert.Equal(t, tt.wantCacheSize, len(w.metadataCache),
				"metadataCache size after handleDelete with deleteErr=%v", tt.deleteErr)
		})
	}
}

func TestHandleDelete_InvalidatedDownstreamDoesNotRecreate(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	labels := map[string]string{kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp}
	downstream := makeUnstructured("1", "", "v1", "ConfigMap", "test-cm", "default", "downstream-uid", labels)
	client := &MockClient{}
	wm := &WatchManager{
		log:    logging.WithName("test"),
		client: client,
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			gvr: {
				metadataCache: map[types.UID]Resource{
					downstream.GetUID(): {
						Name:      downstream.GetName(),
						Namespace: downstream.GetNamespace(),
						Labels:    downstream.GetLabels(),
						Data:      downstream,
					},
				},
			},
		},
	}

	wm.handleDelete(downstream, gvr)

	assert.Empty(t, client.created)
}

// TestHandleDelete_DownstreamRecreatedAfterRepeatedDeletions is the regression test
// for https://github.com/kyverno/kyverno/issues/17265: the metadata cache is keyed by
// UID, and the API server assigns a fresh UID every time the downstream is recreated.
// If the cache entry is not re-registered under the new UID after a recreation, the
// second user deletion looks up the new UID, misses the cache, and the resource is
// never recreated again.
func TestHandleDelete_DownstreamRecreatedAfterRepeatedDeletions(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	labels := map[string]string{
		kyverno.LabelAppManagedBy:      kyverno.ValueKyvernoApp,
		common.GeneratePolicyLabel:     "test-policy",
		common.GenerateTriggerUIDLabel: "trigger-uid",
	}
	downstream := makeUnstructured("1", "", "v1", "ConfigMap", "test-cm", "default", "uid-1", labels)

	newWatchManager := func(client dclient.Interface, cache map[types.UID]Resource) *WatchManager {
		return &WatchManager{
			log:    logging.WithName("test"),
			client: client,
			dynamicWatchers: map[schema.GroupVersionResource]*watcher{
				gvr: {metadataCache: cache},
			},
		}
	}
	seedCache := func() map[types.UID]Resource {
		return map[types.UID]Resource{
			downstream.GetUID(): {
				Name:      downstream.GetName(),
				Namespace: downstream.GetNamespace(),
				Labels:    downstream.GetLabels(),
				Hash:      reportutils.CalculateResourceHash(*downstream),
				Data:      downstream,
			},
		}
	}

	t.Run("cache is re-keyed to the new UID and every deletion is reverted", func(t *testing.T) {
		uidCounter := 1
		client := &MockClient{
			createFn: func(_ context.Context, _, _, _ string, obj interface{}, _ bool) (*unstructured.Unstructured, error) {
				// mimic the API server assigning a fresh UID on each creation
				uidCounter++
				created := obj.(*unstructured.Unstructured).DeepCopy()
				created.SetUID(types.UID(fmt.Sprintf("uid-%d", uidCounter)))
				return created, nil
			},
		}
		wm := newWatchManager(client, seedCache())
		cache := wm.dynamicWatchers[gvr].metadataCache

		// 1st user deletion: the downstream is recreated with a new UID (uid-2)
		wm.handleDelete(downstream, gvr)
		assert.Len(t, client.created, 1)
		assert.NotContains(t, cache, types.UID("uid-1"), "stale UID must be dropped from the cache")
		require.Contains(t, cache, types.UID("uid-2"), "cache must be re-keyed to the recreated resource UID")
		assert.Equal(t, "test-cm", cache["uid-2"].Name)

		// 2nd user deletion: the watch delivers the delete event for the
		// recreated resource (uid-2); it must be recreated again
		recreated := downstream.DeepCopy()
		recreated.SetUID("uid-2")
		wm.handleDelete(recreated, gvr)
		assert.Len(t, client.created, 2, "the downstream must be recreated after every deletion")
		assert.NotContains(t, cache, types.UID("uid-2"))
		assert.Contains(t, cache, types.UID("uid-3"))
	})

	t.Run("create failure keeps the cache entry under the old UID", func(t *testing.T) {
		client := &MockClient{
			createFn: func(_ context.Context, _, _, _ string, _ interface{}, _ bool) (*unstructured.Unstructured, error) {
				return nil, fmt.Errorf("api server unavailable")
			},
		}
		wm := newWatchManager(client, seedCache())
		cache := wm.dynamicWatchers[gvr].metadataCache

		wm.handleDelete(downstream, gvr)
		assert.Contains(t, cache, types.UID("uid-1"), "cache entry must be kept when recreation fails")
	})

	t.Run("missing namespace drops the cache entry", func(t *testing.T) {
		client := &MockClient{
			createFn: func(_ context.Context, _, _, _ string, _ interface{}, _ bool) (*unstructured.Unstructured, error) {
				return nil, fmt.Errorf("namespace is gone")
			},
			getFn: func(_ context.Context, _, kind, _, name string) (*unstructured.Unstructured, error) {
				return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, name)
			},
		}
		wm := newWatchManager(client, seedCache())
		cache := wm.dynamicWatchers[gvr].metadataCache

		wm.handleDelete(downstream, gvr)
		assert.NotContains(t, cache, types.UID("uid-1"))
	})

	t.Run("cluster scoped create failure keeps the cache entry", func(t *testing.T) {
		cluster := downstream.DeepCopy()
		cluster.SetNamespace("")
		client := &MockClient{
			createFn: func(_ context.Context, _, _, _ string, _ interface{}, _ bool) (*unstructured.Unstructured, error) {
				return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, "default")
			},
		}
		cache := map[types.UID]Resource{
			cluster.GetUID(): {
				Name:      cluster.GetName(),
				Namespace: "",
				Labels:    cluster.GetLabels(),
				Hash:      reportutils.CalculateResourceHash(*cluster),
				Data:      cluster,
			},
		}
		wm := newWatchManager(client, cache)

		wm.handleDelete(cluster, gvr)
		assert.Contains(t, wm.dynamicWatchers[gvr].metadataCache, cluster.GetUID())
	})

	t.Run("nil created object keeps the cache entry under the old UID", func(t *testing.T) {
		client := &MockClient{}
		wm := newWatchManager(client, seedCache())
		cache := wm.dynamicWatchers[gvr].metadataCache

		wm.handleDelete(downstream, gvr)
		assert.Len(t, client.created, 1)
		assert.Contains(t, cache, types.UID("uid-1"))
	})
}

// fullMockClient is a purpose-built mock that allows controlling both
// ListResource and DeleteResource return values independently.
type fullMockClient struct {
	MockClient
	listResult *unstructured.UnstructuredList
	deleteErr  error
}

func (f *fullMockClient) ListResource(_ context.Context, _, _, _ string, _ *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return f.listResult, nil
}

func (f *fullMockClient) DeleteResource(_ context.Context, _, _, _, _ string, _ bool, _ metav1.DeleteOptions) error {
	return f.deleteErr
}

func TestHandleAdd(t *testing.T) {
	tests := []struct {
		name    string
		objName string
		gvr     schema.GroupVersionResource
		wantMsg string
	}{
		{
			name:    "simple add",
			objName: "test-object",
			gvr:     schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			wantMsg: "Resource added name=test-object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wm := &WatchManager{
				log: logging.GlobalLogger(),
			}

			obj := &unstructured.Unstructured{}
			obj.SetName(tt.objName)

			wm.handleAdd(obj, tt.gvr)
		})
	}
}

func TestHandleUpdate(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}

	makeObj := func(uid, name, ns string, labels map[string]string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetAPIVersion("v1")
		u.SetKind("Pod")
		u.SetUID(types.UID(uid))
		u.SetName(name)
		u.SetNamespace(ns)
		u.SetLabels(labels)
		return u
	}

	t.Run("source object updates downstream", func(t *testing.T) {
		mockClient := &MockClient{}
		downstream := makeObj("down-uid", "down-pod", "default", map[string]string{common.GenerateSourceUIDLabel: "src-uid"})

		wm := &WatchManager{
			client: mockClient,
			dynamicWatchers: map[schema.GroupVersionResource]*watcher{
				gvr: {metadataCache: map[types.UID]Resource{
					"down-uid": {
						Name:      downstream.GetName(),
						Namespace: downstream.GetNamespace(),
						Labels:    downstream.GetLabels(),
					},
				}},
			},
		}

		src := makeObj("src-uid", "src-pod", "default", nil)
		wm.handleUpdate(src, gvr)
	})

	t.Run("downstream changed by user gets reverted", func(t *testing.T) {
		mockClient := &MockClient{}
		downstream := makeObj("down-uid", "down-pod", "default", nil)
		hashOld := reportutils.CalculateResourceHash(*downstream)
		downstreamModified := downstream.DeepCopy()
		downstreamModified.SetAnnotations(map[string]string{"changed": "true"})

		wm := &WatchManager{
			client: mockClient,
			dynamicWatchers: map[schema.GroupVersionResource]*watcher{
				gvr: {metadataCache: map[types.UID]Resource{
					"down-uid": {
						Name:      downstream.GetName(),
						Namespace: downstream.GetNamespace(),
						Labels:    downstream.GetLabels(),
						Hash:      hashOld,
						Data:      downstream,
					},
				}},
			},
		}

		wm.handleUpdate(downstreamModified, gvr)
	})

	t.Run("object not in watchers - no action", func(t *testing.T) {
		mockClient := &MockClient{}
		wm := &WatchManager{
			client:          mockClient,
			dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
		}

		obj := makeObj("uid", "pod", "default", nil)
		wm.handleUpdate(obj, gvr)
	})

	t.Run("invalidated downstream does not revert", func(t *testing.T) {
		policyName := "test-policy"
		labels := map[string]string{
			kyverno.LabelAppManagedBy:  kyverno.ValueKyvernoApp,
			common.GeneratePolicyLabel: policyName,
		}
		old := makeObj("down-uid", "down-pod", "default", labels)
		old.Object["data"] = map[string]any{"value": "old"}
		updated := old.DeepCopy()
		updated.Object["data"] = map[string]any{"value": "new"}
		client := &MockClient{}
		wm := &WatchManager{
			client: client,
			policyRefs: map[string][]schema.GroupVersionResource{
				policyName: {gvr},
			},
			dynamicWatchers: map[schema.GroupVersionResource]*watcher{
				gvr: {
					metadataCache: map[types.UID]Resource{
						old.GetUID(): {
							Name:      old.GetName(),
							Namespace: old.GetNamespace(),
							Labels:    old.GetLabels(),
							Hash:      reportutils.CalculateResourceHash(*old),
							Data:      old,
						},
					},
				},
			},
		}

		wm.InvalidateDownstreams(policyName, nil)
		wm.handleUpdate(updated, gvr)
		assert.Empty(t, client.updated)
	})

	// The write ProcessUR makes to a downstream lands before SyncWatchers can
	// refresh the cache with its hash, so a watch event for that write can reach
	// handleUpdate while the cache is stale. BeginGenerate marks that window so
	// the mismatch is not reverted as a user edit.
	newWM := func(policyName string, triggerUID types.UID) (*WatchManager, *MockClient, *unstructured.Unstructured) {
		labels := map[string]string{
			common.GeneratePolicyLabel:     policyName,
			common.GenerateTriggerUIDLabel: string(triggerUID),
		}
		cached := makeObj("down-uid", "down-pod", "default", labels)
		client := &MockClient{}
		wm := &WatchManager{
			log:    logging.WithName("test"),
			client: client,
			dynamicWatchers: map[schema.GroupVersionResource]*watcher{
				gvr: {metadataCache: map[types.UID]Resource{
					"down-uid": {Name: cached.GetName(), Namespace: cached.GetNamespace(), Labels: labels, Hash: reportutils.CalculateResourceHash(*cached), Data: cached},
				}},
			},
		}
		return wm, client, cached
	}
	withAnnotation := func(obj *unstructured.Unstructured, key, value string) *unstructured.Unstructured {
		out := obj.DeepCopy()
		out.SetAnnotations(map[string]string{key: value})
		return out
	}
	// refreshCache mimics SyncWatchers recording obj as the desired state.
	refreshCache := func(wm *WatchManager, obj *unstructured.Unstructured) {
		wm.lock.Lock()
		defer wm.lock.Unlock()
		entry := wm.dynamicWatchers[gvr].metadataCache[obj.GetUID()]
		entry.Hash = reportutils.CalculateResourceHash(*obj)
		entry.Data = obj
		wm.dynamicWatchers[gvr].metadataCache[obj.GetUID()] = entry
	}

	t.Run("in-flight generate write is not reverted", func(t *testing.T) {
		wm, client, cached := newWM("generate-secret", "trigger-uid-1")
		own := withAnnotation(cached, "tier", "free")

		end := wm.BeginGenerate("generate-secret", "trigger-uid-1")
		wm.handleUpdate(own, gvr)
		assert.Empty(t, client.updated, "a write Kyverno is still making for this trigger must not be reverted")
		refreshCache(wm, own)
		end()
		assert.Empty(t, client.updated, "the write is now the cached state, there is nothing to revert")
	})

	t.Run("edit made while a generate write is in flight is reverted when it ends", func(t *testing.T) {
		wm, client, cached := newWM("generate-secret", "trigger-uid-1")
		own := withAnnotation(cached, "tier", "free")
		edited := withAnnotation(cached, "tier", "premium")

		end := wm.BeginGenerate("generate-secret", "trigger-uid-1")
		wm.handleUpdate(own, gvr)
		wm.handleUpdate(edited, gvr)
		assert.Empty(t, client.updated, "nothing is reverted while the write is in flight")
		refreshCache(wm, own)
		end()
		assert.Len(t, client.updated, 1, "the edit skipped inside the window must be reverted when it closes")
	})

	t.Run("only the latest event skipped in the window is checked", func(t *testing.T) {
		wm, client, cached := newWM("generate-secret", "trigger-uid-1")
		own := withAnnotation(cached, "tier", "free")
		edited := withAnnotation(cached, "tier", "premium")

		end := wm.BeginGenerate("generate-secret", "trigger-uid-1")
		wm.handleUpdate(edited, gvr)
		wm.handleUpdate(own, gvr)
		refreshCache(wm, own)
		end()
		assert.Empty(t, client.updated, "the edit was undone before the window closed")
	})

	t.Run("skipped event is dropped when its cache entry is gone at the end", func(t *testing.T) {
		wm, client, cached := newWM("generate-secret", "trigger-uid-1")

		end := wm.BeginGenerate("generate-secret", "trigger-uid-1")
		wm.handleUpdate(withAnnotation(cached, "tier", "premium"), gvr)
		wm.lock.Lock()
		delete(wm.dynamicWatchers[gvr].metadataCache, "down-uid")
		wm.lock.Unlock()
		end()
		assert.Empty(t, client.updated)
	})

	t.Run("revert still applies once the generate write completes", func(t *testing.T) {
		wm, client, cached := newWM("generate-secret", "trigger-uid-1")

		end := wm.BeginGenerate("generate-secret", "trigger-uid-1")
		end()
		wm.handleUpdate(withAnnotation(cached, "tampered", "true"), gvr)
		assert.NotEmpty(t, client.updated, "once the generate write is no longer in flight, real drift must still be reverted")
	})

	t.Run("a write in flight for another trigger does not suppress the revert", func(t *testing.T) {
		wm, client, cached := newWM("generate-secret", "trigger-uid-1")

		end := wm.BeginGenerate("generate-secret", "trigger-uid-2")
		wm.handleUpdate(withAnnotation(cached, "tampered", "true"), gvr)
		end()
		assert.NotEmpty(t, client.updated)
	})

	t.Run("BeginGenerate ref-counts overlapping writes for the same trigger", func(t *testing.T) {
		wm, client, cached := newWM("generate-secret", "trigger-uid-1")
		tampered := withAnnotation(cached, "tampered", "true")

		end1 := wm.BeginGenerate("generate-secret", "trigger-uid-1")
		end2 := wm.BeginGenerate("generate-secret", "trigger-uid-1")
		end1()
		wm.handleUpdate(tampered, gvr)
		assert.Empty(t, client.updated, "still in flight: one BeginGenerate call is still outstanding")
		end2()
		assert.Len(t, client.updated, 1, "both calls have ended: the skipped edit is reverted")
		wm.handleUpdate(tampered, gvr)
		assert.Len(t, client.updated, 2, "no longer in flight: drift is reverted immediately")
	})

	t.Run("ending a generate window twice does not end an overlapping one", func(t *testing.T) {
		wm, client, cached := newWM("generate-secret", "trigger-uid-1")

		end1 := wm.BeginGenerate("generate-secret", "trigger-uid-1")
		end2 := wm.BeginGenerate("generate-secret", "trigger-uid-1")
		end1()
		end1()
		wm.handleUpdate(withAnnotation(cached, "tampered", "true"), gvr)
		assert.Empty(t, client.updated, "the second window is still open")
		end2()
	})
}

func TestWatchManager_CacheIntegrity(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}

	makeObj := func(uid, name, ns, rv string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetAPIVersion("v1")
		u.SetKind("ConfigMap")
		u.SetUID(types.UID(uid))
		u.SetName(name)
		u.SetNamespace(ns)
		u.SetResourceVersion(rv)
		u.SetCreationTimestamp(metav1.Now())
		return u
	}

	tests := []struct {
		name     string
		testFunc func(t *testing.T, wm *WatchManager, gvr schema.GroupVersionResource, cachedObj *unstructured.Unstructured, modifiedObj *unstructured.Unstructured)
	}{
		{
			name: "handleUpdate must not mutate cached object",
			testFunc: func(t *testing.T, wm *WatchManager, gvr schema.GroupVersionResource, cachedObj *unstructured.Unstructured, modifiedObj *unstructured.Unstructured) {
				wm.handleUpdate(modifiedObj, gvr)
			},
		},
		{
			name: "handleDelete must not mutate cached object",
			testFunc: func(t *testing.T, wm *WatchManager, gvr schema.GroupVersionResource, cachedObj *unstructured.Unstructured, modifiedObj *unstructured.Unstructured) {
				deletedObj := cachedObj.DeepCopy()
				deletedObj.SetLabels(map[string]string{"app.kubernetes.io/managed-by": "kyverno"})
				wm.handleDelete(deletedObj, gvr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalUID := types.UID("original-uid-12345")
			originalRV := "12345"
			cachedObj := makeObj(string(originalUID), "test-cm", "default", originalRV)
			originalHash := reportutils.CalculateResourceHash(*cachedObj)

			wm := &WatchManager{
				log:    logging.WithName("test"),
				client: &MockClient{},
				dynamicWatchers: map[schema.GroupVersionResource]*watcher{
					gvr: {
						metadataCache: map[types.UID]Resource{
							originalUID: {
								Name:      cachedObj.GetName(),
								Namespace: cachedObj.GetNamespace(),
								Labels:    cachedObj.GetLabels(),
								Hash:      originalHash,
								Data:      cachedObj,
							},
						},
					},
				},
			}

			modifiedObj := cachedObj.DeepCopy()
			modifiedObj.SetAnnotations(map[string]string{"modified": "true"})

			tt.testFunc(t, wm, gvr, cachedObj, modifiedObj)

			cached, exists := wm.dynamicWatchers[gvr].metadataCache[originalUID]
			assert.True(t, exists, "cached resource should still exist in map")
			assert.Equal(t, originalUID, cached.Data.GetUID(), "cached UID must not be mutated")
			assert.Equal(t, originalRV, cached.Data.GetResourceVersion(), "cached ResourceVersion must not be mutated")
			ts := cached.Data.GetCreationTimestamp()
			assert.False(t, ts.IsZero(), "cached CreationTimestamp must not be zeroed")
		})
	}
}

func TestWatcherCleanup_DeadWatcherMarkedStoppedOnExit(t *testing.T) {
	testGVR := schema.GroupVersionResource{Group: "g", Version: "v1", Resource: "res"}
	wm := &WatchManager{
		log:    logging.WithName("test"),
		client: dclient.NewEmptyFakeClient(),
		restMapper: &mockRESTMapper{
			fn: func(gk schema.GroupKind, version string) (*meta.RESTMapping, error) {
				return &meta.RESTMapping{Resource: testGVR}, nil
			},
		},
		dynamicWatchers: make(map[schema.GroupVersionResource]*watcher),
		policyRefs:      make(map[string][]schema.GroupVersionResource),
		refCount:        make(map[schema.GroupVersionResource]int),
	}

	resource := makeUnstructured("1", "g", "v1", "Kind", "n", "ns", "uid1", nil)
	err := wm.SyncWatchers("test-policy", nil, []*unstructured.Unstructured{resource}, false)
	require.NoError(t, err)

	wm.lock.Lock()
	w, exists := wm.dynamicWatchers[testGVR]
	wm.lock.Unlock()
	require.True(t, exists)

	w.watcher.Stop()

	require.Eventually(t, func() bool {
		wm.lock.Lock()
		defer wm.lock.Unlock()
		existing, stillExists := wm.dynamicWatchers[testGVR]
		return stillExists && existing.watcher == nil
	}, 2*time.Second, 10*time.Millisecond)
}

func TestWatcherCleanup_RestartPreservesMetadataCache(t *testing.T) {
	testGVR := schema.GroupVersionResource{Group: "g", Version: "v1", Resource: "res"}
	wm := &WatchManager{
		log:    logging.WithName("test"),
		client: dclient.NewEmptyFakeClient(),
		restMapper: &mockRESTMapper{
			fn: func(gk schema.GroupKind, version string) (*meta.RESTMapping, error) {
				return &meta.RESTMapping{Resource: testGVR}, nil
			},
		},
		dynamicWatchers: make(map[schema.GroupVersionResource]*watcher),
		policyRefs:      make(map[string][]schema.GroupVersionResource),
		refCount:        make(map[schema.GroupVersionResource]int),
	}

	first := makeUnstructured("1", "g", "v1", "Kind", "n1", "ns", "uid1", nil)
	err := wm.SyncWatchers("test-policy", nil, []*unstructured.Unstructured{first}, false)
	require.NoError(t, err)

	wm.lock.Lock()
	w := wm.dynamicWatchers[testGVR]
	wm.lock.Unlock()
	require.NotNil(t, w)
	w.watcher.Stop()

	require.Eventually(t, func() bool {
		wm.lock.Lock()
		defer wm.lock.Unlock()
		existing, ok := wm.dynamicWatchers[testGVR]
		return ok && existing.watcher == nil
	}, 2*time.Second, 10*time.Millisecond)

	second := makeUnstructured("2", "g", "v1", "Kind", "n2", "ns", "uid2", nil)
	err = wm.SyncWatchers("test-policy", nil, []*unstructured.Unstructured{second}, false)
	require.NoError(t, err)

	wm.lock.Lock()
	defer wm.lock.Unlock()
	restarted := wm.dynamicWatchers[testGVR]
	require.NotNil(t, restarted)
	require.NotNil(t, restarted.watcher)
	_, hasFirst := restarted.metadataCache["uid1"]
	_, hasSecond := restarted.metadataCache["uid2"]
	assert.True(t, hasFirst)
	assert.True(t, hasSecond)
}

// Labels copied from a genuine downstream do not register a different UID in
// the CEL controller's cache. Only controller-generated objects can be reverted.
func TestHandleDownstreamEvent_RequiresRegisteredUID(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"update", "delete"} {
		for _, registered := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/registered=%t", operation, registered), func(t *testing.T) {
				t.Parallel()
				gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
				labels := map[string]string{
					kyverno.LabelAppManagedBy:      kyverno.ValueKyvernoApp,
					common.GeneratePolicyLabel:     "test-policy",
					common.GenerateTriggerUIDLabel: "trigger-uid",
				}
				genuine := makeUnstructured("1", "", "v1", "ConfigMap", "generated", "tenant", "generated-uid", labels)
				genuine.Object["data"] = map[string]any{"value": "original"}
				client := &MockClient{}
				wm := &WatchManager{
					log:    logging.WithName("test"),
					client: client,
					dynamicWatchers: map[schema.GroupVersionResource]*watcher{
						gvr: {metadataCache: map[types.UID]Resource{
							genuine.GetUID(): {
								Name:      genuine.GetName(),
								Namespace: genuine.GetNamespace(),
								Labels:    genuine.GetLabels(),
								Hash:      reportutils.CalculateResourceHash(*genuine),
								Data:      genuine,
							},
						}},
					},
				}
				event := genuine.DeepCopy()
				if !registered {
					// A replacement with the same name, namespace and labels has a
					// different UID from the registered downstream.
					event.SetUID("replacement-uid")
				}
				if operation == "update" {
					event.Object["data"] = map[string]any{"value": "changed"}
					wm.handleUpdate(event, gvr)
				} else {
					wm.handleDelete(event, gvr)
				}

				if registered && operation == "update" {
					assert.Len(t, client.updated, 1, "genuine downstream edits must be reverted")
				} else {
					assert.Empty(t, client.updated)
				}
				if registered && operation == "delete" {
					assert.Len(t, client.created, 1, "genuine downstream deletions must be reverted")
				} else {
					assert.Empty(t, client.created)
				}
				assert.Empty(t, client.deleted)
				assert.NotContains(t, wm.dynamicWatchers[gvr].metadataCache, types.UID("replacement-uid"))
			})
		}
	}
}

type downstreamListClient struct {
	MockClient
	downstream unstructured.Unstructured
}

func (c *downstreamListClient) ListResource(context.Context, string, string, string, *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return &unstructured.UnstructuredList{Items: []unstructured.Unstructured{c.downstream}}, nil
}

func TestHandleSourceEvent_RequiresRegisteredDownstreamUID(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"update", "delete"} {
		for _, tt := range []struct {
			name      string
			cachedUID types.UID
		}{
			{name: "absent"},
			{name: "replacement", cachedUID: "old-downstream-uid"},
			{name: "registered", cachedUID: "downstream-uid"},
		} {
			t.Run(fmt.Sprintf("%s/%s", operation, tt.name), func(t *testing.T) {
				t.Parallel()
				gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
				source := makeUnstructured("1", "", "v1", "ConfigMap", "source", "tenant", "source-uid", nil)
				source.Object["data"] = map[string]any{"value": "source"}
				downstream := makeUnstructured("2", "", "v1", "ConfigMap", "generated", "tenant", "downstream-uid", map[string]string{
					kyverno.LabelAppManagedBy:     kyverno.ValueKyvernoApp,
					common.GeneratePolicyLabel:    "test-policy",
					common.GenerateSourceUIDLabel: string(source.GetUID()),
				})
				client := &downstreamListClient{downstream: *downstream}
				metadataCache := map[types.UID]Resource{}
				if tt.cachedUID != "" {
					cached := downstream.DeepCopy()
					cached.SetUID(tt.cachedUID)
					metadataCache[tt.cachedUID] = Resource{
						Name:      cached.GetName(),
						Namespace: cached.GetNamespace(),
						Labels:    cached.GetLabels(),
						Data:      cached,
					}
				}
				wm := &WatchManager{
					log:    logging.WithName("test"),
					client: client,
					dynamicWatchers: map[schema.GroupVersionResource]*watcher{
						gvr: {metadataCache: metadataCache},
					},
				}

				if operation == "update" {
					wm.handleUpdate(source, gvr)
				} else {
					wm.handleDelete(source, gvr)
				}

				if tt.cachedUID == downstream.GetUID() && operation == "update" {
					assert.Len(t, client.updated, 1)
				} else {
					assert.Empty(t, client.updated)
				}
				if tt.cachedUID == downstream.GetUID() && operation == "delete" {
					assert.Len(t, client.deleted, 1)
				} else {
					assert.Empty(t, client.deleted)
				}
				assert.Empty(t, client.created)
			})
		}
	}
}

// TestHandleUpdate_DriftRevert_PreservesResourceVersion is the regression test for the
// CustomResource sync bug: when a user tampers with a generated downstream, the revert must reuse
// the live object's resourceVersion rather than clearing it. Kubernetes accepts a blind (no
// resourceVersion) update for most built-in resources, but rejects it for CustomResources
// ("metadata.resourceVersion: must be specified for an update"), so clearing it silently broke sync
// for every CRD downstream (e.g. a Kai Scheduler Queue). The cached copy carries an older
// resourceVersion, so the fix uses the resourceVersion from the current watch event.
func TestHandleUpdate_DriftRevert_PreservesResourceVersion(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "scheduling.run.ai", Version: "v2", Resource: "queues"}
	labels := map[string]string{common.GeneratePolicyLabel: "generate-user-namespace-resources"}
	// The cached downstream (what sync restores to) carries the resourceVersion from when it was synced.
	cached := makeUnstructured("100", "scheduling.run.ai", "v2", "Queue", "alice", "", "queue-uid", labels)
	cachedHash := reportutils.CalculateResourceHash(*cached)

	var captured *unstructured.Unstructured
	mc := &MockClient{updateFn: func(obj interface{}) {
		if u, ok := obj.(*unstructured.Unstructured); ok {
			captured = u
		}
	}}
	wm := &WatchManager{
		log:    logging.WithName("test"),
		client: mc,
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			gvr: {metadataCache: map[types.UID]Resource{
				"queue-uid": {Name: "alice", Labels: labels, Hash: cachedHash, Data: cached},
			}},
		},
	}

	// The user tampers with the live downstream: same UID, a newer resourceVersion, and a spec change
	// (extra annotation) so the hash differs from the cache and a revert is triggered.
	live := cached.DeepCopy()
	live.SetResourceVersion("205")
	live.SetAnnotations(map[string]string{"tampered": "true"})
	wm.handleUpdate(live, gvr)

	require.NotNil(t, captured, "the revert must call UpdateResource")
	assert.Equal(t, "205", captured.GetResourceVersion(),
		"revert must use the live object's resourceVersion (CRD updates reject an empty one)")
}

// TestHandleUpdate_SourcePropagation_PreservesDownstreamResourceVersion is the regression test for the
// same bug on the source-change propagation path: when the source changes, each downstream is updated,
// and that update must carry the downstream's current resourceVersion. Otherwise propagation to a CRD
// downstream fails the same way the drift revert did.
func TestHandleUpdate_SourcePropagation_PreservesDownstreamResourceVersion(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "scheduling.run.ai", Version: "v2", Resource: "queues"}
	// The source object is not in the metadata cache, which routes handleUpdate to the propagation path.
	src := makeUnstructured("500", "scheduling.run.ai", "v2", "Queue", "source-queue", "", "src-uid", nil)
	src.SetAnnotations(map[string]string{"tier": "free"})
	// The current downstream, returned by the List, carries its own resourceVersion.
	ds := makeUnstructured("305", "scheduling.run.ai", "v2", "Queue", "alice", "", "ds-uid",
		map[string]string{common.GenerateSourceUIDLabel: "src-uid"})

	var captured *unstructured.Unstructured
	mc := &MockClient{
		updateFn:  func(obj interface{}) { captured, _ = obj.(*unstructured.Unstructured) },
		listItems: []unstructured.Unstructured{*ds},
	}
	wm := &WatchManager{
		log:    logging.WithName("test"),
		client: mc,
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			gvr: {metadataCache: map[types.UID]Resource{
				"ds-uid": {Name: "alice", Labels: ds.GetLabels(), Hash: "stale", Data: ds},
			}},
		},
	}

	wm.handleUpdate(src, gvr)

	require.NotNil(t, captured, "propagation must call UpdateResource on the downstream")
	assert.Equal(t, "305", captured.GetResourceVersion(),
		"propagation must set the downstream's current resourceVersion (CRD updates reject an empty one)")
}

// usageDownstream returns a generated Crossplane Usage the way Kyverno caches it: its own labels and
// spec, plus server-set metadata. A cache refreshed from the live object after a restart can also
// hold what another controller added, so it carries Crossplane's finalizer too.
func usageDownstream() *unstructured.Unstructured {
	labels := map[string]string{
		common.GeneratePolicyLabel:     "crossplane-usage-deletion-protection",
		common.GenerateTriggerUIDLabel: "xr-uid",
	}
	u := makeUnstructured("100", "protection.crossplane.io", "v1beta1", "Usage", "block-deletion-xr", "dev", "usage-uid", labels)
	_ = unstructured.SetNestedField(u.Object, "protected", "spec", "reason")
	_ = unstructured.SetNestedField(u.Object, "True", "status", "ready")
	u.SetFinalizers([]string{"usage.apiextensions.crossplane.io"})
	u.SetAnnotations(map[string]string{"crossplane.io/usage-details": "protected"})
	u.SetManagedFields([]metav1.ManagedFieldsEntry{
		managedFieldsEntry("kyverno-generate", metav1.ManagedFieldsOperationApply,
			`{"f:metadata":{"f:labels":{"f:generate.kyverno.io/policy-name":{},"f:generate.kyverno.io/trigger-uid":{}}},"f:spec":{"f:reason":{}}}`),
		managedFieldsEntry("crossplane", metav1.ManagedFieldsOperationUpdate,
			`{"f:metadata":{"f:annotations":{".":{},"f:crossplane.io/usage-details":{}},"f:finalizers":{"v:\"usage.apiextensions.crossplane.io\"":{}}}}`),
	})
	return u
}

func managedFieldsEntry(manager string, operation metav1.ManagedFieldsOperationType, fields string) metav1.ManagedFieldsEntry {
	return metav1.ManagedFieldsEntry{Manager: manager, Operation: operation, FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(fields)}}
}

func newRevertTestWatchManager(cached *unstructured.Unstructured, mc *MockClient, serverSideApply bool) (*WatchManager, schema.GroupVersionResource) {
	gvr := schema.GroupVersionResource{Group: "protection.crossplane.io", Version: "v1beta1", Resource: "usages"}
	return &WatchManager{
		log:    logging.WithName("test"),
		client: mc,
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			gvr: {metadataCache: map[types.UID]Resource{
				cached.GetUID(): {
					Name:            cached.GetName(),
					Namespace:       cached.GetNamespace(),
					Labels:          cached.GetLabels(),
					Hash:            reportutils.CalculateResourceHash(*cached),
					Data:            cached,
					ServerSideApply: serverSideApply,
				},
			}},
		},
	}, gvr
}

// tamperedUsage is the live Usage after Crossplane added its annotation and a user changed the
// generated spec, added a label and set an owner reference.
func tamperedUsage(cached *unstructured.Unstructured) *unstructured.Unstructured {
	live := cached.DeepCopy()
	live.SetResourceVersion("205")
	live.SetAnnotations(map[string]string{"crossplane.io/usage-details": "protected"})
	_ = unstructured.SetNestedField(live.Object, "hacked", "spec", "reason")
	labels := live.GetLabels()
	labels["team"] = "payments"
	live.SetLabels(labels)
	live.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "v1", Kind: "ConfigMap", Name: "owner", UID: "owner-uid"}})
	return live
}

// TestRevertDownstream_RestoresExactlyWhatWasGenerated pins the default revert: everything the
// drift check covers (the spec, labels and annotations) goes back to exactly what the policy
// generated, so a label or annotation someone added is removed again. Finalizers and owner
// references are outside the drift check and belong to whoever set them, so the revert keeps the
// live ones instead of dropping them, and it carries the live resourceVersion.
func TestRevertDownstream_RestoresExactlyWhatWasGenerated(t *testing.T) {
	// a freshly generated Usage, before Crossplane added anything
	cached := usageDownstream()
	cached.SetFinalizers(nil)
	cached.SetAnnotations(nil)
	var captured *unstructured.Unstructured
	mc := &MockClient{updateFn: func(obj interface{}) { captured, _ = obj.(*unstructured.Unstructured) }}
	wm, gvr := newRevertTestWatchManager(cached, mc, false)

	live := tamperedUsage(cached)
	live.SetFinalizers([]string{"usage.apiextensions.crossplane.io"})
	wm.handleUpdate(live, gvr)

	require.NotNil(t, captured, "the revert must update the downstream")
	assert.Empty(t, mc.applied, "the default revert does not use server-side apply")
	reason, _, _ := unstructured.NestedString(captured.Object, "spec", "reason")
	assert.Equal(t, "protected", reason, "the generated spec is restored")
	assert.NotContains(t, captured.GetLabels(), "team", "a label someone added is removed")
	assert.Empty(t, captured.GetAnnotations(), "an annotation someone added is removed")
	assert.Equal(t, live.GetFinalizers(), captured.GetFinalizers(), "the live finalizers are kept")
	assert.Equal(t, live.GetOwnerReferences(), captured.GetOwnerReferences(), "the live owner references are kept")
	assert.Equal(t, "205", captured.GetResourceVersion(), "the update carries the live resourceVersion")
}

// TestRevertDownstream_ServerSideApplyLeavesOtherOwnersAlone covers a policy with
// useServerSideApply. The revert applies what the policy generated as the generate field manager,
// so it only takes back the fields the policy set and leaves what other controllers own alone,
// which is what keeps it from fighting a controller like Crossplane over its annotation. The apply
// configuration must not carry server-set metadata, status, finalizers or owner references: owning
// a finalizer would make Kyverno put it back while its controller tries to remove it.
func TestRevertDownstream_ServerSideApplyLeavesOtherOwnersAlone(t *testing.T) {
	cached := usageDownstream()
	var applied *unstructured.Unstructured
	var manager string
	mc := &MockClient{applyFn: func(fieldManager string, obj interface{}) {
		manager = fieldManager
		applied, _ = obj.(*unstructured.Unstructured)
	}}
	wm, gvr := newRevertTestWatchManager(cached, mc, true)

	wm.handleUpdate(tamperedUsage(cached), gvr)

	assert.Empty(t, mc.updated, "a policy using server-side apply must not revert with a full update")
	require.NotNil(t, applied, "the revert must apply the generated state")
	assert.Equal(t, "generate", manager, "the same field manager that generated the resource")
	reason, _, _ := unstructured.NestedString(applied.Object, "spec", "reason")
	assert.Equal(t, "protected", reason, "the generated spec is applied")
	assert.Equal(t, cached.GetLabels(), applied.GetLabels(), "the labels the policy applied")
	assert.Empty(t, applied.GetAnnotations(), "Crossplane's annotation is in the cached copy but Kyverno never applied it")
	assert.Equal(t, "block-deletion-xr", applied.GetName())
	assert.Equal(t, "dev", applied.GetNamespace())
	assert.Empty(t, applied.GetFinalizers(), "finalizers are not part of the apply configuration")
	assert.Empty(t, applied.GetOwnerReferences(), "owner references are not part of the apply configuration")
	assert.Empty(t, applied.GetResourceVersion())
	assert.Empty(t, applied.GetUID())
	assert.Empty(t, applied.GetManagedFields())
	_, hasStatus := applied.Object["status"]
	assert.False(t, hasStatus, "status is not part of the apply configuration")
}

// TestHandleUpdate_SourcePropagation_ServerSideApply covers the other write sync makes: when the
// source changes, a downstream of a policy with useServerSideApply is updated by applying the new
// content rather than replacing the object.
func TestHandleUpdate_SourcePropagation_ServerSideApply(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "scheduling.run.ai", Version: "v2", Resource: "queues"}
	src := makeUnstructured("500", "scheduling.run.ai", "v2", "Queue", "source-queue", "", "src-uid", nil)
	src.SetAnnotations(map[string]string{"tier": "free"})
	ds := makeUnstructured("305", "scheduling.run.ai", "v2", "Queue", "alice", "", "ds-uid",
		map[string]string{common.GenerateSourceUIDLabel: "src-uid"})
	// an owner reference Kyverno applied to the downstream, and a finalizer another controller added
	owner := metav1.OwnerReference{APIVersion: "v1", Kind: "Namespace", Name: "alice", UID: "ns-uid"}
	ds.SetOwnerReferences([]metav1.OwnerReference{owner})
	ds.SetFinalizers([]string{"example.com/other-controller"})
	ds.SetLabels(map[string]string{common.GenerateSourceUIDLabel: "src-uid", "team": "payments"})
	ds.SetManagedFields([]metav1.ManagedFieldsEntry{
		managedFieldsEntry("kyverno-generate", metav1.ManagedFieldsOperationApply,
			`{"f:metadata":{"f:labels":{"f:generate.kyverno.io/source-uid":{}},"f:ownerReferences":{"k:{\"uid\":\"ns-uid\"}":{}}}}`),
	})

	// what the server returns: the propagated content, with another controller's annotation kept
	returned := ds.DeepCopy()
	returned.SetAnnotations(map[string]string{"tier": "free", "example.com/other": "x"})

	var applied *unstructured.Unstructured
	mc := &MockClient{
		applyFn:      func(_ string, obj interface{}) { applied, _ = obj.(*unstructured.Unstructured) },
		applyReturns: returned,
		listItems:    []unstructured.Unstructured{*ds},
	}
	wm := &WatchManager{
		log:    logging.WithName("test"),
		client: mc,
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			gvr: {metadataCache: map[types.UID]Resource{
				"ds-uid": {Name: "alice", Labels: ds.GetLabels(), Hash: "stale", Data: ds, ServerSideApply: true},
			}},
		},
	}

	wm.handleUpdate(src, gvr)

	assert.Empty(t, mc.updated, "propagation for a server-side apply policy must not replace the object")
	require.NotNil(t, applied, "propagation must apply the new content")
	assert.Equal(t, "alice", applied.GetName())
	assert.Equal(t, "free", applied.GetAnnotations()["tier"])
	assert.Equal(t, []metav1.OwnerReference{owner}, applied.GetOwnerReferences(), "the owner reference Kyverno applied is kept")
	assert.Empty(t, applied.GetFinalizers(), "another controller's finalizer is not taken over")
	assert.Equal(t, map[string]string{common.GenerateSourceUIDLabel: "src-uid"}, applied.GetLabels(), "another controller's label is not taken over")
	entry := wm.dynamicWatchers[gvr].metadataCache["ds-uid"]
	assert.Equal(t, reportutils.CalculateResourceHash(*returned), entry.Hash, "the cache tracks the object the apply returned")
	assert.Equal(t, "free", entry.Data.GetAnnotations()["tier"], "the desired content is the propagated source content")
	assert.NotContains(t, entry.Data.GetAnnotations(), "example.com/other")
	assert.Empty(t, applied.GetResourceVersion())
	assert.Empty(t, applied.GetUID())
}

// TestSyncWatchers_RecordsServerSideApply checks the setting reaches the cache entries the reverts
// read it from.
func TestSyncWatchers_RecordsServerSideApply(t *testing.T) {
	queues := schema.GroupVersionResource{Group: "scheduling.run.ai", Version: "v2", Resource: "queues"}
	wm := &WatchManager{
		log:    logging.WithName("test"),
		client: &MockClient{},
		restMapper: &mockRESTMapper{fn: func(_ schema.GroupKind, _ string) (*meta.RESTMapping, error) {
			return &meta.RESTMapping{Resource: queues}, nil
		}},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			queues: {
				watcher:       watch.MockWatcher{StopFunc: func() {}, ResultChanFunc: func() <-chan watch.Event { return nil }},
				metadataCache: map[types.UID]Resource{},
			},
		},
		policyRefs: map[string][]schema.GroupVersionResource{},
		refCount:   map[schema.GroupVersionResource]int{},
	}
	queue := makeUnstructured("1", "scheduling.run.ai", "v2", "Queue", "alice", "", "q-uid",
		map[string]string{common.GeneratePolicyLabel: "pol"})

	require.NoError(t, wm.SyncWatchers("pol", nil, []*unstructured.Unstructured{queue}, true))

	assert.True(t, wm.dynamicWatchers[queues].metadataCache["q-uid"].ServerSideApply)
}

// TestRevertDownstream_ServerSideApplyKeepsWhatThePolicySet covers owner references and finalizers
// the policy itself generates. Server-side apply removes a field its manager applied before and
// leaves out now, so the revert must keep applying them, while still leaving out the finalizer
// Crossplane added.
func TestRevertDownstream_ServerSideApplyKeepsWhatThePolicySet(t *testing.T) {
	cached := usageDownstream()
	owner := metav1.OwnerReference{APIVersion: "v1", Kind: "ConfigMap", Name: "owner", UID: "11111111-2222-3333-4444-555555555555"}
	cached.SetOwnerReferences([]metav1.OwnerReference{owner})
	cached.SetFinalizers([]string{"example.com/policy-finalizer", "usage.apiextensions.crossplane.io"})
	cached.SetManagedFields([]metav1.ManagedFieldsEntry{
		managedFieldsEntry("kyverno-generate", metav1.ManagedFieldsOperationApply,
			`{"f:metadata":{"f:finalizers":{"v:\"example.com/policy-finalizer\"":{}},"f:labels":{"f:generate.kyverno.io/policy-name":{}},"f:ownerReferences":{"k:{\"uid\":\"11111111-2222-3333-4444-555555555555\"}":{}}},"f:spec":{"f:reason":{}}}`),
		managedFieldsEntry("crossplane", metav1.ManagedFieldsOperationUpdate,
			`{"f:metadata":{"f:annotations":{"f:crossplane.io/usage-details":{}},"f:finalizers":{"v:\"usage.apiextensions.crossplane.io\"":{}}}}`),
	})
	var applied *unstructured.Unstructured
	mc := &MockClient{applyFn: func(_ string, obj interface{}) { applied, _ = obj.(*unstructured.Unstructured) }}
	wm, gvr := newRevertTestWatchManager(cached, mc, true)

	wm.handleUpdate(tamperedUsage(cached), gvr)

	require.NotNil(t, applied)
	assert.Equal(t, []metav1.OwnerReference{owner}, applied.GetOwnerReferences(), "the owner reference the policy set is kept")
	assert.Equal(t, []string{"example.com/policy-finalizer"}, applied.GetFinalizers(), "only the finalizer the policy set")
	assert.Equal(t, map[string]string{common.GeneratePolicyLabel: "crossplane-usage-deletion-protection"}, applied.GetLabels(),
		"only the labels the policy applied")
	assert.Empty(t, applied.GetAnnotations())
}

// TestRevertDownstream_ServerSideApplyWithoutApplyRecord covers a downstream with no record of a
// server-side apply by Kyverno, e.g. one created before the policy switched to server-side apply.
// Kyverno owns no finalizers or owner references on it, so it applies none, and applying none
// removes nothing.
func TestRevertDownstream_ServerSideApplyWithoutApplyRecord(t *testing.T) {
	cached := usageDownstream()
	cached.SetManagedFields(nil)
	cached.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "v1", Kind: "ConfigMap", Name: "owner", UID: "owner-uid"}})
	kyvernoLabels := cached.GetLabels()
	labels := cached.GetLabels()
	labels["team"] = "payments"
	cached.SetLabels(labels)
	var applied *unstructured.Unstructured
	mc := &MockClient{applyFn: func(_ string, obj interface{}) { applied, _ = obj.(*unstructured.Unstructured) }}
	wm, gvr := newRevertTestWatchManager(cached, mc, true)

	wm.handleUpdate(tamperedUsage(cached), gvr)

	require.NotNil(t, applied)
	assert.Equal(t, kyvernoLabels, applied.GetLabels(), "only the labels Kyverno puts on every generated resource")
	assert.Empty(t, applied.GetAnnotations(), "annotations in the cached copy may belong to another controller")
	assert.Empty(t, applied.GetOwnerReferences())
	assert.Empty(t, applied.GetFinalizers())
}

// TestRevertDownstream_ServerSideApplyAfterPropagation covers a revert after a source change was
// propagated. The cache then holds the propagated content, which has no managedFields, while the
// live object still records what Kyverno applied. The revert must read ownership from the live
// object, so it keeps the finalizer Kyverno applied and leaves another team's label alone.
func TestRevertDownstream_ServerSideApplyAfterPropagation(t *testing.T) {
	cached := usageDownstream()
	cached.SetManagedFields(nil)
	cached.SetFinalizers(nil)
	cached.SetAnnotations(nil)
	labels := cached.GetLabels()
	labels["team"] = "other-team"
	cached.SetLabels(labels)

	live := cached.DeepCopy()
	live.SetResourceVersion("205")
	_ = unstructured.SetNestedField(live.Object, "hacked", "spec", "reason")
	live.SetFinalizers([]string{"example.com/policy-finalizer", "example.com/other-controller"})
	live.SetManagedFields([]metav1.ManagedFieldsEntry{
		managedFieldsEntry("kyverno-generate", metav1.ManagedFieldsOperationApply,
			`{"f:metadata":{"f:finalizers":{"v:\"example.com/policy-finalizer\"":{}},"f:labels":{"f:generate.kyverno.io/policy-name":{},"f:generate.kyverno.io/trigger-uid":{}}},"f:spec":{"f:reason":{}}}`),
		managedFieldsEntry("other-team", metav1.ManagedFieldsOperationUpdate,
			`{"f:metadata":{"f:finalizers":{"v:\"example.com/other-controller\"":{}},"f:labels":{"f:team":{}}}}`),
	})
	var applied *unstructured.Unstructured
	mc := &MockClient{applyFn: func(_ string, obj interface{}) { applied, _ = obj.(*unstructured.Unstructured) }}
	wm, gvr := newRevertTestWatchManager(cached, mc, true)

	wm.handleUpdate(live, gvr)

	require.NotNil(t, applied)
	reason, _, _ := unstructured.NestedString(applied.Object, "spec", "reason")
	assert.Equal(t, "protected", reason)
	assert.Equal(t, []string{"example.com/policy-finalizer"}, applied.GetFinalizers(), "the finalizer Kyverno applied is kept")
	assert.NotContains(t, applied.GetLabels(), "team", "another team's label is not taken over")
	assert.Equal(t, "crossplane-usage-deletion-protection", applied.GetLabels()[common.GeneratePolicyLabel])
}

// TestHandleDelete_ServerSideApplyRecreatesWithApply covers a deleted downstream of a policy using
// server-side apply: it is recreated by applying what Kyverno generated, so the new object carries
// Kyverno's apply record like the original, and the cache follows it to its new UID.
func TestHandleDelete_ServerSideApplyRecreatesWithApply(t *testing.T) {
	cached := usageDownstream()
	cached.SetLabels(map[string]string{
		kyverno.LabelAppManagedBy:      kyverno.ValueKyvernoApp,
		common.GeneratePolicyLabel:     "crossplane-usage-deletion-protection",
		common.GenerateTriggerUIDLabel: "xr-uid",
	})
	cached.SetManagedFields([]metav1.ManagedFieldsEntry{
		managedFieldsEntry("kyverno-generate", metav1.ManagedFieldsOperationApply,
			`{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{},"f:generate.kyverno.io/policy-name":{},"f:generate.kyverno.io/trigger-uid":{}}},"f:spec":{"f:reason":{}}}`),
	})
	recreated := cached.DeepCopy()
	recreated.SetUID("usage-uid-2")
	var applied *unstructured.Unstructured
	mc := &MockClient{
		applyFn:      func(_ string, obj interface{}) { applied, _ = obj.(*unstructured.Unstructured) },
		applyReturns: recreated,
	}
	wm, gvr := newRevertTestWatchManager(cached, mc, true)

	wm.handleDelete(cached.DeepCopy(), gvr)

	assert.Empty(t, mc.created, "a server-side apply policy recreates with apply")
	require.NotNil(t, applied)
	reason, _, _ := unstructured.NestedString(applied.Object, "spec", "reason")
	assert.Equal(t, "protected", reason)
	assert.Empty(t, applied.GetAnnotations(), "Crossplane's annotation in the cached copy is not applied")
	assert.Empty(t, applied.GetFinalizers(), "Crossplane's finalizer in the cached copy is not applied")
	_, stillOld := wm.dynamicWatchers[gvr].metadataCache[cached.GetUID()]
	_, followsNew := wm.dynamicWatchers[gvr].metadataCache["usage-uid-2"]
	assert.False(t, stillOld)
	assert.True(t, followsNew, "the cache entry moves to the recreated object's UID")
}

// TestRevertDownstream_ServerSideApplyRefreshesCachedHash covers what happens after an apply that
// left another controller's metadata in place: the cache has to track the object as the server
// returned it, otherwise that metadata keeps looking like drift and every later event applies
// again. The desired content stays what the policy generated.
func TestRevertDownstream_ServerSideApplyRefreshesCachedHash(t *testing.T) {
	cached := usageDownstream()
	cached.SetFinalizers(nil)
	cached.SetAnnotations(nil)
	live := cached.DeepCopy()
	live.SetResourceVersion("205")
	live.SetFinalizers([]string{"usage.apiextensions.crossplane.io"})
	live.SetAnnotations(map[string]string{"crossplane.io/usage-details": "protected"})
	mc := &MockClient{applyReturns: live}
	wm, gvr := newRevertTestWatchManager(cached, mc, true)

	wm.handleUpdate(live, gvr)
	require.Len(t, mc.applied, 1, "Crossplane's annotation differs from the cached hash, so the first event applies")
	entry := wm.dynamicWatchers[gvr].metadataCache[cached.GetUID()]
	assert.Equal(t, reportutils.CalculateResourceHash(*live), entry.Hash, "the cache tracks the object the apply returned")
	assert.Empty(t, entry.Data.GetAnnotations(), "the desired content is still what the policy generated")

	wm.handleUpdate(live, gvr)
	assert.Len(t, mc.applied, 1, "the same object again is not drift")
}

// TestAppliedByGenerate reads the apply record only from a server-side apply entry of the
// generate field manager on the main resource.
func TestAppliedByGenerate(t *testing.T) {
	owned := `{"f:metadata":{"f:labels":{"f:team":{}},"f:annotations":{"f:note":{}},"f:finalizers":{"v:\"example.com/f\"":{}},"f:ownerReferences":{"k:{\"uid\":\"u1\"}":{}}}}`
	tests := []struct {
		name    string
		entries []metav1.ManagedFieldsEntry
		wantOK  bool
	}{
		{name: "apply entry of the generate manager", entries: []metav1.ManagedFieldsEntry{managedFieldsEntry("kyverno-generate", metav1.ManagedFieldsOperationApply, owned)}, wantOK: true},
		{name: "update entry of the generate manager is ignored", entries: []metav1.ManagedFieldsEntry{managedFieldsEntry("kyverno-generate", metav1.ManagedFieldsOperationUpdate, owned)}},
		{name: "entry of another manager is ignored", entries: []metav1.ManagedFieldsEntry{managedFieldsEntry("crossplane", metav1.ManagedFieldsOperationApply, owned)}},
		{name: "status subresource entry is ignored", entries: func() []metav1.ManagedFieldsEntry {
			e := managedFieldsEntry("kyverno-generate", metav1.ManagedFieldsOperationApply, owned)
			e.Subresource = "status"
			return []metav1.ManagedFieldsEntry{e}
		}()},
		{name: "malformed fields are no record", entries: []metav1.ManagedFieldsEntry{managedFieldsEntry("kyverno-generate", metav1.ManagedFieldsOperationApply, `{"f:metadata":`)}},
		{name: "no managed fields", entries: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := makeUnstructured("1", "", "v1", "ConfigMap", "cm", "ns", "uid", map[string]string{"team": "a"})
			obj.SetManagedFields(tt.entries)
			applied, ok := appliedByGenerate(obj)
			assert.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				return
			}
			assert.True(t, applied.labels["team"])
			assert.True(t, applied.annotations["note"])
			assert.True(t, applied.finalizers["example.com/f"])
			assert.True(t, applied.ownerUIDs["u1"])
		})
	}
}

// TestRevertDownstream_ServerSideApplyErrorKeepsCache covers a failed apply: the cache must stay
// as it was, so the next event still sees the drift and retries the revert.
func TestRevertDownstream_ServerSideApplyErrorKeepsCache(t *testing.T) {
	cached := usageDownstream()
	// what a successful apply would return, so a refresh on failure would show in the hash
	mc := &MockClient{err: errors.New("apply failed"), applyReturns: tamperedUsage(cached)}
	wm, gvr := newRevertTestWatchManager(cached, mc, true)
	before := wm.dynamicWatchers[gvr].metadataCache[cached.GetUID()]

	wm.handleUpdate(tamperedUsage(cached), gvr)

	require.Len(t, mc.applied, 1, "the revert was attempted")
	assert.Equal(t, before, wm.dynamicWatchers[gvr].metadataCache[cached.GetUID()])
}

// TestHandleUpdate_SourcePropagationErrorKeepsCache covers a failed propagation: the downstream's
// cache entry must not be replaced with content that was never written.
func TestHandleUpdate_SourcePropagationErrorKeepsCache(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "scheduling.run.ai", Version: "v2", Resource: "queues"}
	src := makeUnstructured("500", "scheduling.run.ai", "v2", "Queue", "source-queue", "", "src-uid", nil)
	src.SetAnnotations(map[string]string{"tier": "free"})
	ds := makeUnstructured("305", "scheduling.run.ai", "v2", "Queue", "alice", "", "ds-uid",
		map[string]string{common.GenerateSourceUIDLabel: "src-uid"})
	entry := Resource{Name: "alice", Labels: ds.GetLabels(), Hash: "before", Data: ds, ServerSideApply: true}
	mc := &MockClient{listItems: []unstructured.Unstructured{*ds}}
	mc.applyFn = func(string, interface{}) { mc.err = errors.New("apply failed") }
	wm := &WatchManager{
		log:    logging.WithName("test"),
		client: mc,
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			gvr: {metadataCache: map[types.UID]Resource{"ds-uid": entry}},
		},
	}

	wm.handleUpdate(src, gvr)

	require.Len(t, mc.applied, 1)
	assert.Equal(t, entry, wm.dynamicWatchers[gvr].metadataCache["ds-uid"])
}

func TestPolicyMatches(t *testing.T) {
	labelsWith := func(namespace *string) map[string]string {
		labels := map[string]string{common.GeneratePolicyLabel: "pol"}
		if namespace != nil {
			labels[common.GeneratePolicyNamespaceLabel] = *namespace
		}
		return labels
	}
	tests := []struct {
		name      string
		labels    map[string]string
		policyKey string
		want      bool
	}{
		{name: "namespaced key, same namespace", labels: labelsWith(ptr.To("tenant-ns")), policyKey: "tenant-ns/pol", want: true},
		{name: "namespaced key, other namespace", labels: labelsWith(ptr.To("other-ns")), policyKey: "tenant-ns/pol", want: false},
		{name: "namespaced key, cluster label", labels: labelsWith(ptr.To("")), policyKey: "tenant-ns/pol", want: false},
		{name: "namespaced key, missing label", labels: labelsWith(nil), policyKey: "tenant-ns/pol", want: false},
		{name: "cluster key, cluster label", labels: labelsWith(ptr.To("")), policyKey: "pol", want: true},
		{name: "cluster key, missing label", labels: labelsWith(nil), policyKey: "pol", want: true},
		{name: "cluster key, namespaced label", labels: labelsWith(ptr.To("tenant-ns")), policyKey: "pol", want: false},
		{name: "other name", labels: labelsWith(ptr.To("tenant-ns")), policyKey: "tenant-ns/other", want: false},
		{name: "invalid key", labels: labelsWith(ptr.To("tenant-ns")), policyKey: "a/b/pol", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, policyMatches(tt.labels, tt.policyKey))
		})
	}
}

// A NamespacedGeneratingPolicy is keyed namespace/name in the watch manager.
// Its downstreams must be found and deleted under that key, without touching
// the downstream of a same-named policy in another namespace or at cluster
// scope.
func TestWatchManager_NamespacedPolicyKeyMatchesDownstream(t *testing.T) {
	secretGVR := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	trigger := &kyverno.ResourceSpec{UID: "uid-trigger"}
	downstream := func(name, ns, policyNamespace string) *unstructured.Unstructured {
		return makeUnstructured("1", "", "v1", "Secret", name, ns, types.UID("uid-"+name), map[string]string{
			kyverno.LabelAppManagedBy:           kyverno.ValueKyvernoApp,
			common.GeneratePolicyLabel:          "pol",
			common.GeneratePolicyNamespaceLabel: policyNamespace,
			common.GenerateTriggerUIDLabel:      "uid-trigger",
		})
	}
	tenant := downstream("tenant", "tenant-ns", "tenant-ns")
	other := downstream("other", "other-ns", "other-ns")
	cluster := downstream("cluster", "tenant-ns", "")

	client := &MockClient{}
	wm := &WatchManager{
		log:    logging.WithName("test"),
		client: client,
		restMapper: &mockRESTMapper{fn: func(schema.GroupKind, string) (*meta.RESTMapping, error) {
			return &meta.RESTMapping{Resource: secretGVR}, nil
		}},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			// pre-seeded so SyncWatchers reuses it instead of starting a real watcher
			secretGVR: {watcher: watch.NewFake(), metadataCache: map[types.UID]Resource{}},
		},
		policyRefs: map[string][]schema.GroupVersionResource{},
		refCount:   map[schema.GroupVersionResource]int{},
	}
	require.NoError(t, wm.SyncWatchers("tenant-ns/pol", trigger, []*unstructured.Unstructured{tenant}, false))
	require.NoError(t, wm.SyncWatchers("other-ns/pol", trigger, []*unstructured.Unstructured{other}, false))
	require.NoError(t, wm.SyncWatchers("pol", trigger, []*unstructured.Unstructured{cluster}, false))

	names := func(objs []*unstructured.Unstructured) []string {
		var out []string
		for _, obj := range objs {
			out = append(out, obj.GetName())
		}
		return out
	}
	assert.Equal(t, []string{"tenant"}, names(wm.GetDownstreams("tenant-ns/pol")))
	assert.Equal(t, []string{"other"}, names(wm.GetDownstreams("other-ns/pol")))
	assert.Equal(t, []string{"cluster"}, names(wm.GetDownstreams("pol")))

	wm.DeleteDownstreams("tenant-ns/pol", trigger)
	assert.Equal(t, []string{"Secret/tenant-ns/tenant"}, client.deleted)
	assert.ElementsMatch(t, []string{"other", "cluster"}, names(append(wm.GetDownstreams("other-ns/pol"), wm.GetDownstreams("pol")...)))
}
