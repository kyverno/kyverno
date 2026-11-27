package background

import (
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type revisionMetadataCache struct {
	*fakeMetadataCache
	keys []string
}

func (c *revisionMetadataCache) GetAllResourceKeys() []string {
	return c.keys
}

func TestPolicyUpdate_StatusAndMetadataOnlyChangesDoNotEnqueue(t *testing.T) {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[string](),
		workqueue.TypedRateLimitingQueueConfig[string]{Name: "test-revision-updates"},
	)
	defer queue.ShutDown()

	ctrl := &controller{
		queue: queue,
		metadataCache: &revisionMetadataCache{
			fakeMetadataCache: &fakeMetadataCache{},
			keys:              []string{"default/one", "default/two"},
		},
	}
	before := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy", Generation: 3, ResourceVersion: "10"},
	}
	after := before.DeepCopy()
	after.ResourceVersion = "11"
	after.Annotations = map[string]string{"example.com/unrelated": "new"}
	after.Status.ConditionStatus.Message = "status refreshed"

	ctrl.updateVP(before, after)
	if got := queue.Len(); got != 0 {
		t.Fatalf("status-only VPOL update enqueued %d resources, want zero", got)
	}

	mpolOld := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mutating", Generation: 3, ResourceVersion: "10"},
	}
	mpolNew := mpolOld.DeepCopy()
	mpolNew.ResourceVersion = "11"
	mpolNew.Status.ConditionStatus.Message = "status refreshed"
	ctrl.updateMP(mpolOld, mpolNew)
	if got := queue.Len(); got != 0 {
		t.Fatalf("status-only MPOL update enqueued %d resources, want zero", got)
	}

	ivpolOld := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "image", Generation: 3, ResourceVersion: "10"},
	}
	ivpolNew := ivpolOld.DeepCopy()
	ivpolNew.ResourceVersion = "11"
	ivpolNew.Status.ConditionStatus.Message = "status refreshed"
	ctrl.updateIVP(ivpolOld, ivpolNew)
	if got := queue.Len(); got != 0 {
		t.Fatalf("status-only IVPOL update enqueued %d resources, want zero", got)
	}

	// Drain between cases: workqueue deduplicates keys still queued.
	checkGeneration := func(name string, update func()) {
		t.Helper()
		update()
		if got := queue.Len(); got != 2 {
			t.Fatalf("%s generation update enqueued %d resources, want 2", name, got)
		}
		for queue.Len() > 0 {
			item, shutdown := queue.Get()
			if shutdown {
				t.Fatal("queue shut down during revision test")
			}
			queue.Done(item)
			queue.Forget(item)
		}
	}
	vpNext := after.DeepCopy()
	vpNext.Generation = 4
	checkGeneration("VPOL", func() { ctrl.updateVP(before, vpNext) })
	mpNext := mpolNew.DeepCopy()
	mpNext.Generation = 4
	checkGeneration("MPOL", func() { ctrl.updateMP(mpolOld, mpNext) })
	ivpNext := ivpolNew.DeepCopy()
	ivpNext.Generation = 4
	checkGeneration("IVPOL", func() { ctrl.updateIVP(ivpolOld, ivpNext) })
}

func TestNeedsReconcile_CELPolicyEffectiveRevision(t *testing.T) {
	const namespace, uid, hash = "default", "pod-uid-123", "stable-hash"

	before := engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy", Generation: 2, ResourceVersion: "100"},
	})
	report := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name: uid, Namespace: namespace,
			Labels: map[string]string{
				reportutils.LabelResourceHash:   hash,
				reportutils.PolicyLabel(before): reportutils.PolicyRevision(before),
			},
			Annotations: map[string]string{
				annotationLastScanTime: time.Now().Format(time.RFC3339),
			},
		},
	}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})
	if err := indexer.Add(report); err != nil {
		t.Fatal(err)
	}
	ctrl := &controller{
		forceDelay: time.Hour,
		bgscanrLister: cache.NewGenericLister(
			indexer,
			schema.GroupVersionResource{Group: "reports.kyverno.io", Version: "v1", Resource: "ephemeralreports"}.GroupResource(),
		),
	}
	metadataOnly := engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "policy", Generation: 2, ResourceVersion: "101",
			Annotations: map[string]string{"example.com/note": "updated"},
		},
	})
	_, needs, _, err := ctrl.needsReconcile(namespace, uid, hash, nil, nil, metadataOnly)
	if err != nil || needs {
		t.Fatalf("metadata-only update invalidated unchanged report: needs=%v err=%v", needs, err)
	}

	specChanged := engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy", Generation: 3, ResourceVersion: "102"},
	})
	_, needs, full, err := ctrl.needsReconcile(namespace, uid, hash, nil, nil, specChanged)
	if err != nil || !needs || full {
		t.Fatalf("new policy generation must cause partial scan: needs=%v full=%v err=%v", needs, full, err)
	}

	// Reports created before this fix store the raw Kubernetes resourceVersion.
	// A legacy label must be reconciled once into the semantic revision format.
	report.Labels[reportutils.PolicyLabel(before)] = "100"
	if err := indexer.Update(report); err != nil {
		t.Fatal(err)
	}
	_, needs, full, err = ctrl.needsReconcile(namespace, uid, hash, nil, nil, metadataOnly)
	if err != nil || !needs || full {
		t.Fatalf("legacy report must be upgraded through partial scan: needs=%v full=%v err=%v", needs, full, err)
	}
	report.Labels[reportutils.PolicyLabel(before)] = reportutils.PolicyRevision(metadataOnly)
	if err := indexer.Update(report); err != nil {
		t.Fatal(err)
	}
	_, needs, _, err = ctrl.needsReconcile(namespace, uid, hash, nil, nil, metadataOnly)
	if err != nil || needs {
		t.Fatalf("upgraded revision must converge without repeated work: needs=%v err=%v", needs, err)
	}
}

func TestNeedsReconcile_CELExceptionRevision(t *testing.T) {
	const namespace, uid, hash = "default", "pod-with-exception", "same-hash"
	policy := engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy", UID: "policy-uid", Generation: 2},
	})
	first := &policiesv1beta1.PolicyException{ObjectMeta: metav1.ObjectMeta{
		Namespace: namespace, Name: "exception", UID: "exception-uid", ResourceVersion: "10",
	}}
	first.Spec.PolicyRefs = []policiesv1beta1.PolicyRef{{Kind: "ValidatingPolicy", Name: policy.GetName()}}
	exceptionLabel := celExceptionPolicyLabel(policy)
	report := &reportsv1.EphemeralReport{ObjectMeta: metav1.ObjectMeta{
		Namespace: namespace, Name: uid,
		Labels: map[string]string{
			reportutils.LabelResourceHash:   hash,
			reportutils.PolicyLabel(policy): reportutils.PolicyRevision(policy),
		},
		Annotations: map[string]string{annotationLastScanTime: time.Now().Format(time.RFC3339)},
	}}
	report.Labels[exceptionLabel] = celExceptionRevision([]*policiesv1beta1.PolicyException{first})
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	if err := indexer.Add(report); err != nil {
		t.Fatal(err)
	}
	ctrl := &controller{forceDelay: time.Hour, exceptionNamespace: "*",
		bgscanrLister: cache.NewGenericLister(indexer,
			schema.GroupVersionResource{Group: "reports.kyverno.io", Version: "v1", Resource: "ephemeralreports"}.GroupResource()),
	}
	check := func(name string, exceptions []*policiesv1beta1.PolicyException, want bool) {
		t.Helper()
		_, needs, full, err := ctrl.needsReconcileWithExceptions(namespace, uid, hash, nil, nil, exceptions, policy)
		if err != nil || needs != want || full {
			t.Fatalf("%s: needs=%v full=%v err=%v (want needs=%v)", name, needs, full, err, want)
		}
	}
	check("unchanged exceptions", []*policiesv1beta1.PolicyException{first}, false)
	second := first.DeepCopy()
	second.ResourceVersion = "11"
	check("updated exception", []*policiesv1beta1.PolicyException{second}, true)
	check("deleted exception", nil, true)
	// Emulate successful partial reconcile persisting the new exception label.
	report.Labels[exceptionLabel] = celExceptionRevision([]*policiesv1beta1.PolicyException{second})
	if err := indexer.Update(report); err != nil {
		t.Fatal(err)
	}
	check("updated exception already processed", []*policiesv1beta1.PolicyException{second}, false)

	unrelated := second.DeepCopy()
	unrelated.Name = "unrelated"
	unrelated.Spec.PolicyRefs[0].Name = "another-policy"
	check("unrelated exception added", []*policiesv1beta1.PolicyException{second, unrelated}, false)

	// A report-wide legacy fingerprint cannot identify which old CEL results
	// depended on an exception. Reconcile once even when no exception remains.
	delete(report.Labels, exceptionLabel)
	report.Labels[celExceptionRevisionLabel] = celExceptionRevision([]*policiesv1beta1.PolicyException{second})
	if err := indexer.Update(report); err != nil {
		t.Fatal(err)
	}
	check("legacy revision with removed exception", nil, true)
}
