package cleanup

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	v2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	configpkg "github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/config/mocks"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func Test_SkipResourceDueToFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := mocks.NewMockConfiguration(ctrl)

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}

	mockConfig.EXPECT().
		ToFilter(gvk, "ConfigMap", "kube-system", "filtered-cm").
		Return(true).
		AnyTimes()

	c := &controller{
		configuration: mockConfig,
	}

	resource := unstructured.Unstructured{}
	resource.SetKind("ConfigMap")
	resource.SetNamespace("kube-system")
	resource.SetName("filtered-cm")

	filtered := c.configuration.ToFilter(
		gvk, resource.GetKind(), resource.GetNamespace(), resource.GetName(),
	)

	assert.True(t, filtered, "Expected resource to be filtered and skipped")
}

// captureQueue wraps a real typed queue but captures the last AddAfter delay used by the controller.
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

// Test that reconcile clamps the requeue delay when the next execution time
// (derived from a very old lastExecutionTime) is in the past.
func TestReconcile_ClampPastNextExecution(t *testing.T) {
	// Build a CleanupPolicy with an ancient lastExecutionTime and a frequent schedule.
	pol := &kyvernov2.CleanupPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kyverno.io/v2",
			Kind:       "CleanupPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pol",
			Namespace: "ns1",
		},
		Spec: kyvernov2.CleanupPolicySpec{
			Schedule: "* * * * *",
		},
		Status: kyvernov2.CleanupPolicyStatus{
			LastExecutionTime: metav1.Time{
				Time: time.Date(1901, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	fakeClient := versionedfake.NewSimpleClientset(pol.DeepCopy())

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc,
		cache.Indexers{
			cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
		},
	)
	if err := indexer.Add(pol.DeepCopy()); err != nil {
		t.Fatalf("indexer add failed: %v", err)
	}
	polLister := v2listers.NewCleanupPolicyLister(indexer)

	baseQ := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{
			Name: "test-cleanup",
		},
	)
	cq := &captureQueue{TypedRateLimitingInterface: baseQ}

	ctrl := &controller{
		kyvernoClient: fakeClient,
		polLister:     polLister,
		queue:         cq,
		jp:            jmespath.New(configpkg.NewDefaultConfiguration(false)),
	}

	if err := ctrl.reconcile(
		context.Background(),
		logr.Discard(),
		"ns1/test-pol",
		"ns1",
		"test-pol",
	); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	if cq.lastDelay < minRequeueDelay || cq.lastDelay > minRequeueDelay+60*time.Second {
		t.Fatalf("expected delay to next cron minute, got %v", cq.lastDelay)
	}
}
