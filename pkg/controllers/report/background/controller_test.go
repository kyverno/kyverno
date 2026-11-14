package background

import (
	"context"
	"errors"
	"github.com/kyverno/kyverno/pkg/breaker"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/openreports"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	reportresource "github.com/kyverno/kyverno/pkg/controllers/report/resource"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
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

func TestStoreReport_AllowedResultsSuppression(t *testing.T) {
	// Configure allowedResults to exclude "pass"
	reportutils.ResetReportingConfig([]string{"fail", "error", "warn"}, "validate")
	t.Cleanup(func() {
		reportutils.ResetReportingConfig([]string{"pass", "fail", "error", "warn", "skip"}, "validate")
	})

	breaker.SetReportsBreaker(breaker.NewBreaker("test", func(context.Context) bool { return false }))

	const (
		namespace = "default"
		uid       = "test-uid-123"
	)

	// Create lister with existing policyreport for test-uid-123
	polrIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})
	polrLister := cache.NewGenericLister(
		polrIndexer,
		schema.GroupVersionResource{Group: "wgpolicyk8s.io", Version: "v1alpha2", Resource: "policyreports"}.GroupResource(),
	)

	// Test 1: desired has only "pass", no existing policyreport -> suppressed (no ephemeral report created)
	{
		client := versionedfake.NewSimpleClientset()
		ctrl := &controller{
			kyvernoClient: client,
			polrLister:    polrLister,
		}

		observed := &reportsv1.EphemeralReport{} // hasReport = false
		desired := &reportsv1.EphemeralReport{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Labels: map[string]string{
					reportutils.LabelResourceUid: "other-uid-without-report",
				},
			},
			Spec: reportsv1.EphemeralReportSpec{
				Results: []openreportsv1alpha1.ReportResult{
					{Result: openreports.StatusPass},
				},
			},
		}

		err := ctrl.storeReport(context.Background(), observed, desired)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, a := range client.Fake.Actions() {
			if a.GetVerb() == "create" && a.GetResource().Resource == "ephemeralreports" {
				t.Fatalf("expected no ephemeral report created when pass is not allowed and resource has no policy report")
			}
		}
	}

	// Test 2: desired has only "pass", but existing policyreport exists -> created so aggregate-controller can clear it
	{
		// Add report for test-uid-123 to polrIndexer
		existingReportMeta := &metav1.PartialObjectMetadata{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      uid,
			},
		}
		_ = polrIndexer.Add(existingReportMeta)

		client := versionedfake.NewSimpleClientset()
		ctrl := &controller{
			kyvernoClient: client,
			polrLister:    polrLister,
		}

		observed := &reportsv1.EphemeralReport{} // hasReport = false
		desired := &reportsv1.EphemeralReport{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Labels: map[string]string{
					reportutils.LabelResourceUid: uid,
				},
			},
			Spec: reportsv1.EphemeralReportSpec{
				Results: []openreportsv1alpha1.ReportResult{
					{Result: openreports.StatusPass},
				},
			},
		}

		err := ctrl.storeReport(context.Background(), observed, desired)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var sawCreate bool
		for _, a := range client.Fake.Actions() {
			if a.GetVerb() == "create" && a.GetResource().Resource == "ephemeralreports" {
				sawCreate = true
			}
		}
		if !sawCreate {
			t.Fatalf("expected ephemeral report created to clear existing policy report")
		}
	}

	// Test 3: desired has "fail" -> created
	{
		client := versionedfake.NewSimpleClientset()
		ctrl := &controller{
			kyvernoClient: client,
			polrLister:    polrLister,
		}

		observed := &reportsv1.EphemeralReport{} // hasReport = false
		desired := &reportsv1.EphemeralReport{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Labels: map[string]string{
					reportutils.LabelResourceUid: "failing-uid",
				},
			},
			Spec: reportsv1.EphemeralReportSpec{
				Results: []openreportsv1alpha1.ReportResult{
					{Result: openreports.StatusFail},
				},
			},
		}

		err := ctrl.storeReport(context.Background(), observed, desired)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var sawCreate bool
		for _, a := range client.Fake.Actions() {
			if a.GetVerb() == "create" && a.GetResource().Resource == "ephemeralreports" {
				sawCreate = true
			}
		}
		if !sawCreate {
			t.Fatalf("expected ephemeral report created for fail result")
		}
	}
}
