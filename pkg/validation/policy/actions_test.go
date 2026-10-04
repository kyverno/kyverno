package policy

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/policy/auth"
	"github.com/stretchr/testify/assert"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached/memory"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// Test_validateActions_NilRule verifies a nil rule is a no-op.
func Test_validateActions_NilRule(t *testing.T) {
	warnings, err := validateActions(0, nil, nil, true, "", "", nil)
	assert.Nil(t, err)
	assert.Nil(t, warnings)
}

// Test_validateActions_GenerateSameKind verifies a generate rule whose target
// kind matches its own match kind is rejected.
func Test_validateActions_GenerateSameKind(t *testing.T) {
	rule := &kyvernov1.Rule{
		Name: "test-rule",
		MatchResources: kyvernov1.MatchResources{
			Any: kyvernov1.ResourceFilters{
				{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{"ConfigMap"},
					},
				},
			},
		},
		Generation: &kyvernov1.Generation{
			GeneratePattern: kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{
					Kind: "ConfigMap",
				},
			},
		},
	}
	rule.MatchResources.Kinds = []string{"ConfigMap"}

	warnings, err := validateActions(0, rule, nil, true, "", "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "generation kind and match resource kind should not be the same")
	assert.Nil(t, warnings)
}

// Test_validateActions_GenerateMockSuccess verifies a valid generate rule
// passes in mock mode.
func Test_validateActions_GenerateMockSuccess(t *testing.T) {
	rule := &kyvernov1.Rule{
		Name: "test-rule",
		Generation: &kyvernov1.Generation{
			GeneratePattern: kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{
					Kind: "ConfigMap",
					Name: "test-cm",
				},
			},
		},
	}

	warnings, err := validateActions(0, rule, nil, true, "", "", nil)
	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

// Test_validateActions_LiveAuthError exercises the non-mock caller: a real
// dclient whose SubjectAccessReview requests fail. It drives validateActions
// through the live (reportsSA, non-nil cache) path and asserts the
// authorization error is surfaced rather than swallowed. The cache tests cover
// the decorator in isolation; this covers the caller wiring.
func Test_validateActions_LiveAuthError(t *testing.T) {
	kube := kubefake.NewSimpleClientset()
	kube.PrependReactor("create", "subjectaccessreviews", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("authorization request failed")
	})
	client := dclient.NewFakeClientWithDisco(
		dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		kube,
		dclient.NewFakeDiscoveryClient(nil),
	)

	rule := &kyvernov1.Rule{
		Name: "deny-rule",
		MatchResources: kyvernov1.MatchResources{
			ResourceDescription: kyvernov1.ResourceDescription{
				Kinds: []string{"ConfigMap"},
			},
		},
		Validation: &kyvernov1.Validation{
			Deny: &kyvernov1.Deny{},
		},
	}

	_, err := validateActions(0, rule, client, false, "background-sa", "reports-sa", auth.NewResultCache())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authorization request failed")
}

// cachedDisco wraps the fake IDiscovery with a real CachedDiscoveryInterface so
// the full Validate() path (which calls discovery.ServerPreferredResources on
// it) works; all other discovery lookups delegate to the embedded fake.
type cachedDisco struct {
	dclient.IDiscovery
	cached discovery.CachedDiscoveryInterface
}

func (d cachedDisco) CachedDiscoveryInterface() discovery.CachedDiscoveryInterface {
	return d.cached
}

// sarCountingClient returns a live (non-mock) dclient whose SubjectAccessReview
// requests are approved and counted per (subject, resource, verb) tuple.
func sarCountingClient(t *testing.T) (dclient.Interface, func() map[string]int) {
	t.Helper()
	var mu sync.Mutex
	counts := map[string]int{}

	kube := kubefake.NewSimpleClientset()
	// Populate discovery so Validate()'s discovery.ServerPreferredResources has
	// at least one API group to walk (an empty fake discovery panics there).
	// Resources live on the FakeDiscovery, not on the clientset.
	kube.Discovery().(*discoveryfake.FakeDiscovery).Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "configmaps", Namespaced: true, Kind: "ConfigMap"},
				{Name: "secrets", Namespaced: true, Kind: "Secret"},
			},
		},
	}
	kube.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		sar := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		ra := sar.Spec.ResourceAttributes
		key := sar.Spec.User + "|" + ra.Resource + "|" + ra.Verb
		mu.Lock()
		counts[key]++
		mu.Unlock()
		return true, &authorizationv1.SubjectAccessReview{
			Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true},
		}, nil
	})

	disco := cachedDisco{
		IDiscovery: dclient.NewFakeDiscoveryClient(nil),
		cached:     memory.NewMemCacheClient(kube.Discovery()),
	}
	client := dclient.NewFakeClientWithDisco(
		dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		kube,
		disco,
	)
	snapshot := func() map[string]int {
		mu.Lock()
		defer mu.Unlock()
		out := make(map[string]int, len(counts))
		for k, v := range counts {
			out[k] = v
		}
		return out
	}
	return client, snapshot
}

// Test_validateActions_SharesAuthCacheAcrossRules is the integration regression
// test for the shared-cache contract. It mirrors Validate()'s per-pass loop: a
// single ResultCache is threaded through validateActions for every rule, so a
// (subject, kind, verb) authorization tuple is checked at most once across the
// whole pass — even when several rules match the same kind and even across
// different action factories (validate and mutate both run the reports check).
// If a future change recreates the cache per rule or drops the cache threading
// into any factory, the overlapping ConfigMap checks would re-issue their
// SubjectAccessReviews and a per-tuple count would exceed one.
func Test_validateActions_SharesAuthCacheAcrossRules(t *testing.T) {
	client, counts := sarCountingClient(t)
	cache := auth.NewResultCache() // one cache for the whole pass, as Validate() does

	cm := kyvernov1.MatchResources{ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"ConfigMap"}}}
	secret := kyvernov1.MatchResources{ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Secret"}}}

	rules := []*kyvernov1.Rule{
		// validate on ConfigMap
		{Name: "cm-validate-a", MatchResources: cm, Validation: &kyvernov1.Validation{Deny: &kyvernov1.Deny{}}},
		// a second validate on ConfigMap — overlaps the first (same factory)
		{Name: "cm-validate-b", MatchResources: cm, Validation: &kyvernov1.Validation{Deny: &kyvernov1.Deny{}}},
		// mutate on ConfigMap — its reports check overlaps the validate rules (different factory)
		{Name: "cm-mutate", MatchResources: cm, Mutation: &kyvernov1.Mutation{PatchesJSON6902: "dummy"}},
		// validate on Secret — a distinct kind
		{Name: "secret-validate", MatchResources: secret, Validation: &kyvernov1.Validation{Deny: &kyvernov1.Deny{}}},
	}

	for i, rule := range rules {
		_, err := validateActions(i, rule, client, false, "background-sa", "reports-sa", cache)
		assert.NoErrorf(t, err, "rule %s", rule.Name)
	}

	// Reports auth checks "get","list","watch" per kind under reportsSA. Assert
	// the exact tuple set so a check misrouted to another resource or subject is
	// caught, not just a missing one.
	expected := map[string]int{
		"reports-sa|configmaps|get":   1,
		"reports-sa|configmaps|list":  1,
		"reports-sa|configmaps|watch": 1,
		"reports-sa|secrets|get":      1,
		"reports-sa|secrets|list":     1,
		"reports-sa|secrets|watch":    1,
	}
	assert.Equal(t, expected, counts())
}

// Test_Validate_SharesAuthCacheAcrossRules drives the real exported Validate()
// with the counting client. Unlike the validateActions test above (which
// supplies its own cache), this proves Validate() itself creates one
// ResultCache for the whole pass and threads it into every rule's factory: a
// (subject, kind, verb) tuple that recurs across rules is requested only once.
// A regression that recreates the cache per rule, or fails to pass it to
// validateActions, would make the duplicated ConfigMap tuples count more than
// once and fail this test.
func Test_Validate_SharesAuthCacheAcrossRules(t *testing.T) {
	// ConfigMap and Secret are registered in the fake discovery and are not Pod
	// controllers, so autogen leaves these rules unexpanded. Two rules overlap
	// on ConfigMap to exercise cross-rule deduplication within one pass.
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {"name": "require-labels"},
		"spec": {
			"validationFailureAction": "Audit",
			"background": false,
			"rules": [
				{
					"name": "configmap-a",
					"match": {"resources": {"kinds": ["ConfigMap"]}},
					"validate": {"message": "label app is required", "pattern": {"metadata": {"labels": {"app": "?*"}}}}
				},
				{
					"name": "configmap-b",
					"match": {"resources": {"kinds": ["ConfigMap"]}},
					"validate": {"message": "label team is required", "pattern": {"metadata": {"labels": {"team": "?*"}}}}
				},
				{
					"name": "secret-c",
					"match": {"resources": {"kinds": ["Secret"]}},
					"validate": {"message": "label app is required", "pattern": {"metadata": {"labels": {"app": "?*"}}}}
				}
			]
		}
	}`)

	var policy *kyvernov1.ClusterPolicy
	assert.NoError(t, json.Unmarshal(rawPolicy, &policy))

	client, counts := sarCountingClient(t)

	_, err := Validate(policy, nil, client, false, "background-sa", "reports-sa")
	assert.NoError(t, err)

	expected := map[string]int{
		"reports-sa|configmaps|get":   1,
		"reports-sa|configmaps|list":  1,
		"reports-sa|configmaps|watch": 1,
		"reports-sa|secrets|get":      1,
		"reports-sa|secrets|list":     1,
		"reports-sa|secrets|watch":    1,
	}
	assert.Equal(t, expected, counts())
}
