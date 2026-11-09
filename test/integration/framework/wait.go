package framework

import (
	"context"
	"testing"
	"time"

	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

// WaitForVpols blocks until the provider has compiled every named ValidatingPolicy or
// NamespacedValidatingPolicy. A name is "name" for a cluster-scoped policy and "namespace/name"
// for a namespaced one.
//
// Wait by name rather than by counting compiled entries: the provider keeps one entry per autogen
// target (a Pod policy also yields entries for Deployments, CronJobs and so on), so a count is
// reached before every policy has been compiled.
func WaitForVpols(t *testing.T, provider vpolengine.Provider, names ...string) {
	t.Helper()
	waitForPolicies(t, names, func(ctx context.Context) ([]metav1.Object, error) {
		policies, err := provider.Fetch(ctx)
		objs := make([]metav1.Object, 0, len(policies))
		for _, p := range policies {
			objs = append(objs, p.Policy)
		}
		return objs, err
	})
}

// WaitForMpols is WaitForVpols for MutatingPolicy and NamespacedMutatingPolicy.
func WaitForMpols(t *testing.T, provider mpolengine.Provider, names ...string) {
	t.Helper()
	waitForPolicies(t, names, func(ctx context.Context) ([]metav1.Object, error) {
		policies := provider.Fetch(ctx, false)
		objs := make([]metav1.Object, 0, len(policies))
		for _, p := range policies {
			objs = append(objs, p.Policy)
		}
		return objs, nil
	})
}

// WaitForIvpols is WaitForVpols for ImageValidatingPolicy and NamespacedImageValidatingPolicy.
func WaitForIvpols(t *testing.T, provider ivpolengine.Provider, names ...string) {
	t.Helper()
	waitForPolicies(t, names, func(ctx context.Context) ([]metav1.Object, error) {
		policies, err := provider.Fetch(ctx)
		objs := make([]metav1.Object, 0, len(policies))
		for _, p := range policies {
			objs = append(objs, p.Policy)
		}
		return objs, err
	})
}

func waitForPolicies(t *testing.T, names []string, fetch func(context.Context) ([]metav1.Object, error)) {
	t.Helper()
	if len(names) == 0 {
		t.Fatal("no policy names to wait for")
	}
	var missing []string
	var lastErr error
	err := wait.PollUntilContextTimeout(context.Background(), 100*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		policies, err := fetch(ctx)
		if err != nil {
			lastErr = err
			return false, nil
		}
		compiled := make(map[string]bool, len(policies))
		for _, p := range policies {
			compiled[cache.MetaObjectToName(p).String()] = true
		}
		missing = missing[:0]
		for _, name := range names {
			if !compiled[name] {
				missing = append(missing, name)
			}
		}
		return len(missing) == 0, nil
	})
	if err != nil {
		if len(missing) == 0 {
			missing = names
		}
		t.Fatalf("policies %v not compiled in time (last fetch error: %v): %v", missing, lastErr, err)
	}
}
