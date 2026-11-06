package background

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	reportresource "github.com/kyverno/kyverno/pkg/controllers/report/resource"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type failingValidatingPolicyLister struct {
	policiesv1beta1listers.ValidatingPolicyLister
}

func (f *failingValidatingPolicyLister) List(labels.Selector) ([]*policiesv1beta1.ValidatingPolicy, error) {
	return nil, errors.New("transient validating policy lister error")
}

type fakeMetadataCache struct {
	resource reportresource.Resource
	gvk      schema.GroupVersionKind
	gvr      schema.GroupVersionResource
	exists   bool
}

func (f *fakeMetadataCache) GetResourceHash(types.UID) (reportresource.Resource, schema.GroupVersionKind, schema.GroupVersionResource, bool) {
	return f.resource, f.gvk, f.gvr, f.exists
}

func (f *fakeMetadataCache) GetAllResourceKeys() []string { return nil }

func (f *fakeMetadataCache) UpdateResourceHash(schema.GroupVersionResource, types.UID, reportresource.Resource) {
}

func (f *fakeMetadataCache) AddEventHandler(reportresource.EventHandler) {}

func (f *fakeMetadataCache) Warmup(context.Context) error { return nil }

type captureQueue struct {
	workqueue.TypedRateLimitingInterface[string]
	lastDelay time.Duration
	lastKey   string
}

func (c *captureQueue) AddAfter(item string, delay time.Duration) {
	c.lastDelay = delay
	c.lastKey = item
	c.TypedRateLimitingInterface.AddAfter(item, delay)
}

func TestReconcile_NoReconcileNeeded_StillRearmed(t *testing.T) {
	const (
		namespace  = "default"
		uid        = "pod-uid-123"
		hash       = "test-hash"
		forceDelay = time.Hour
	)

	key := namespace + "/" + uid

	report := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uid,
			Namespace: namespace,
			Labels: map[string]string{
				reportutils.LabelResourceHash: hash,
			},
			Annotations: map[string]string{
				annotationLastScanTime: time.Now().Format(time.RFC3339),
			},
		},
	}

	reportIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})
	if err := reportIndexer.Add(report); err != nil {
		t.Fatalf("failed to add report to indexer: %v", err)
	}

	celpolexIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})

	baseQ := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[string](),
		workqueue.TypedRateLimitingQueueConfig[string]{Name: "test-background-scan"},
	)
	cq := &captureQueue{TypedRateLimitingInterface: baseQ}

	ctrl := &controller{
		celpolexListener: policiesv1beta1listers.NewPolicyExceptionLister(celpolexIndexer),
		bgscanrLister: cache.NewGenericLister(
			reportIndexer,
			schema.GroupVersionResource{Group: "reports.kyverno.io", Version: "v1", Resource: "ephemeralreports"}.GroupResource(),
		),
		metadataCache: &fakeMetadataCache{
			resource: reportresource.Resource{
				Name:      "test-pod",
				Namespace: namespace,
				Hash:      hash,
			},
			gvk:    schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			gvr:    schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			exists: true,
		},
		queue:      cq,
		forceDelay: forceDelay,
	}

	if err := ctrl.reconcile(context.Background(), logr.Discard(), key, namespace, uid); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	if cq.lastKey != key {
		t.Fatalf("expected key %q, got %q", key, cq.lastKey)
	}
	if cq.lastDelay != forceDelay {
		t.Fatalf("expected delay %v, got %v", forceDelay, cq.lastDelay)
	}
}

func TestReconcile_ListerError_StillRearmed(t *testing.T) {
	const (
		namespace  = "default"
		uid        = "pod-uid-err"
		forceDelay = time.Hour
	)

	key := namespace + "/" + uid

	baseQ := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[string](),
		workqueue.TypedRateLimitingQueueConfig[string]{Name: "test-background-scan-err"},
	)
	cq := &captureQueue{TypedRateLimitingInterface: baseQ}

	ctrl := &controller{
		vpolLister: &failingValidatingPolicyLister{},
		metadataCache: &fakeMetadataCache{
			resource: reportresource.Resource{
				Name:      "test-pod",
				Namespace: namespace,
				Hash:      "hash",
			},
			gvk:    schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			gvr:    schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			exists: true,
		},
		queue:      cq,
		forceDelay: forceDelay,
	}

	if err := ctrl.reconcile(context.Background(), logr.Discard(), key, namespace, uid); err == nil {
		t.Fatal("expected reconcile to return error")
	}

	if cq.lastKey != key {
		t.Fatalf("expected key %q, got %q", key, cq.lastKey)
	}
	if cq.lastDelay != forceDelay {
		t.Fatalf("expected delay %v, got %v", forceDelay, cq.lastDelay)
	}
}

func TestShouldKeepStaleResult_PolicyUnchanged(t *testing.T) {
	policyNameToLabel := map[string]string{"require-requests-limits": "policy.kyverno.io/require-requests-limits"}
	labels := map[string]string{"policy.kyverno.io/require-requests-limits": "1"}
	result := openreportsv1alpha1.ReportResult{Policy: "require-requests-limits"}

	assert.True(t, shouldKeepStaleResult(result, policyNameToLabel, labels, labels))
}

// A policy edited to exclude a namespace bumps its resource version; a stale "pass" recorded
// under the old version must be dropped so the resource is rescanned.
func TestShouldKeepStaleResult_PolicyChanged(t *testing.T) {
	policyNameToLabel := map[string]string{"require-requests-limits": "policy.kyverno.io/require-requests-limits"}
	expected := map[string]string{"policy.kyverno.io/require-requests-limits": "2"}
	actual := map[string]string{"policy.kyverno.io/require-requests-limits": "1"}
	result := openreportsv1alpha1.ReportResult{Policy: "require-requests-limits"}

	assert.False(t, shouldKeepStaleResult(result, policyNameToLabel, expected, actual))
}

func TestShouldKeepStaleResult_BindingChanged(t *testing.T) {
	for _, property := range []string{"binding", "mapBinding"} {
		t.Run(property, func(t *testing.T) {
			policyNameToLabel := map[string]string{
				"my-policy":  "policy.kyverno.io/my-policy",
				"my-binding": "binding.example/my-binding",
			}
			expected := map[string]string{"policy.kyverno.io/my-policy": "1", "binding.example/my-binding": "2"}
			actual := map[string]string{"policy.kyverno.io/my-policy": "1", "binding.example/my-binding": "1"}
			result := openreportsv1alpha1.ReportResult{
				Policy:     "my-policy",
				Properties: map[string]string{property: "my-binding"},
			}

			assert.False(t, shouldKeepStaleResult(result, policyNameToLabel, expected, actual))
		})
	}
}

func TestShouldKeepStaleResult_BindingDeleted(t *testing.T) {
	for _, property := range []string{"binding", "mapBinding"} {
		t.Run(property, func(t *testing.T) {
			policyNameToLabel := map[string]string{"my-policy": "policy.kyverno.io/my-policy"}
			labels := map[string]string{"policy.kyverno.io/my-policy": "1"}
			result := openreportsv1alpha1.ReportResult{
				Policy:     "my-policy",
				Properties: map[string]string{property: "my-binding"},
			}

			assert.False(t, shouldKeepStaleResult(result, policyNameToLabel, labels, labels))
		})
	}
}

func TestShouldKeepStaleResult_PolicyDeleted(t *testing.T) {
	result := openreportsv1alpha1.ReportResult{Policy: "some-other-policy"}

	assert.False(t, shouldKeepStaleResult(result, map[string]string{}, map[string]string{}, map[string]string{}))
}

func TestShouldKeepStaleResult_NoReferences(t *testing.T) {
	policyNameToLabel := map[string]string{"my-policy": "policy.kyverno.io/my-policy"}
	labels := map[string]string{"policy.kyverno.io/my-policy": "1"}

	assert.False(t, shouldKeepStaleResult(openreportsv1alpha1.ReportResult{}, policyNameToLabel, labels, labels))
}
