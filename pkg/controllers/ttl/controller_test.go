package ttl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func TestReconcile_DeleteError_StillRearmed(t *testing.T) {
	obj := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cm",
			Namespace:         "default",
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

	itemKey := "default/test-cm"
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
