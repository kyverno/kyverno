package generator

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

func newURLister(t *testing.T, urs ...*kyvernov2.UpdateRequest) kyvernov2listers.UpdateRequestLister {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, ur := range urs {
		if err := indexer.Add(ur); err != nil {
			t.Fatalf("failed to add UpdateRequest to indexer: %v", err)
		}
	}
	return kyvernov2listers.NewUpdateRequestLister(indexer)
}

func newUR(namespace, name string) *kyvernov2.UpdateRequest {
	return &kyvernov2.UpdateRequest{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
}

// failingURLister always fails to list UpdateRequests.
type failingURLister struct {
	kyvernov2listers.UpdateRequestLister
}

func (failingURLister) List(labels.Selector) ([]*kyvernov2.UpdateRequest, error) {
	return nil, errors.New("list failed")
}

func TestGenerate_BelowThreshold_CreatesUpdateRequest(t *testing.T) {
	ctx := context.Background()

	// Create a mock ConfigMap and load it into the configuration
	cfg := config.NewDefaultConfiguration(false)
	mockConfigMap := &corev1.ConfigMap{
		Data: map[string]string{
			"updateRequestThreshold": "10",
		},
	}
	cfg.Load(mockConfigMap)

	urLister := newURLister(t, newUR(config.KyvernoNamespace(), "ur-1"))

	kyvernoClient := fake.NewSimpleClientset()

	gen := NewUpdateRequestGenerator(cfg, urLister)

	resource := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ur",
		},
	}

	_, err := gen.Generate(ctx, kyvernoClient, resource, logr.Discard())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the UpdateRequest was actually created
	createdUR, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(ctx, "test-ur", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get created UpdateRequest: %v", err)
	}
	if createdUR == nil {
		t.Fatalf("expected UpdateRequest to be created, but it was not found")
	}
}

func TestGenerate_AtThreshold_SkipsCreation(t *testing.T) {
	ctx := context.Background()

	cfg := config.NewDefaultConfiguration(false)
	mockConfigMap := &corev1.ConfigMap{
		Data: map[string]string{
			"updateRequestThreshold": "1",
		},
	}
	cfg.Load(mockConfigMap)

	urLister := newURLister(t, newUR(config.KyvernoNamespace(), "ur-1"))

	kyvernoClient := fake.NewSimpleClientset()
	gen := NewUpdateRequestGenerator(cfg, urLister)

	resource := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "should-not-be-created",
		},
	}

	result, err := gen.Generate(ctx, kyvernoClient, resource, logr.Discard())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected no UpdateRequest to be created when threshold is met or exceeded")
	}

	// Verify the UpdateRequest was NOT created
	urs, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list UpdateRequests: %v", err)
	}
	if len(urs.Items) > 0 {
		t.Fatalf("expected no UpdateRequests to be created, but found %d", len(urs.Items))
	}
}

func TestGenerate_ListerError_ReturnsError(t *testing.T) {
	ctx := context.Background()

	cfg := config.NewDefaultConfiguration(false)
	mockConfigMap := &corev1.ConfigMap{
		Data: map[string]string{
			"updateRequestThreshold": "10",
		},
	}
	cfg.Load(mockConfigMap)

	kyvernoClient := fake.NewSimpleClientset()

	gen := NewUpdateRequestGenerator(cfg, failingURLister{})

	resource := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ur",
		},
	}

	result, err := gen.Generate(ctx, kyvernoClient, resource, logr.Discard())

	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if result != nil {
		t.Fatalf("expected no UpdateRequest on error")
	}
}
func TestGenerate_CleanupEnabled_AppliesLabel(t *testing.T) {
	ctx := context.Background()

	cfg := config.NewDefaultConfiguration(false)
	mockConfigMap := &corev1.ConfigMap{
		Data: map[string]string{
			"updateRequestThreshold":     "10",
			"enableUpdateRequestCleanup": "true",
			"updateRequestCleanupTTL":    "1h",
		},
	}
	cfg.Load(mockConfigMap)

	urLister := newURLister(t)
	kyvernoClient := fake.NewSimpleClientset()
	gen := NewUpdateRequestGenerator(cfg, urLister)

	resource := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ur-ttl",
		},
	}

	_, err := gen.Generate(ctx, kyvernoClient, resource, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	createdUR, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(ctx, "test-ur-ttl", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get created UpdateRequest: %v", err)
	}

	if createdUR.Labels == nil || createdUR.Labels[kyverno.LabelCleanupTtl] != "1h" {
		t.Fatalf("expected label %s to be 1h, got %v", kyverno.LabelCleanupTtl, createdUR.Labels)
	}
}

func TestGenerate_CleanupDisabled_NoLabel(t *testing.T) {
	ctx := context.Background()

	cfg := config.NewDefaultConfiguration(false)
	mockConfigMap := &corev1.ConfigMap{
		Data: map[string]string{
			"updateRequestThreshold":     "10",
			"enableUpdateRequestCleanup": "false",
			"updateRequestCleanupTTL":    "1h", // Even with TTL, it shouldn't apply if cleanup is disabled
		},
	}
	cfg.Load(mockConfigMap)

	urLister := newURLister(t)
	kyvernoClient := fake.NewSimpleClientset()
	gen := NewUpdateRequestGenerator(cfg, urLister)

	resource := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ur-nottl",
		},
	}

	_, err := gen.Generate(ctx, kyvernoClient, resource, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if createdUR, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(ctx, "test-ur-nottl", metav1.GetOptions{}); err != nil {
		t.Fatalf("failed to get created UpdateRequest: %v", err)
	} else if _, present := createdUR.Labels[kyverno.LabelCleanupTtl]; present {
		t.Fatalf("expected no ttl label, got %v", createdUR.Labels[kyverno.LabelCleanupTtl])
	}
}

func TestGenerate_CleanupEnabled_InvalidTTL_NoLabel(t *testing.T) {
	ctx := context.Background()

	cfg := config.NewDefaultConfiguration(false)
	mockConfigMap := &corev1.ConfigMap{
		Data: map[string]string{
			"updateRequestThreshold":     "10",
			"enableUpdateRequestCleanup": "true",
			"updateRequestCleanupTTL":    "invalid", // This is the bad value
		},
	}
	cfg.Load(mockConfigMap)

	urLister := newURLister(t)
	kyvernoClient := fake.NewSimpleClientset()
	gen := NewUpdateRequestGenerator(cfg, urLister)

	resource := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ur-invalid-ttl",
		},
	}

	_, err := gen.Generate(ctx, kyvernoClient, resource, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	createdUR, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(ctx, "test-ur-invalid-ttl", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get created UpdateRequest: %v", err)
	}

	if _, ok := createdUR.Labels[kyverno.LabelCleanupTtl]; ok {
		t.Fatalf("expected no ttl label, got %v", createdUR.Labels[kyverno.LabelCleanupTtl])
	}
}

func TestGenerate_CountsFromCacheWithoutListingAPIServer(t *testing.T) {
	ctx := context.Background()

	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{Data: map[string]string{"updateRequestThreshold": "10"}})

	kyvernoClient := fake.NewSimpleClientset()
	gen := NewUpdateRequestGenerator(cfg, newURLister(t, newUR(config.KyvernoNamespace(), "ur-1")))

	for _, name := range []string{"ur-a", "ur-b", "ur-c"} {
		if _, err := gen.Generate(ctx, kyvernoClient, newUR("", name), logr.Discard()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	for _, action := range kyvernoClient.Actions() {
		if action.GetVerb() != "create" {
			t.Fatalf("expected only create calls to the API server, got %s %s", action.GetVerb(), action.GetResource().Resource)
		}
	}
	if got := len(kyvernoClient.Actions()); got != 3 {
		t.Fatalf("expected 3 create calls, got %d", got)
	}
}

func TestGenerate_ThresholdCountsUpdateRequestsInAllNamespaces(t *testing.T) {
	ctx := context.Background()

	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{Data: map[string]string{"updateRequestThreshold": "2"}})

	kyvernoClient := fake.NewSimpleClientset()
	urLister := newURLister(t, newUR(config.KyvernoNamespace(), "ur-1"), newUR("other", "ur-2"))
	gen := NewUpdateRequestGenerator(cfg, urLister)

	result, err := gen.Generate(ctx, kyvernoClient, newUR("", "should-not-be-created"), logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected no UpdateRequest to be created when the cached count reaches the threshold")
	}
	if got := len(kyvernoClient.Actions()); got != 0 {
		t.Fatalf("expected no API calls, got %d", got)
	}
}

func TestGenerate_ThresholdIsBestEffortWithCacheLag(t *testing.T) {
	ctx := context.Background()

	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{Data: map[string]string{"updateRequestThreshold": "5"}})

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, name := range []string{"cached-1", "cached-2", "cached-3", "cached-4"} {
		if err := indexer.Add(newUR(config.KyvernoNamespace(), name)); err != nil {
			t.Fatalf("failed to add UpdateRequest to indexer: %v", err)
		}
	}
	kyvernoClient := fake.NewSimpleClientset()
	gen := NewUpdateRequestGenerator(cfg, kyvernov2listers.NewUpdateRequestLister(indexer))

	// The cache still holds 4 UpdateRequests, so both calls are below the threshold.
	var created []*kyvernov2.UpdateRequest
	for _, name := range []string{"ur-a", "ur-b"} {
		ur, err := gen.Generate(ctx, kyvernoClient, newUR("", name), logr.Discard())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ur == nil {
			t.Fatalf("expected %s to be created while the cache is behind", name)
		}
		created = append(created, ur)
	}

	// After the cache catches up, it holds 6 UpdateRequests, so the next call is skipped.
	for _, ur := range created {
		if err := indexer.Add(ur); err != nil {
			t.Fatalf("failed to add UpdateRequest to indexer: %v", err)
		}
	}
	ur, err := gen.Generate(ctx, kyvernoClient, newUR("", "ur-c"), logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ur != nil {
		t.Fatalf("expected ur-c to be skipped once the cache reaches the threshold")
	}
}
