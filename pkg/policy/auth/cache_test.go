package auth

import (
	"context"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

// countingSAR wraps a fake kube clientset with a SubjectAccessReview reactor
// that records every request and allows all of them, so a test can assert how
// many SubjectAccessReviews were actually issued.
func countingSAR() (*kubefake.Clientset, func() int) {
	kube := kubefake.NewSimpleClientset()
	var mu sync.Mutex
	count := 0
	kube.PrependReactor("create", "subjectaccessreviews", func(action clienttesting.Action) (bool, runtime.Object, error) {
		mu.Lock()
		count++
		mu.Unlock()
		review := action.(clienttesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		review.Status.Allowed = true
		return true, review, nil
	})
	return kube, func() int {
		mu.Lock()
		defer mu.Unlock()
		return count
	}
}

// TestAuth_CollapsesKindSpellings drives the full Auth.CanI -> NewCanI ->
// RunAccessCheck path so GVK->GVR resolution runs before the cache. "ConfigMap"
// and "v1/ConfigMap" resolve to the same GVR, so the second call is a cache hit
// and issues no additional SubjectAccessReview — the key benefit of caching at
// the checker layer instead of on the raw kind string.
func TestAuth_CollapsesKindSpellings(t *testing.T) {
	kube, requests := countingSAR()
	client := dclient.NewFakeClientWithDisco(nil, kube, dclient.NewFakeDiscoveryClient(nil))
	a := NewAuth(client, "reports-sa", logr.Discard(), checker.NewResultCache())
	ctx := context.Background()

	ok1, _, err := a.CanI(ctx, []string{"get"}, "ConfigMap", "", "", "")
	assert.NoError(t, err)
	assert.True(t, ok1)

	ok2, _, err := a.CanI(ctx, []string{"get"}, "v1/ConfigMap", "", "", "")
	assert.NoError(t, err)
	assert.True(t, ok2)

	assert.Equal(t, 1, requests())
}

// TestAuth_SharedCacheDeduplicatesAcrossCheckers verifies the main motivation of
// the PR: a single cache shared between two Auth instances for the same subject
// (as the mutate/validate/generate factories build) issues each distinct
// (resource, verb) SubjectAccessReview only once.
func TestAuth_SharedCacheDeduplicatesAcrossCheckers(t *testing.T) {
	kube, requests := countingSAR()
	client := dclient.NewFakeClientWithDisco(nil, kube, dclient.NewFakeDiscoveryClient(nil))
	cache := checker.NewResultCache()
	first := NewAuth(client, "reports-sa", logr.Discard(), cache)
	second := NewAuth(client, "reports-sa", logr.Discard(), cache)
	ctx := context.Background()

	_, _, _ = first.CanI(ctx, []string{"get", "list"}, "ConfigMap", "", "", "")
	_, _, _ = second.CanI(ctx, []string{"get", "list"}, "ConfigMap", "", "", "")

	// get + list = two distinct checks; the second Auth reuses both from cache.
	assert.Equal(t, 2, requests())
}
