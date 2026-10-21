package ttl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/metric"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	metafake "k8s.io/client-go/metadata/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
)

// TestDeterminePropagationPolicy tests the determinePropagationPolicy function
func TestDeterminePropagationPolicy(t *testing.T) {
	logger := logr.Discard() // Use a no-op logger

	testCases := []struct {
		name           string
		annotations    map[string]string
		expectedPolicy *metav1.DeletionPropagation
	}{
		{
			name:           "No annotations",
			annotations:    nil,
			expectedPolicy: nil,
		},
		{
			name: "Foreground policy",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "Foreground",
			},
			expectedPolicy: ptr.To(metav1.DeletePropagationForeground),
		},
		{
			name: "Background policy",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "Background",
			},
			expectedPolicy: ptr.To(metav1.DeletePropagationBackground),
		},
		{
			name: "Orphan policy",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "Orphan",
			},
			expectedPolicy: ptr.To(metav1.DeletePropagationOrphan),
		},
		{
			name: "Empty annotation",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "",
			},
			expectedPolicy: nil,
		},
		{
			name: "Unknown policy",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "Unknown",
			},
			expectedPolicy: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock metadata object with annotations
			metaObj := &metav1.ObjectMeta{
				Annotations: tc.annotations,
			}
			// Call the function
			policy := determinePropagationPolicy(metaObj, logger)
			// Assert the result
			assert.Equal(t, tc.expectedPolicy, policy)
		})
	}
}

type captureQueue struct {
	workqueue.TypedRateLimitingInterface[any]
	lastDelay time.Duration
	lastKey   any
}

func (c *captureQueue) AddAfter(item any, delay time.Duration) {
	c.lastDelay = delay
	c.lastKey = item
	c.TypedRateLimitingInterface.AddAfter(item, delay)
}

func newDeleteErrorTestController(t *testing.T) (*controller, *captureQueue, string) {
	t.Helper()

	obj := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cm",
			Namespace:         "default",
			UID:               types.UID("test-cm-uid"),
			ResourceVersion:   "10",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
			Labels: map[string]string{
				kyverno.LabelCleanupTtl: "1h",
			},
		},
	}

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})
	if err := indexer.Add(obj); err != nil {
		t.Fatalf("indexer add failed: %v", err)
	}

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	lister := cache.NewGenericLister(indexer, gvr.GroupResource())

	client := metafake.NewSimpleMetadataClient(runtime.NewScheme())
	client.PrependReactor("delete", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("transient delete failure")
	})

	baseQ := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: "test-ttl"},
	)
	cq := &captureQueue{TypedRateLimitingInterface: baseQ}

	ctrl := &controller{
		client: client.Resource(gvr),
		queue:  cq,
		lister: lister,
		logger: logr.Discard(),
		gvr:    gvr,
	}

	return ctrl, cq, "default/test-cm"
}

func exhaustRetryBudget(t *testing.T, q workqueue.TypedRateLimitingInterface[any], itemKey string) {
	t.Helper()

	q.Add(itemKey)
	for q.NumRequeues(itemKey) < maxRetries {
		q.AddRateLimited(itemKey)
	}
}

func TestReconcile_DeleteError_NoRearmBeforeRetryExhausted(t *testing.T) {
	ctrl, cq, itemKey := newDeleteErrorTestController(t)

	err := ctrl.reconcile(context.Background(), logr.Discard(), itemKey, "", "")
	if err == nil {
		t.Fatal("expected reconcile to return error")
	}
	if cq.lastKey != nil {
		t.Fatalf("expected no re-arm before retry budget exhausted, got key=%v delay=%v", cq.lastKey, cq.lastDelay)
	}
}

func TestReconcile_DeleteError_StillRearmed(t *testing.T) {
	ctrl, cq, itemKey := newDeleteErrorTestController(t)
	assertRearmedAfterRetryExhaustion(t, ctrl, cq, itemKey)
}

type nonMetaObject struct{}

func (n *nonMetaObject) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }
func (n *nonMetaObject) DeepCopyObject() runtime.Object   { return &nonMetaObject{} }

func newListerErrorTestController(t *testing.T, lister cache.GenericLister) (*controller, *captureQueue, string) {
	t.Helper()

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	baseQ := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: "test-ttl-lister"},
	)
	cq := &captureQueue{TypedRateLimitingInterface: baseQ}

	ctrl := &controller{
		client: metafake.NewSimpleMetadataClient(runtime.NewScheme()).Resource(gvr),
		queue:  cq,
		lister: lister,
		logger: logr.Discard(),
		gvr:    gvr,
	}

	return ctrl, cq, "default/test-cm"
}

func assertRearmedAfterRetryExhaustion(t *testing.T, ctrl *controller, cq *captureQueue, itemKey string) {
	t.Helper()

	exhaustRetryBudget(t, cq.TypedRateLimitingInterface, itemKey)

	cq.lastDelay = 0
	cq.lastKey = nil

	err := ctrl.reconcile(context.Background(), logr.Discard(), itemKey, "", "")
	if err == nil {
		t.Fatal("expected reconcile to return error")
	}
	if cq.lastKey != itemKey {
		t.Fatalf("expected key %q, got %v", itemKey, cq.lastKey)
	}
	if cq.lastDelay != minRequeueDelay {
		t.Fatalf("expected delay %v, got %v", minRequeueDelay, cq.lastDelay)
	}
}

func TestReconcile_ListerError_StillRearmed(t *testing.T) {
	ctrl, cq, itemKey := newListerErrorTestController(t, &mockLister{
		err: errors.New("transient lister error"),
	})
	assertRearmedAfterRetryExhaustion(t, ctrl, cq, itemKey)
}

func TestReconcile_AccessorError_StillRearmed(t *testing.T) {
	ctrl, cq, itemKey := newListerErrorTestController(t, &mockLister{
		obj: &nonMetaObject{},
	})
	assertRearmedAfterRetryExhaustion(t, ctrl, cq, itemKey)
}

type mockResourceInterface struct {
	metadata.ResourceInterface
	deleteCalled  bool
	deletedName   string
	deleteErr     error
	deleteOptions metav1.DeleteOptions
}

func (m *mockResourceInterface) Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error {
	m.deleteCalled = true
	m.deletedName = name
	m.deleteOptions = opts
	return m.deleteErr
}

type mockMetadataGetter struct {
	metadata.Getter
	resource           *mockResourceInterface
	requestedNamespace string
}

func (m *mockMetadataGetter) Namespace(namespace string) metadata.ResourceInterface {
	m.requestedNamespace = namespace
	return m.resource
}

type mockLister struct {
	cache.GenericLister
	obj runtime.Object
	err error
}

func (m *mockLister) Get(name string) (runtime.Object, error) {
	return m.obj, m.err
}

func (m *mockLister) ByNamespace(namespace string) cache.GenericNamespaceLister {
	return &mockNamespaceLister{obj: m.obj, err: m.err}
}

type mockNamespaceLister struct {
	cache.GenericNamespaceLister
	obj runtime.Object
	err error
}

func (m *mockNamespaceLister) Get(name string) (runtime.Object, error) {
	return m.obj, m.err
}

type mockMetrics struct {
	deletedCount int
	failureCount int
}

func (m *mockMetrics) RecordTTLInfo(ctx context.Context, gvr schema.GroupVersionResource, observer metric.Observer) {
}

func (m *mockMetrics) RecordDeletedObject(ctx context.Context, gvr schema.GroupVersionResource, namespace string) {
	m.deletedCount++
}

func (m *mockMetrics) RecordTTLFailure(ctx context.Context, gvr schema.GroupVersionResource, namespace string) {
	m.failureCount++
}

func (m *mockMetrics) RegisterCallback(f metric.Callback) (metric.Registration, error) {
	return nil, nil
}

type mockQueue struct {
	workqueue.TypedRateLimitingInterface[any]
	addAfterCalled bool
	addAfterItem   any
}

func (m *mockQueue) AddAfter(item any, duration time.Duration) {
	m.addAfterCalled = true
	m.addAfterItem = item
	m.TypedRateLimitingInterface.AddAfter(item, duration)
}

func TestReconcileMetrics(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	logger := logr.Discard()

	tests := []struct {
		name                  string
		ttlValue              string
		creationTime          time.Time
		deleteErr             error
		expectDelete          bool
		expectDeleteCalled    bool
		expectedDeletedMetric int
		expectedFailureMetric int
		expectErr             bool
	}{
		{
			name:                  "TTL not expired",
			ttlValue:              "2h",
			creationTime:          time.Now(),
			expectDelete:          false,
			expectDeleteCalled:    false,
			expectedDeletedMetric: 0,
			expectedFailureMetric: 0,
			expectErr:             false,
		},
		{
			name:                  "TTL expired - successful deletion",
			ttlValue:              "1m",
			creationTime:          time.Now().Add(-2 * time.Minute),
			expectDelete:          true,
			expectDeleteCalled:    true,
			deleteErr:             nil,
			expectedDeletedMetric: 1,
			expectedFailureMetric: 0,
			expectErr:             false,
		},
		{
			name:                  "TTL expired - failed deletion",
			ttlValue:              "1m",
			creationTime:          time.Now().Add(-2 * time.Minute),
			expectDelete:          true,
			expectDeleteCalled:    true,
			deleteErr:             apierrors.NewInternalError(fmt.Errorf("some internal error")),
			expectedDeletedMetric: 0,
			expectedFailureMetric: 1,
			expectErr:             true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			obj := &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-resource",
					Namespace:         "test-namespace",
					UID:               types.UID("test-resource-uid"),
					ResourceVersion:   "10",
					CreationTimestamp: metav1.NewTime(tc.creationTime),
					Labels: map[string]string{
						kyverno.LabelCleanupTtl: tc.ttlValue,
					},
				},
			}

			mockRes := &mockResourceInterface{
				deleteErr: tc.deleteErr,
			}
			mockClient := &mockMetadataGetter{
				resource: mockRes,
			}
			mockL := &mockLister{
				obj: obj,
			}
			mockM := &mockMetrics{}
			baseQ := workqueue.NewTypedRateLimitingQueueWithConfig(
				workqueue.DefaultTypedControllerRateLimiter[any](),
				workqueue.TypedRateLimitingQueueConfig[any]{Name: "test-ttl-metrics"},
			)
			queue := &mockQueue{TypedRateLimitingInterface: baseQ}

			c := &controller{
				client:  mockClient,
				lister:  mockL,
				metrics: mockM,
				queue:   queue,
				gvr:     gvr,
			}

			err := c.reconcile(context.TODO(), logger, "test-namespace/test-resource", "", "")

			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectDeleteCalled, mockRes.deleteCalled)
			assert.Equal(t, tc.expectedDeletedMetric, mockM.deletedCount)
			assert.Equal(t, tc.expectedFailureMetric, mockM.failureCount)

			if !tc.expectDelete {
				assert.True(t, queue.addAfterCalled, "object should be re-queued")
				assert.Equal(t, "test-namespace/test-resource", queue.addAfterItem)
			} else {
				assert.False(t, queue.addAfterCalled, "object should not be re-queued")
			}
		})
	}
}

func TestReconcileExpiredTTLDeletePreconditions(t *testing.T) {
	for _, tc := range []struct {
		name            string
		namespace       string
		resourceVersion string
		annotations     map[string]string
		propagation     *metav1.DeletionPropagation
	}{
		{name: "namespaced", namespace: "tenant", resourceVersion: "100"},
		{name: "cluster scoped", resourceVersion: "100"},
		{name: "empty revision", namespace: "tenant"},
		{
			name: "preserves propagation policy", namespace: "tenant", resourceVersion: "100",
			annotations: map[string]string{kyverno.AnnotationCleanupPropagationPolicy: "Foreground"},
			propagation: ptr.To(metav1.DeletePropagationForeground),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cached := &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Name: "job-config", Namespace: tc.namespace,
					UID: types.UID("old-uid"), ResourceVersion: tc.resourceVersion,
					CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
					Labels:            map[string]string{kyverno.LabelCleanupTtl: "1h"},
					Annotations:       tc.annotations,
				},
			}
			resource := &mockResourceInterface{}
			getter := &mockMetadataGetter{resource: resource}
			queue := newTTLTestQueue(t)
			c := &controller{
				client: getter,
				lister: &mockLister{obj: cached},
				queue:  queue,
			}
			key, err := cache.MetaNamespaceKeyFunc(cached)
			if err != nil {
				t.Fatal(err)
			}
			if err := c.reconcile(context.Background(), logr.Discard(), key, "", ""); err != nil {
				t.Fatal(err)
			}
			if !resource.deleteCalled || resource.deletedName != cached.Name {
				t.Fatalf("expected deletion for %q, got called=%v name=%q", key, resource.deleteCalled, resource.deletedName)
			}
			assert.Equal(t, tc.namespace, getter.requestedNamespace)
			got := resource.deleteOptions.Preconditions
			if got == nil || got.UID == nil || *got.UID != cached.UID {
				t.Fatalf("DELETE UID does not match cached object: %+v", got)
			}
			if tc.resourceVersion == "" {
				assert.Nil(t, got.ResourceVersion)
			} else if got.ResourceVersion == nil || *got.ResourceVersion != tc.resourceVersion {
				t.Fatalf("DELETE resourceVersion does not match cached revision: %+v", got)
			}
			assert.Equal(t, tc.propagation, resource.deleteOptions.PropagationPolicy)
		})
	}
}

func newTTLTestQueue(t *testing.T) workqueue.TypedRateLimitingInterface[any] {
	t.Helper()
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: "ttl-preconditions-test"},
	)
	t.Cleanup(queue.ShutDown)
	return queue
}

func TestReconcileExpiredTTLRefusesMissingUID(t *testing.T) {
	cached := &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{
			Name: "job-config", Namespace: "tenant",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
			Labels:            map[string]string{kyverno.LabelCleanupTtl: "1h"},
		},
	}
	resource := &mockResourceInterface{}
	metrics := &mockMetrics{}
	c := &controller{
		client:  &mockMetadataGetter{resource: resource},
		lister:  &mockLister{obj: cached},
		queue:   newTTLTestQueue(t),
		metrics: metrics,
	}
	err := c.reconcile(context.Background(), logr.Discard(), "tenant/job-config", "", "")
	assert.ErrorContains(t, err, "missing resource UID")
	assert.False(t, resource.deleteCalled, "missing UID must never permit unconditional deletion")
	assert.Equal(t, 0, metrics.deletedCount)
	assert.Equal(t, 1, metrics.failureCount)
}

func TestReconcileExpiredTTLDeleteConflictPreservesFailureAccounting(t *testing.T) {
	cached := &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{
			Name: "job-config", Namespace: "tenant",
			UID: types.UID("old-uid"), ResourceVersion: "100",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
			Labels:            map[string]string{kyverno.LabelCleanupTtl: "1h"},
		},
	}
	resource := &mockResourceInterface{
		deleteErr: apierrors.NewConflict(
			schema.GroupResource{Resource: "configmaps"}, cached.Name,
			fmt.Errorf("resource UID changed"),
		),
	}
	metrics := &mockMetrics{}
	c := &controller{
		client:  &mockMetadataGetter{resource: resource},
		lister:  &mockLister{obj: cached},
		queue:   newTTLTestQueue(t),
		metrics: metrics,
	}
	err := c.reconcile(context.Background(), logr.Discard(), "tenant/job-config", "", "")
	assert.True(t, apierrors.IsConflict(err))
	assert.True(t, resource.deleteCalled)
	assert.NotNil(t, resource.deleteOptions.Preconditions)
	assert.Equal(t, 0, metrics.deletedCount)
	assert.Equal(t, 1, metrics.failureCount)
}

// This test uses the real client-go HTTP metadata client against a small test
// API handler. Unlike the mock above, a conflicting DELETE is checked against
// the simulated live object, and a follow-up GET verifies it is still present.
func TestReconcileStaleTTLDeletePreservesLiveObjectHTTP(t *testing.T) {
	for _, tc := range []struct {
		name    string
		liveUID types.UID
		liveRV  string
	}{
		{name: "recreated under same name", liveUID: "replacement-uid", liveRV: "200"},
		{name: "TTL removed on same object", liveUID: "original-uid", liveRV: "200"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cached := &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Name: "job-config", Namespace: "tenant", UID: "original-uid",
					ResourceVersion:   "100",
					CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
					Labels:            map[string]string{kyverno.LabelCleanupTtl: "1h"},
				},
			}
			live := cached.DeepCopy()
			live.UID = tc.liveUID
			live.ResourceVersion = tc.liveRV
			delete(live.Labels, kyverno.LabelCleanupTtl)
			live.TypeMeta = metav1.TypeMeta{APIVersion: "meta.k8s.io/v1", Kind: "PartialObjectMetadata"}

			var mu sync.Mutex
			deleted := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path != "/api/v1/namespaces/tenant/configmaps/job-config" {
					http.Error(w, "unexpected resource path", http.StatusNotFound)
					return
				}
				switch r.Method {
				case http.MethodDelete:
					var opts metav1.DeleteOptions
					if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					mu.Lock()
					defer mu.Unlock()
					if opts.Preconditions != nil && ((opts.Preconditions.UID != nil && *opts.Preconditions.UID != live.UID) || (opts.Preconditions.ResourceVersion != nil && *opts.Preconditions.ResourceVersion != live.ResourceVersion)) {
						w.WriteHeader(http.StatusConflict)
						_ = json.NewEncoder(w).Encode(metav1.Status{
							TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"},
							Status:   metav1.StatusFailure, Reason: metav1.StatusReasonConflict,
							Code:    http.StatusConflict,
							Message: "UID or resourceVersion precondition failed",
						})
						return
					}
					deleted = true
					_ = json.NewEncoder(w).Encode(metav1.Status{
						TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"},
						Status:   metav1.StatusSuccess,
					})
				case http.MethodGet:
					mu.Lock()
					defer mu.Unlock()
					if deleted {
						http.Error(w, "deleted", http.StatusNotFound)
						return
					}
					_ = json.NewEncoder(w).Encode(live)
				default:
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				}
			}))
			defer server.Close()
			metadataClient, err := metadata.NewForConfig(&rest.Config{Host: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			resourceClient := metadataClient.Resource(schema.GroupVersionResource{Version: "v1", Resource: "configmaps"})
			metrics := &mockMetrics{}
			c := &controller{
				client:  resourceClient,
				lister:  &mockLister{obj: cached},
				queue:   newTTLTestQueue(t),
				metrics: metrics,
			}
			err = c.reconcile(context.Background(), logr.Discard(), "tenant/job-config", "", "")
			if !apierrors.IsConflict(err) {
				t.Fatalf("expected a stale-delete conflict, got %v", err)
			}
			current, err := resourceClient.Namespace("tenant").Get(context.Background(), "job-config", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("replacement was not preserved: %v", err)
			}
			assert.Equal(t, tc.liveUID, current.UID)
			assert.Equal(t, tc.liveRV, current.ResourceVersion)
			mu.Lock()
			wasDeleted := deleted
			mu.Unlock()
			assert.False(t, wasDeleted)
			assert.Equal(t, 0, metrics.deletedCount)
			assert.Equal(t, 1, metrics.failureCount)
		})
	}
}
