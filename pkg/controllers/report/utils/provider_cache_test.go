package utils

import (
	"sync"
	"sync/atomic"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
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
	provider1, err := cache.GetValidatingPolicyProvider(policy, nil)
	require.NoError(t, err)
	require.NotNil(t, provider1)
	assert.Equal(t, 1, compileCount, "first lookup must compile policy")

	// 2. Second lookup with same policy and revision -> cache hit, does not recompile
	provider2, err := cache.GetValidatingPolicyProvider(policy, nil)
	require.NoError(t, err)
	require.NotNil(t, provider2)
	assert.Equal(t, 1, compileCount, "second lookup with same revision must be a cache hit")

	// 3. Update ResourceVersion -> cache miss, recompiles
	policyUpdated := newTestValidatingPolicy("test-vpol", "101")
	provider3, err := cache.GetValidatingPolicyProvider(policyUpdated, nil)
	require.NoError(t, err)
	require.NotNil(t, provider3)
	assert.Equal(t, 2, compileCount, "updating policy resourceVersion must trigger recompilation")

	// 4. Repeated lookup with new ResourceVersion -> cache hit
	provider4, err := cache.GetValidatingPolicyProvider(policyUpdated, nil)
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
	_, err := cache.GetValidatingPolicyProvider(policy, []*policiesv1beta1.PolicyException{polex1})
	require.NoError(t, err)
	assert.Equal(t, 1, compileCount)

	// 2. Lookup with same polex1 -> hit
	_, err = cache.GetValidatingPolicyProvider(policy, []*policiesv1beta1.PolicyException{polex1})
	require.NoError(t, err)
	assert.Equal(t, 1, compileCount)

	// 3. Lookup with polex1 and unrelated exception -> hit (unrelated exception does not affect key)
	_, err = cache.GetValidatingPolicyProvider(policy, []*policiesv1beta1.PolicyException{polex1, polexUnrelated})
	require.NoError(t, err)
	assert.Equal(t, 1, compileCount, "unrelated policy exception must not invalidate cache")

	// 4. Modify matching exception resourceVersion -> recompile
	polex1Updated := newTestPolicyException("polex-1", "default", "11", "ValidatingPolicy", "test-vpol-exc")
	_, err = cache.GetValidatingPolicyProvider(policy, []*policiesv1beta1.PolicyException{polex1Updated})
	require.NoError(t, err)
	assert.Equal(t, 2, compileCount, "matching exception resourceVersion update must trigger recompilation")

	// 5. Add second matching exception -> recompile
	polex2 := newTestPolicyException("polex-2", "default", "20", "ValidatingPolicy", "test-vpol-exc")
	_, err = cache.GetValidatingPolicyProvider(policy, []*policiesv1beta1.PolicyException{polex1Updated, polex2})
	require.NoError(t, err)
	assert.Equal(t, 3, compileCount, "adding matching exception must trigger recompilation")

	// 6. Remove matching exception -> recompile
	_, err = cache.GetValidatingPolicyProvider(policy, []*policiesv1beta1.PolicyException{polex1Updated})
	require.NoError(t, err)
	assert.Equal(t, 4, compileCount, "removing matching exception must trigger recompilation")
}

func TestProviderCache_MutatingPolicy_HitAndMiss(t *testing.T) {
	cache := NewProviderCache()
	var compileCount int
	HookMpolCompiled = func() { compileCount++ }
	defer func() { HookMpolCompiled = nil }()

	policy := newTestMutatingPolicy("test-mpol", "200")

	// 1. Initial lookup -> cache miss
	provider1, err := cache.GetMutatingPolicyProvider(policy, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, provider1)
	assert.Equal(t, 1, compileCount)

	// 2. Second lookup -> cache hit
	provider2, err := cache.GetMutatingPolicyProvider(policy, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, provider2)
	assert.Equal(t, 1, compileCount)

	// 3. Update ResourceVersion -> recompile
	policyUpdated := newTestMutatingPolicy("test-mpol", "201")
	provider3, err := cache.GetMutatingPolicyProvider(policyUpdated, nil, nil)
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
	provider1, err := cache.GetImageValidatingPolicyProvider(policy, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, provider1)
	assert.Equal(t, 1, compileCount)

	// 2. Second lookup -> cache hit
	provider2, err := cache.GetImageValidatingPolicyProvider(policy, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, provider2)
	assert.Equal(t, 1, compileCount)

	// 3. Update ResourceVersion -> recompile
	policyUpdated := newTestImageValidatingPolicy("test-ivpol", "301")
	provider3, err := cache.GetImageValidatingPolicyProvider(policyUpdated, nil, nil)
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
	_, err := cache.GetValidatingPolicyProvider(policy, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, vpolCompileCount)

	// Evict specific policy
	cache.Evict(policy.GetKind(), policy.GetNamespace(), policy.GetName())

	// Next lookup should recompile
	_, err = cache.GetValidatingPolicyProvider(policy, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, vpolCompileCount, "evicted policy must be recompiled on next lookup")

	// Clear whole cache
	cache.Clear()
	_, err = cache.GetValidatingPolicyProvider(policy, nil)
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
				p, err := cache.GetValidatingPolicyProvider(policy, nil)
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
				p, err := cache.GetValidatingPolicyProvider(policy, nil)
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
				p, err := cache.GetMutatingPolicyProvider(policy, nil, nil)
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
				p, err := cache.GetImageValidatingPolicyProvider(policy, nil, nil)
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
		p, err := cache.GetImageValidatingPolicyProvider(policyV1, nil, nil)
		assert.NoError(t, err)
		assert.NotNil(t, p)
	}()

	<-compile1Started
	// Evict while v1 compilation is in flight, then populate v2.
	cache.Evict(policyV1.GetKind(), policyV1.GetNamespace(), policyV1.GetName())

	p2, err := cache.GetImageValidatingPolicyProvider(policyV2, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, p2)
	assert.Equal(t, int32(2), compileCount.Load())

	// Let v1 finish and ensure it does not overwrite v2 in the cache.
	close(allowCompile1Finish)
	wg.Wait()

	p2Again, err := cache.GetImageValidatingPolicyProvider(policyV2, nil, nil)
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
		p, err := cache.GetValidatingPolicyProvider(policy, nil)
		assert.NoError(t, err)
		assert.NotNil(t, p)
	}()

	<-compile1Started
	cache.Evict(policy.GetKind(), policy.GetNamespace(), policy.GetName())
	close(allowCompile1Finish)
	wg.Wait()

	// Next lookup must recompile because the in-flight compilation was evicted before completion.
	_, err := cache.GetValidatingPolicyProvider(policy, nil)
	require.NoError(t, err)
	assert.Equal(t, int32(2), compileCount.Load(), "evicted in-flight compilation must not be resurrected in cache")
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
