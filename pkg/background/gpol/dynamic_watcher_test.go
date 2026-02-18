package gpol

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/logging"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/assert"
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
	err      error
	deleteFn func(ctx context.Context, apiVersion, kind, namespace, name string, dryRun bool, options metav1.DeleteOptions) error
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
	return nil, nil
}
func (m *MockClient) PatchResource(ctx context.Context, apiVersion string, kind string, namespace, name string, path []byte) (*unstructured.Unstructured, error) {
	return nil, nil
}
func (m *MockClient) ListResource(ctx context.Context, apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	if m.err != nil {
		return nil, m.err
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
	return nil, nil
}
func (m *MockClient) UpdateResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool, subresource ...string) (*unstructured.Unstructured, error) {
	if m.err != nil {
		return nil, m.err
	}
	return makeUnstructured("", "", "", "", "", "", "", nil), nil
}
func (m *MockClient) UpdateStatusResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	return nil, nil
}
func (m *MockClient) ApplyResource(ctx context.Context, apiVersion string, kind string, namespace, name string, obj interface{}, dryRun bool, fieldManager string, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, nil
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
				existing := &watcher{metadataCache: map[types.UID]Resource{}}
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
			err := wm.SyncWatchers(tc.polName, tc.generatedResources)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
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
		trigger        *v1.ResourceSpec
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
			trigger:    &v1.ResourceSpec{UID: triggerUID},
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
			trigger:    &v1.ResourceSpec{UID: triggerUID},
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
