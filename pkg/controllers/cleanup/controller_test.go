package cleanup

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	v2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	configpkg "github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/config/mocks"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type mockDClient struct {
	dclient.Interface
	listResource   func(ctx context.Context, apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error)
	deleteResource func(ctx context.Context, apiVersion string, kind string, namespace string, name string, dryRun bool, options metav1.DeleteOptions) error
}

func (m *mockDClient) ListResource(ctx context.Context, apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	if m.listResource != nil {
		return m.listResource(ctx, apiVersion, kind, namespace, lselector)
	}
	return m.Interface.ListResource(ctx, apiVersion, kind, namespace, lselector)
}

func (m *mockDClient) DeleteResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, dryRun bool, options metav1.DeleteOptions) error {
	if m.deleteResource != nil {
		return m.deleteResource(ctx, apiVersion, kind, namespace, name, dryRun, options)
	}
	return m.Interface.DeleteResource(ctx, apiVersion, kind, namespace, name, dryRun, options)
}

func Test_Cleanup_SkipResourceNamespaceMismatch(t *testing.T) {
	// Define a CleanupPolicy in ns1
	policy := &kyvernov2.CleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "ns1",
		},
		Spec: kyvernov2.CleanupPolicySpec{
			MatchResources: kyvernov2.MatchResources{
				Any: []kyvernov1.ResourceFilter{
					{
						ResourceDescription: kyvernov1.ResourceDescription{
							Kinds: []string{"ConfigMap"},
						},
					},
				},
			},
		},
	}

	// Create a resource in ns2
	resource := unstructured.Unstructured{}
	resource.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	})
	resource.SetName("test-cm")
	resource.SetNamespace("ns2") // Mismatching namespace

	// Setup mock dclient
	baseClient := dclient.NewEmptyFakeClient()
	mockClient := &mockDClient{
		Interface: baseClient,
	}

	// Override ListResource to return the resource in ns2
	mockClient.listResource = func(ctx context.Context, apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
		return &unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{resource},
		}, nil
	}

	// Override DeleteResource to fail if called
	mockClient.deleteResource = func(ctx context.Context, apiVersion string, kind string, namespace string, name string, dryRun bool, options metav1.DeleteOptions) error {
		t.Fatalf("DeleteResource should not be called for resource in namespace %s when policy is in %s", namespace, policy.GetNamespace())
		return nil
	}

	// Setup controller dependencies
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock Config
	mockConfig := mocks.NewMockConfiguration(ctrl)
	// Expect ToFilter to be called and return false (don't skip due to filter)
	mockConfig.EXPECT().
		ToFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false).
		AnyTimes()

	// Mock Namespace Lister
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	ns2 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns2"}}
	if err := indexer.Add(ns2); err != nil {
		t.Fatalf("failed to add namespace: %v", err)
	}
	nsLister := corev1listers.NewNamespaceLister(indexer)

	c := &controller{
		client:        mockClient,
		configuration: mockConfig,
		nsLister:      nsLister,
		jp:            jmespath.New(configpkg.NewDefaultConfiguration(false)),
		gctxStore:     nil,
		cmResolver:    nil,
	}

	ctx := context.Background()
	err := c.cleanup(ctx, logr.Discard(), policy)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
}

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

	// add a tolerance to the lower bound to account for test flakiness
	if cq.lastDelay < minRequeueDelay-100*time.Millisecond || cq.lastDelay > minRequeueDelay+60*time.Second {
		t.Fatalf("expected delay to next cron minute, got %v", cq.lastDelay)
	}
}
