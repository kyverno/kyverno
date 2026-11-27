package background

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	reportresource "github.com/kyverno/kyverno/pkg/controllers/report/resource"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func TestReconcileReportChecksResourceUID(t *testing.T) {
	t.Parallel()

	const liveUID types.UID = "live-uid"
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}
	for _, tc := range []struct {
		name      string
		queuedUID types.UID
		wantScan  bool
	}{
		{name: "same UID", queuedUID: liveUID, wantScan: true},
		{name: "resource replaced", queuedUID: "deleted-uid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resource := &unstructured.Unstructured{}
			resource.SetGroupVersionKind(gvk)
			resource.SetName("test-namespace")
			resource.SetUID(liveUID)
			dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), resource)
			kyvernoClient := versionedfake.NewSimpleClientset()
			c := &controller{
				client:        dclient.NewFakeClientWithDisco(dynamicClient, nil, dclient.NewFakeDiscoveryClient(nil)),
				kyvernoClient: kyvernoClient,
			}

			err := c.reconcileReport(t.Context(), "", string(tc.queuedUID), true, tc.queuedUID, gvk, gvr, reportresource.Resource{Name: resource.GetName()}, nil, nil, nil)
			require.NoError(t, err)
			if tc.wantScan {
				require.Len(t, kyvernoClient.Actions(), 1)
				require.True(t, kyvernoClient.Actions()[0].Matches("get", "clusterephemeralreports"))
			} else {
				require.Empty(t, kyvernoClient.Actions())
			}
		})
	}
}

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
		celpolexListener: celengine.NewPolicyExceptionLister(policiesv1beta1listers.NewPolicyExceptionLister(celpolexIndexer), "*"),
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
