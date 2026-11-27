package utils

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func newTestValidatingPolicy(name, rv string) *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: rv,
		},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{"CREATE"},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
			},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "object.metadata.name != 'forbidden'",
			}},
		},
	}
}

func newTestMutatingPolicy(name, rv string) *policiesv1beta1.MutatingPolicy {
	return &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: rv,
		},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{"CREATE"},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `[JSONPatch{op: "add", path: "/metadata/labels/test", value: "cached"}]`,
				},
			}},
		},
	}
}

func newTestImageValidatingPolicy(name, rv string) *policiesv1beta1.ImageValidatingPolicy {
	return &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: rv,
		},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{"CREATE"},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{
				{Glob: "ghcr.io/*"},
			},
			Attestors: []policiesv1beta1.Attestor{
				{
					Name: "notary",
					Notary: &policiesv1beta1.Notary{
						Certs: &policiesv1beta1.StringOrExpression{Value: "cert"},
					},
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "images.containers.map(image, verifyImageSignatures(image, [attestors.notary])).all(e, e > 0)"},
			},
		},
	}
}

func newTestPolicyException(name, namespace, rv string, targetKind, targetName string) *policiesv1beta1.PolicyException {
	return &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			ResourceVersion: rv,
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{
				{
					Kind: targetKind,
					Name: targetName,
				},
			},
		},
	}
}

func TestProviderCache_ValidatingPolicy_HitAndMiss(t *testing.T) {
	cache := NewProviderCache()
	var compileCount int
	HookVpolCompiled = func() { compileCount++ }
	defer func() { HookVpolCompiled = nil }()

	policy := newTestValidatingPolicy("test-vpol", "100")

	// 1. Initial lookup -> cache miss, compiles
	provider1, err := cache.GetValidatingPolicyProvider(policy, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	require.NotNil(t, provider1)
	assert.Equal(t, 1, compileCount, "first lookup must compile policy")

	// 2. Second lookup with same policy and revision -> cache hit, does not recompile
	provider2, err := cache.GetValidatingPolicyProvider(policy, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	require.NotNil(t, provider2)
	assert.Equal(t, 1, compileCount, "second lookup with same revision must be a cache hit")

	// 3. Update ResourceVersion -> cache miss, recompiles
	policyUpdated := newTestValidatingPolicy("test-vpol", "101")
	provider3, err := cache.GetValidatingPolicyProvider(policyUpdated, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	require.NotNil(t, provider3)
	assert.Equal(t, 2, compileCount, "updating policy resourceVersion must trigger recompilation")

	// 4. Repeated lookup with new ResourceVersion -> cache hit
	provider4, err := cache.GetValidatingPolicyProvider(policyUpdated, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	require.NotNil(t, provider4)
	assert.Equal(t, 2, compileCount, "subsequent lookup with updated revision must be a cache hit")
}

func TestProviderCache_ValidatingPolicy_ExceptionChanges(t *testing.T) {
	cache := NewProviderCache()
	var compileCount int
	HookVpolCompiled = func() { compileCount++ }
	defer func() { HookVpolCompiled = nil }()

	policy := newTestValidatingPolicy("test-vpol-exc", "100")
	polex1 := newTestPolicyException("polex-1", "default", "10", "ValidatingPolicy", "test-vpol-exc")
	polexUnrelated := newTestPolicyException("polex-other", "default", "50", "ValidatingPolicy", "other-policy")

	// 1. Initial compile with polex1
	_, err := cache.GetValidatingPolicyProvider(policy, []*policiesv1beta1.PolicyException{polex1}, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	assert.Equal(t, 1, compileCount)

	// 2. Lookup with same polex1 -> hit
	_, err = cache.GetValidatingPolicyProvider(policy, []*policiesv1beta1.PolicyException{polex1}, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	assert.Equal(t, 1, compileCount)

	// 3. Lookup with polex1 and unrelated exception -> hit (unrelated exception does not affect key)
	_, err = cache.GetValidatingPolicyProvider(policy, []*policiesv1beta1.PolicyException{polex1, polexUnrelated}, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	assert.Equal(t, 1, compileCount, "unrelated policy exception must not invalidate cache")

	// 4. Modify matching exception resourceVersion -> recompile
	polex1Updated := newTestPolicyException("polex-1", "default", "11", "ValidatingPolicy", "test-vpol-exc")
	_, err = cache.GetValidatingPolicyProvider(policy, []*policiesv1beta1.PolicyException{polex1Updated}, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	assert.Equal(t, 2, compileCount, "matching exception resourceVersion update must trigger recompilation")

	// 5. Add second matching exception -> recompile
	polex2 := newTestPolicyException("polex-2", "default", "20", "ValidatingPolicy", "test-vpol-exc")
	_, err = cache.GetValidatingPolicyProvider(policy, []*policiesv1beta1.PolicyException{polex1Updated, polex2}, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	assert.Equal(t, 3, compileCount, "adding matching exception must trigger recompilation")

	// 6. Remove matching exception -> recompile
	_, err = cache.GetValidatingPolicyProvider(policy, []*policiesv1beta1.PolicyException{polex1Updated}, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	assert.Equal(t, 4, compileCount, "removing matching exception must trigger recompilation")
}

func TestProviderCache_ValidatingPolicy_ExpiredExceptionInvalidatesCache(t *testing.T) {
	cache := NewProviderCache()
	var compileCount int
	HookVpolCompiled = func() { compileCount++ }
	defer func() { HookVpolCompiled = nil }()

	policy := newTestValidatingPolicy("test-vpol-exp", "100")
	future := metav1.NewTime(time.Now().Add(time.Hour))
	polex := newTestPolicyException("polex-exp", "default", "10", "ValidatingPolicy", "test-vpol-exp")
	polex.Spec.ExpiresAt = &future

	pod := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name":      "forbidden",
				"namespace": "default",
			},
		},
	}
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	req := celengine.Request(
		libs.GetLibsCtx(),
		pod.GroupVersionKind(),
		podGVR,
		"",
		pod.GetName(),
		pod.GetNamespace(),
		admissionv1.Create,
		authenticationv1.UserInfo{},
		&pod,
		nil,
		false,
		nil,
	)

	// 1. Active exception -> compiles with exception.
	p1, err := cache.GetValidatingPolicyProvider(policy, []*policiesv1beta1.PolicyException{polex}, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	assert.Equal(t, 1, compileCount)
	resp1, err := vpolengine.NewEngine(p1, func(string) *corev1.Namespace { return nil }, matching.NewMatcher()).Handle(t.Context(), req, nil)
	require.NoError(t, err)
	require.NotEmpty(t, resp1.Policies)
	require.Len(t, resp1.Policies[0].Rules, 1)
	assert.NotEmpty(t, resp1.Policies[0].Rules[0].Exceptions())

	// 2. Cache hit while still active.
	_, err = cache.GetValidatingPolicyProvider(policy, []*policiesv1beta1.PolicyException{polex}, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	assert.Equal(t, 1, compileCount)

	// 3. Exception expires without a ResourceVersion bump -> must invalidate and recompile without the expired exception.
	past := metav1.NewTime(time.Now().Add(-time.Hour))
	polexExpired := polex.DeepCopy()
	polexExpired.Spec.ExpiresAt = &past

	p2, err := cache.GetValidatingPolicyProvider(policy, []*policiesv1beta1.PolicyException{polexExpired}, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	assert.Equal(t, 2, compileCount, "expired PolicyException must invalidate cache without a ResourceVersion bump")
	resp2, err := vpolengine.NewEngine(p2, func(string) *corev1.Namespace { return nil }, matching.NewMatcher()).Handle(t.Context(), req, nil)
	require.NoError(t, err)
	require.NotEmpty(t, resp2.Policies)
	require.Len(t, resp2.Policies[0].Rules, 1)
	assert.Empty(t, resp2.Policies[0].Rules[0].Exceptions(), "recompiled provider must not include expired exception")
}

func TestProviderCache_ValidatingPolicy_ExceptionScopeNamespaceIsolation(t *testing.T) {
	cache := NewProviderCache()
	var compileCount int
	HookVpolCompiled = func() { compileCount++ }
	defer func() { HookVpolCompiled = nil }()

	policyA := &policiesv1beta1.NamespacedValidatingPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind: "NamespacedValidatingPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "shared-name",
			Namespace:       "ns-a",
			ResourceVersion: "100",
		},
		Spec: newTestValidatingPolicy("shared-name", "100").Spec,
	}
	polexA := newTestPolicyException("polex-a", "ns-a", "10", "NamespacedValidatingPolicy", "shared-name")
	polexB := newTestPolicyException("polex-b", "ns-b", "20", "NamespacedValidatingPolicy", "shared-name")

	_, err := cache.GetValidatingPolicyProvider(policyA, []*policiesv1beta1.PolicyException{polexA}, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	assert.Equal(t, 1, compileCount)

	// Adding or updating a foreign-namespace exception for a same-named policy must not churn ns-a's cache key.
	_, err = cache.GetValidatingPolicyProvider(policyA, []*policiesv1beta1.PolicyException{polexA, polexB}, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	assert.Equal(t, 1, compileCount, "foreign-namespace exception must not invalidate namespaced policy cache")
}

func TestProviderCache_MutatingPolicy_HitAndMiss(t *testing.T) {
	cache := NewProviderCache()
	var compileCount int
	HookMpolCompiled = func() { compileCount++ }
	defer func() { HookMpolCompiled = nil }()

	policy := newTestMutatingPolicy("test-mpol", "200")

	// 1. Initial lookup -> cache miss
	provider1, err := cache.GetMutatingPolicyProvider(policy, nil, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	require.NotNil(t, provider1)
	assert.Equal(t, 1, compileCount)

	// 2. Second lookup -> cache hit
	provider2, err := cache.GetMutatingPolicyProvider(policy, nil, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	require.NotNil(t, provider2)
	assert.Equal(t, 1, compileCount)

	// 3. Update ResourceVersion -> recompile
	policyUpdated := newTestMutatingPolicy("test-mpol", "201")
	provider3, err := cache.GetMutatingPolicyProvider(policyUpdated, nil, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	require.NotNil(t, provider3)
	assert.Equal(t, 2, compileCount)
}

func TestProviderCache_ImageValidatingPolicy_HitAndMiss(t *testing.T) {
	cache := NewProviderCache()
	var compileCount int
	HookIvpolCompiled = func() { compileCount++ }
	defer func() { HookIvpolCompiled = nil }()

	policy := newTestImageValidatingPolicy("test-ivpol", "300")

	// 1. Initial lookup -> cache miss
	provider1, err := cache.GetImageValidatingPolicyProvider(policy, nil, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	require.NotNil(t, provider1)
	assert.Equal(t, 1, compileCount)

	// 2. Second lookup -> cache hit
	provider2, err := cache.GetImageValidatingPolicyProvider(policy, nil, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	require.NotNil(t, provider2)
	assert.Equal(t, 1, compileCount)

	// 3. Update ResourceVersion -> recompile
	policyUpdated := newTestImageValidatingPolicy("test-ivpol", "301")
	provider3, err := cache.GetImageValidatingPolicyProvider(policyUpdated, nil, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	require.NotNil(t, provider3)
	assert.Equal(t, 2, compileCount)
}

func TestProviderCache_EvictAndClear(t *testing.T) {
	cache := NewProviderCache()
	var vpolCompileCount int
	HookVpolCompiled = func() { vpolCompileCount++ }
	defer func() { HookVpolCompiled = nil }()

	policy := newTestValidatingPolicy("test-vpol-evict", "1")
	_, err := cache.GetValidatingPolicyProvider(policy, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	assert.Equal(t, 1, vpolCompileCount)

	// Evict specific policy
	cache.Evict(policy.GetKind(), policy.GetNamespace(), policy.GetName())

	// Next lookup should recompile
	_, err = cache.GetValidatingPolicyProvider(policy, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	assert.Equal(t, 2, vpolCompileCount, "evicted policy must be recompiled on next lookup")

	// Clear whole cache
	cache.Clear()
	_, err = cache.GetValidatingPolicyProvider(policy, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	assert.Equal(t, 3, vpolCompileCount, "clearing cache must force recompilation")
}

func TestProviderCache_Concurrent(t *testing.T) {
	cache := NewProviderCache()
	policy := newTestValidatingPolicy("concurrent-vpol", "1")

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				p, err := cache.GetValidatingPolicyProvider(policy, nil, celengine.ExceptionScopeAll)
				assert.NoError(t, err)
				assert.NotNil(t, p)
			}
		}()
	}

	wg.Wait()
}

func TestProviderCache_ConcurrentMissesCompileOnce(t *testing.T) {
	const goroutines = 20

	t.Run("ValidatingPolicy", func(t *testing.T) {
		cache := NewProviderCache()
		var compileCount atomic.Int32
		HookVpolCompiled = func() { compileCount.Add(1) }
		defer func() { HookVpolCompiled = nil }()

		policy := newTestValidatingPolicy("concurrent-miss-vpol", "1")
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(goroutines)

		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				<-start
				p, err := cache.GetValidatingPolicyProvider(policy, nil, celengine.ExceptionScopeAll)
				assert.NoError(t, err)
				assert.NotNil(t, p)
			}()
		}

		close(start)
		wg.Wait()
		assert.Equal(t, int32(1), compileCount.Load(), "concurrent cold-cache lookups must compile validating policy once")
	})

	t.Run("MutatingPolicy", func(t *testing.T) {
		cache := NewProviderCache()
		var compileCount atomic.Int32
		HookMpolCompiled = func() { compileCount.Add(1) }
		defer func() { HookMpolCompiled = nil }()

		policy := newTestMutatingPolicy("concurrent-miss-mpol", "1")
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(goroutines)

		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				<-start
				p, err := cache.GetMutatingPolicyProvider(policy, nil, nil, celengine.ExceptionScopeAll)
				assert.NoError(t, err)
				assert.NotNil(t, p)
			}()
		}

		close(start)
		wg.Wait()
		assert.Equal(t, int32(1), compileCount.Load(), "concurrent cold-cache lookups must compile mutating policy once")
	})

	t.Run("ImageValidatingPolicy", func(t *testing.T) {
		cache := NewProviderCache()
		var compileCount atomic.Int32
		HookIvpolCompiled = func() { compileCount.Add(1) }
		defer func() { HookIvpolCompiled = nil }()

		policy := newTestImageValidatingPolicy("concurrent-miss-ivpol", "1")
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(goroutines)

		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				<-start
				p, err := cache.GetImageValidatingPolicyProvider(policy, nil, nil, celengine.ExceptionScopeAll)
				assert.NoError(t, err)
				assert.NotNil(t, p)
			}()
		}

		close(start)
		wg.Wait()
		assert.Equal(t, int32(1), compileCount.Load(), "concurrent cold-cache lookups must compile image validating policy once")
	})
}

func TestProviderCache_InflightEvictDoesNotOverwriteNewerEntry(t *testing.T) {
	cache := NewProviderCache()
	var compileCount atomic.Int32
	compile1Started := make(chan struct{})
	allowCompile1Finish := make(chan struct{})

	HookIvpolCompiled = func() {
		if compileCount.Add(1) == 1 {
			close(compile1Started)
			<-allowCompile1Finish
		}
	}
	defer func() { HookIvpolCompiled = nil }()

	policyV1 := newTestImageValidatingPolicy("inflight-ivpol", "1")
	policyV2 := newTestImageValidatingPolicy("inflight-ivpol", "2")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		p, err := cache.GetImageValidatingPolicyProvider(policyV1, nil, nil, celengine.ExceptionScopeAll)
		assert.NoError(t, err)
		assert.NotNil(t, p)
	}()

	<-compile1Started
	// Evict while v1 compilation is in flight, then populate v2.
	cache.Evict(policyV1.GetKind(), policyV1.GetNamespace(), policyV1.GetName())

	p2, err := cache.GetImageValidatingPolicyProvider(policyV2, nil, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	require.NotNil(t, p2)
	assert.Equal(t, int32(2), compileCount.Load())

	// Let v1 finish and ensure it does not overwrite v2 in the cache.
	close(allowCompile1Finish)
	wg.Wait()

	p2Again, err := cache.GetImageValidatingPolicyProvider(policyV2, nil, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	require.NotNil(t, p2Again)
	assert.Equal(t, int32(2), compileCount.Load(), "stale in-flight compilation must not overwrite newer cached entry")

	fetched, err := p2Again.Fetch(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, fetched)
	for _, pol := range fetched {
		assert.Equal(t, "2", pol.Policy.GetResourceVersion())
	}
}

func TestProviderCache_InflightEvictDoesNotResurrect(t *testing.T) {
	cache := NewProviderCache()
	var compileCount atomic.Int32
	compile1Started := make(chan struct{})
	allowCompile1Finish := make(chan struct{})

	HookVpolCompiled = func() {
		if compileCount.Add(1) == 1 {
			close(compile1Started)
			<-allowCompile1Finish
		}
	}
	defer func() { HookVpolCompiled = nil }()

	policy := newTestValidatingPolicy("evict-inflight-vpol", "1")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		p, err := cache.GetValidatingPolicyProvider(policy, nil, celengine.ExceptionScopeAll)
		assert.NoError(t, err)
		assert.NotNil(t, p)
	}()

	<-compile1Started
	cache.Evict(policy.GetKind(), policy.GetNamespace(), policy.GetName())
	close(allowCompile1Finish)
	wg.Wait()

	// Next lookup must recompile because the in-flight compilation was evicted before completion.
	_, err := cache.GetValidatingPolicyProvider(policy, nil, celengine.ExceptionScopeAll)
	require.NoError(t, err)
	assert.Equal(t, int32(2), compileCount.Load(), "evicted in-flight compilation must not be resurrected in cache")
}

func TestProviderCache_DetachedWaiterDoesNotOverwriteNewerEntry(t *testing.T) {
	for _, tc := range []struct {
		name   string
		detach func(ProviderCache, *policiesv1beta1.ValidatingPolicy)
	}{
		{
			name: "evict",
			detach: func(c ProviderCache, p *policiesv1beta1.ValidatingPolicy) {
				c.Evict(p.GetKind(), p.GetNamespace(), p.GetName())
			},
		},
		{
			name: "clear",
			detach: func(c ProviderCache, _ *policiesv1beta1.ValidatingPolicy) {
				c.Clear()
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache := NewProviderCache()
			var compileCount atomic.Int32
			var slotLockAttempts atomic.Int32
			compile1Started := make(chan struct{})
			waiterSelectedSlot := make(chan struct{})
			allowCompile1Finish := make(chan struct{})

			cache.(*providerCache).vpol.onBeforeSlotLock = func() {
				if slotLockAttempts.Add(1) == 2 {
					close(waiterSelectedSlot)
				}
			}

			HookVpolCompiled = func() {
				if compileCount.Add(1) == 1 {
					close(compile1Started)
					<-allowCompile1Finish
				}
			}
			defer func() { HookVpolCompiled = nil }()

			policyV1 := newTestValidatingPolicy("detached-waiter-vpol", "1")
			policyV2 := newTestValidatingPolicy("detached-waiter-vpol", "2")

			var wg sync.WaitGroup
			wg.Add(2)

			// Goroutine 1 acquires slot1.mu and blocks in compile(v1).
			go func() {
				defer wg.Done()
				p, err := cache.GetValidatingPolicyProvider(policyV1, nil, celengine.ExceptionScopeAll)
				assert.NoError(t, err)
				assert.NotNil(t, p)
			}()

			<-compile1Started

			// Goroutine 2 sees slot1 in the map and signals before waiting on slot1.mu.
			go func() {
				defer wg.Done()
				p, err := cache.GetValidatingPolicyProvider(policyV1, nil, celengine.ExceptionScopeAll)
				assert.NoError(t, err)
				assert.NotNil(t, p)
			}()

			<-waiterSelectedSlot
			tc.detach(cache, policyV1)

			// Populate a replacement slot with v2 while Goroutine 1 and 2 are still on slot1.
			p2, err := cache.GetValidatingPolicyProvider(policyV2, nil, celengine.ExceptionScopeAll)
			require.NoError(t, err)
			require.NotNil(t, p2)
			compilesAfterV2 := compileCount.Load()

			// Unblock Goroutine 1, which also unblocks Goroutine 2 on slot1.mu.
			close(allowCompile1Finish)
			wg.Wait()

			// Neither Goroutine 1 nor Goroutine 2 may overwrite v2 in the replacement slot.
			compilesBeforeCheck := compileCount.Load()
			p2Again, err := cache.GetValidatingPolicyProvider(policyV2, nil, celengine.ExceptionScopeAll)
			require.NoError(t, err)
			require.NotNil(t, p2Again)
			assert.GreaterOrEqual(t, compilesAfterV2, int32(2))
			assert.Equal(t, compilesBeforeCheck, compileCount.Load(), "detached waiter must not overwrite newer cached entry")
		})
	}
}

func TestScanner_ProviderCachingAcrossResources(t *testing.T) {
	var compileCount int
	HookVpolCompiled = func() { compileCount++ }
	defer func() { HookVpolCompiled = nil }()

	scanner := newTestScanner(t)

	vp := newTestValidatingPolicy("vpol-scanner-cache", "1")
	policy := engineapi.NewValidatingPolicy(vp)

	pod1 := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "pod-1",
				"namespace": "test-ns",
			},
		},
	}
	pod2 := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "pod-2",
				"namespace": "test-ns",
			},
		},
	}
	pod3 := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "pod-3",
				"namespace": "test-ns",
			},
		},
	}
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}

	// Scan Resource 1: initial compile
	results1 := scanner.ScanResource(t.Context(), pod1, podGVR, "", nil, nil, nil, nil, policy)
	assert.Len(t, results1, 1)
	assert.Equal(t, 1, compileCount, "resource 1 should trigger first policy compilation")

	// Scan Resource 2 with same scanner: should reuse compiled provider
	results2 := scanner.ScanResource(t.Context(), pod2, podGVR, "", nil, nil, nil, nil, policy)
	assert.Len(t, results2, 1)
	assert.Equal(t, 1, compileCount, "resource 2 should reuse cached provider without compiling again")

	// Scan Resource 3: should still reuse cached provider
	results3 := scanner.ScanResource(t.Context(), pod3, podGVR, "", nil, nil, nil, nil, policy)
	assert.Len(t, results3, 1)
	assert.Equal(t, 1, compileCount, "resource 3 should reuse cached provider without compiling again")

	// Update policy revision: should recompile exactly once
	vpUpdated := newTestValidatingPolicy("vpol-scanner-cache", "2")
	policyUpdated := engineapi.NewValidatingPolicy(vpUpdated)

	resultsUpdated := scanner.ScanResource(t.Context(), pod1, podGVR, "", nil, nil, nil, nil, policyUpdated)
	assert.Len(t, resultsUpdated, 1)
	assert.Equal(t, 2, compileCount, "updated policy resourceVersion should trigger exactly one recompile")

	// Scan pod 2 with updated policy: should reuse updated compiled provider
	resultsUpdated2 := scanner.ScanResource(t.Context(), pod2, podGVR, "", nil, nil, nil, nil, policyUpdated)
	assert.Len(t, resultsUpdated2, 1)
	assert.Equal(t, 2, compileCount, "pod 2 should reuse updated cached provider")
}
