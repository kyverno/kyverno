package gpol

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/logging"
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
	return nil, nil
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
	return nil, nil
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
	gvr = schema.GroupVersionResource{Group: "g", Version: "v1", Resource: "res"}
)

func makeUnstructured(group, version, kind, name, ns string, uid types.UID, labels map[string]string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: version, Kind: kind})
	u.SetName(name)
	u.SetNamespace(ns)
	u.SetUID(uid)
	u.SetLabels(labels)
	return u
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
			generatedResources: []*unstructured.Unstructured{makeUnstructured("g", "v1", "Kind", "n", "ns", "uid1", nil)},
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
			generatedResources: []*unstructured.Unstructured{makeUnstructured("g", "v1", "Kind", "n", "ns", "uid1", nil)},
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
			generatedResources: []*unstructured.Unstructured{makeUnstructured("g", "v1", "Kind", "n", "ns", "uid1", nil)},
			wantErr:            true,
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
	// mock watcher with metadata cache
	type mockWatcher struct {
		metadataCache map[types.UID]Resource
	}

	// helper to create CachedResource
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
					"uid1": makeCached(makeUnstructured("apps", "v1", "ConfigMap", "res1", "ns1", "uid1", map[string]string{"foo": "bar"})),
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
					"uid1": makeCached(makeUnstructured("apps", "v1", "Deployment", "res1", "ns1", "uid1", map[string]string{common.GeneratePolicyLabel: "p1"})),
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
					"uid1": makeCached(makeUnstructured("apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{common.GeneratePolicyLabel: "p1"})),
					"uid2": makeCached(makeUnstructured("apps", "v1", "Service", "res2", "ns1", "uid2", map[string]string{common.GeneratePolicyLabel: "p1"})),
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
					"uid1": makeCached(makeUnstructured("apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{common.GeneratePolicyLabel: "p1"})),
				}},
				gvr2: {metadataCache: map[types.UID]Resource{
					"uid2": makeCached(makeUnstructured("apps", "v1", "ConfigMap", "res2", "ns1", "uid1", map[string]string{common.GeneratePolicyLabel: "p1"})),
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
					"uid1": makeCached(makeUnstructured("apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{"foo": "bar"})),
					"uid2": makeCached(makeUnstructured("apps", "v1", "Service", "res2", "ns1", "uid1", map[string]string{common.GeneratePolicyLabel: "p1"})),
				}},
			},
			wantKinds: []string{"Service"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert mockWatcher to actual type
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
					"uid1": makeCached(makeUnstructured("apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{"foo": "bar"}), map[string]string{"foo": "bar"}),
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
					"uid1": makeCached(makeUnstructured("apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{
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
					"uid1": makeCached(makeUnstructured("apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{
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
					"uid1": makeCached(makeUnstructured("apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{
						common.GeneratePolicyLabel:     "p1",
						common.GenerateTriggerUIDLabel: string(triggerUID),
					}), map[string]string{
						common.GeneratePolicyLabel:     "p1",
						common.GenerateTriggerUIDLabel: string(triggerUID),
					}),
					"uid2": makeCached(makeUnstructured("apps", "v1", "Service", "res2", "ns1", "uid2", map[string]string{
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
					"uid1": makeCached(makeUnstructured("apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{
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

// func TestRemoveWatchersForPolicy(t *testing.T) {
// 	makeCached := func(obj *unstructured.Unstructured, labels map[string]string) Resource {
// 		obj.SetLabels(labels)
// 		return Resource{
// 			Name:      obj.GetName(),
// 			Namespace: obj.GetNamespace(),
// 			Labels:    labels,
// 			Data:      obj,
// 		}
// 	}

// 	gvr1 := schema.GroupVersionResource{Group: "g", Version: "v1", Resource: "res1"}
// 	gvr2 := schema.GroupVersionResource{Group: "g", Version: "v1", Resource: "res2"}

// 	tests := []struct {
// 		name             string
// 		policyName       string
// 		policyRefs       map[string][]schema.GroupVersionResource
// 		dynamicW         map[schema.GroupVersionResource]*watcher
// 		refCount         map[schema.GroupVersionResource]int
// 		deleteDownstream bool
// 		clientErr        error
// 		wantDeleted      []string
// 		wantCacheSizes   map[schema.GroupVersionResource]int
// 		wantStopped      map[schema.GroupVersionResource]bool
// 		wantPolicyGone   bool
// 	}{
// 		{
// 			name:           "no watchers for policy",
// 			policyName:     "p1",
// 			policyRefs:     map[string][]schema.GroupVersionResource{},
// 			dynamicW:       map[schema.GroupVersionResource]*watcher{},
// 			refCount:       map[schema.GroupVersionResource]int{},
// 			wantDeleted:    nil,
// 			wantCacheSizes: map[schema.GroupVersionResource]int{},
// 			wantPolicyGone: false,
// 		},
// 		{
// 			name:       "watcher exists, no matching resources, stop watcher",
// 			policyName: "p1",
// 			policyRefs: map[string][]schema.GroupVersionResource{"p1": {gvr1}},
// 			dynamicW: map[schema.GroupVersionResource]*watcher{
// 				gvr1: {metadataCache: map[types.UID]Resource{}},
// 			},
// 			refCount:       map[schema.GroupVersionResource]int{gvr1: 1},
// 			wantDeleted:    nil,
// 			wantCacheSizes: map[schema.GroupVersionResource]int{},
// 			wantStopped:    map[schema.GroupVersionResource]bool{gvr1: true},
// 			wantPolicyGone: true,
// 		},
// 		{
// 			name:             "matching resources, deleteDownstream=false",
// 			policyName:       "p1",
// 			deleteDownstream: false,
// 			policyRefs:       map[string][]schema.GroupVersionResource{"p1": {gvr1}},
// 			dynamicW: map[schema.GroupVersionResource]*watcher{
// 				gvr1: {
// 					metadataCache: map[types.UID]Resource{
// 						"u1": makeCached(makeUnstructured("apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{
// 							common.GeneratePolicyLabel: "p1",
// 						}), map[string]string{
// 							common.GeneratePolicyLabel: "p1",
// 						}),
// 					},
// 				},
// 			},
// 			refCount:       map[schema.GroupVersionResource]int{gvr1: 1},
// 			wantDeleted:    nil,
// 			wantCacheSizes: map[schema.GroupVersionResource]int{},
// 			wantStopped:    map[schema.GroupVersionResource]bool{gvr1: true},
// 			wantPolicyGone: true,
// 		},
// 		{
// 			name:             "matching resources, deleteDownstream=true, no source UID label",
// 			policyName:       "p1",
// 			deleteDownstream: true,
// 			policyRefs:       map[string][]schema.GroupVersionResource{"p1": {gvr1}},
// 			dynamicW: map[schema.GroupVersionResource]*watcher{
// 				gvr1: {
// 					metadataCache: map[types.UID]Resource{
// 						"u1": makeCached(makeUnstructured("apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{
// 							common.GeneratePolicyLabel: "p1",
// 						}), map[string]string{
// 							common.GeneratePolicyLabel: "p1",
// 						}),
// 					},
// 				},
// 			},
// 			refCount:       map[schema.GroupVersionResource]int{gvr1: 1},
// 			wantDeleted:    []string{"Pod/ns1/res1"},
// 			wantCacheSizes: map[schema.GroupVersionResource]int{},
// 			wantStopped:    map[schema.GroupVersionResource]bool{gvr1: true},
// 			wantPolicyGone: true,
// 		},
// 		{
// 			name:             "matching resources, deleteDownstream=true, has source UID label",
// 			policyName:       "p1",
// 			deleteDownstream: true,
// 			policyRefs:       map[string][]schema.GroupVersionResource{"p1": {gvr1}},
// 			dynamicW: map[schema.GroupVersionResource]*watcher{
// 				gvr1: {
// 					metadataCache: map[types.UID]Resource{
// 						"u1": makeCached(makeUnstructured("apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{
// 							common.GeneratePolicyLabel:    "p1",
// 							common.GenerateSourceUIDLabel: "src-uid",
// 						}), map[string]string{
// 							common.GeneratePolicyLabel:    "p1",
// 							common.GenerateSourceUIDLabel: "src-uid",
// 						}),
// 					},
// 				},
// 			},
// 			refCount:       map[schema.GroupVersionResource]int{gvr1: 1},
// 			wantDeleted:    nil,
// 			wantCacheSizes: map[schema.GroupVersionResource]int{},
// 			wantStopped:    map[schema.GroupVersionResource]bool{gvr1: true},
// 			wantPolicyGone: true,
// 		},
// 		{
// 			name:             "delete error still removes from cache",
// 			policyName:       "p1",
// 			deleteDownstream: true,
// 			clientErr:        fmt.Errorf("delete failed"),
// 			policyRefs:       map[string][]schema.GroupVersionResource{"p1": {gvr1}},
// 			dynamicW: map[schema.GroupVersionResource]*watcher{
// 				gvr1: {
// 					metadataCache: map[types.UID]Resource{
// 						"u1": makeCached(makeUnstructured("apps", "v1", "Pod", "res1", "ns1", "uid1", map[string]string{
// 							common.GeneratePolicyLabel: "p1",
// 						}), map[string]string{
// 							common.GeneratePolicyLabel: "p1",
// 						}),
// 					},
// 				},
// 			},
// 			refCount:       map[schema.GroupVersionResource]int{gvr1: 1},
// 			wantDeleted:    []string{"Pod/ns1/res1"},
// 			wantCacheSizes: map[schema.GroupVersionResource]int{},
// 			wantStopped:    map[schema.GroupVersionResource]bool{gvr1: true},
// 			wantPolicyGone: true,
// 		},
// 		{
// 			name:             "multiple GVRs for one policy",
// 			policyName:       "p1",
// 			deleteDownstream: false,
// 			policyRefs:       map[string][]schema.GroupVersionResource{"p1": {gvr1, gvr2}},
// 			dynamicW: map[schema.GroupVersionResource]*watcher{
// 				gvr1: {metadataCache: map[types.UID]Resource{}},
// 				gvr2: {metadataCache: map[types.UID]Resource{}},
// 			},
// 			refCount:       map[schema.GroupVersionResource]int{gvr1: 1, gvr2: 1},
// 			wantCacheSizes: map[schema.GroupVersionResource]int{},
// 			wantStopped:    map[schema.GroupVersionResource]bool{gvr1: true, gvr2: true},
// 			wantPolicyGone: true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			client := &MockClient{err: tt.clientErr}
// 			wm := &WatchManager{
// 				policyRefs:      tt.policyRefs,
// 				dynamicWatchers: tt.dynamicW,
// 				refCount:        tt.refCount,
// 				client:          client,
// 				log:             logging.WithName("test"),
// 			}

// 			wm.RemoveWatchersForPolicy(tt.policyName, tt.deleteDownstream)

// 			assert.ElementsMatch(t, tt.wantDeleted, client.deleted)
// 			for gvr, wantSize := range tt.wantCacheSizes {
// 				if w, ok := wm.dynamicWatchers[gvr]; ok {
// 					assert.Equal(t, wantSize, len(w.metadataCache))
// 				}
// 			}
// 			// for gvr, stopped := range tt.wantStopped {
// 			// 	if w, ok := tt.dynamicW[gvr]; ok {
// 			// 		if s, ok := w.watcher.(*stoppable); ok {
// 			// 			assert.Equal(t, stopped, s.stopped)
// 			// 		}
// 			// 	}
// 			// }
// 			if tt.wantPolicyGone {
// 				_, exists := wm.policyRefs[tt.policyName]
// 				assert.False(t, exists, "policyRefs should not contain policy after removal")
// 			}
// 		})
// 	}
// }

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
	// Prepare a WatchManager with fake watchers
	wm := &WatchManager{
		// ensure lock is initialized
		lock: sync.Mutex{},
		// filled maps
		dynamicWatchers: make(map[schema.GroupVersionResource]*watcher),
		policyRefs:      make(map[string][]schema.GroupVersionResource),
		refCount:        make(map[schema.GroupVersionResource]int),
	}

	// Add a fake watcher
	gvr1 := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	mock1 := &mockStopper{}
	wm.dynamicWatchers[gvr1] = &watcher{watcher: mock1}
	wm.policyRefs["policy1"] = []schema.GroupVersionResource{gvr1}
	wm.refCount[gvr1] = 1

	// Add another fake watcher
	gvr2 := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	mock2 := &mockStopper{}
	wm.dynamicWatchers[gvr2] = &watcher{watcher: mock2}
	wm.policyRefs["policy2"] = []schema.GroupVersionResource{gvr2}
	wm.refCount[gvr2] = 2

	// Act
	wm.StopWatchers()

	// Assert watchers were stopped
	if !mock1.stopped {
		t.Errorf("Expected watcher for %v to be stopped", gvr1)
	}
	if !mock2.stopped {
		t.Errorf("Expected watcher for %v to be stopped", gvr2)
	}

	// Assert all maps are cleared
	if len(wm.dynamicWatchers) != 0 {
		t.Errorf("Expected dynamicWatchers to be empty, got %d", len(wm.dynamicWatchers))
	}
	if len(wm.policyRefs) != 0 {
		t.Errorf("Expected policyRefs to be empty, got %d", len(wm.policyRefs))
	}
	if len(wm.refCount) != 0 {
		t.Errorf("Expected refCount to be empty, got %d", len(wm.refCount))
	}
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
		{
			name:    "different resource",
			objName: "custom-obj",
			gvr:     schema.GroupVersionResource{Group: "custom.io", Version: "v1alpha1", Resource: "widgets"},
			wantMsg: "Resource added name=custom-obj",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			wm := &WatchManager{
				log: logging.GlobalLogger(), // assume your struct has exported field `Log` or settable in test
			}

			obj := &unstructured.Unstructured{}
			obj.SetName(tt.objName)

			wm.handleAdd(obj, tt.gvr)

			got := strings.TrimSpace(buf.String())
			if got != tt.wantMsg {
				t.Errorf("unexpected log output: got %q, want %q", got, tt.wantMsg)
			}
		})
	}
}
