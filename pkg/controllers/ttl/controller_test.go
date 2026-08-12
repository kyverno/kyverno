package ttl

import (
	"context"
	"fmt"
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
	"k8s.io/client-go/metadata"
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

type mockResourceInterface struct {
	metadata.ResourceInterface
	deleteCalled bool
	deletedName  string
	deleteErr    error
}

func (m *mockResourceInterface) Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error {
	m.deleteCalled = true
	m.deletedName = name
	return m.deleteErr
}

type mockMetadataGetter struct {
	metadata.Getter
	resource *mockResourceInterface
}

func (m *mockMetadataGetter) Namespace(string) metadata.ResourceInterface {
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
			queue := &mockQueue{}

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
