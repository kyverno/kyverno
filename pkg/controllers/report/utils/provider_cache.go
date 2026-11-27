package utils

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	iveval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// Testing hooks to observe compilations and cache hits in unit tests.
var (
	HookVpolCompiled  func()
	HookMpolCompiled  func()
	HookIvpolCompiled func()
)

// ProviderCache caches compiled policy providers across scanned resources in background reporting.
type ProviderCache interface {
	GetValidatingPolicyProvider(
		policy policiesv1beta1.ValidatingPolicyLike,
		exceptions []*policiesv1beta1.PolicyException,
		scope celengine.ExceptionScope,
	) (vpolengine.Provider, error)

	GetMutatingPolicyProvider(
		policy policiesv1beta1.MutatingPolicyLike,
		exceptions []*policiesv1beta1.PolicyException,
		libCxt libs.Context,
		scope celengine.ExceptionScope,
	) (mpolengine.Provider, error)

	GetImageValidatingPolicyProvider(
		policy policiesv1beta1.ImageValidatingPolicyLike,
		exceptions []*policiesv1beta1.PolicyException,
		secretLister corev1listers.SecretLister,
		scope celengine.ExceptionScope,
	) (ivpolengine.Provider, error)

	Evict(kind, namespace, name string)
	Clear()
}

type cachedEntry[T any] struct {
	policyRV      string
	exceptionsKey string
	provider      T
	err           error
}

type cacheSlot[T any] struct {
	mu    sync.Mutex
	entry *cachedEntry[T]
}

// providerStore coordinates concurrent lookups and cache fills per policy key.
// Fast-path hits only take lock.RLock; misses serialize on a per-key slot.mu
// without holding lock during compilation so unrelated keys, Evict, and Clear
// never block on CEL compilation. Lock order is strictly slot.mu -> lock.
type providerStore[T any] struct {
	lock             sync.RWMutex
	slots            map[string]*cacheSlot[T]
	onBeforeSlotLock func()
}

func newProviderStore[T any]() *providerStore[T] {
	return &providerStore[T]{
		slots: make(map[string]*cacheSlot[T]),
	}
}

func (s *providerStore[T]) getOrCompile(
	key string,
	rv string,
	excKey string,
	compile func() (T, error),
) (T, error) {
	// Fast path: check cached entry under read lock.
	s.lock.RLock()
	slot := s.slots[key]
	if slot != nil {
		if entry := slot.entry; entry != nil && entry.policyRV == rv && entry.exceptionsKey == excKey {
			provider, err := entry.provider, entry.err
			s.lock.RUnlock()
			return provider, err
		}
	}
	s.lock.RUnlock()

	s.lock.Lock()
	slot = s.slots[key]
	if slot == nil {
		slot = &cacheSlot[T]{}
		s.slots[key] = slot
	}
	s.lock.Unlock()

	return s.fillSlot(slot, key, rv, excKey, compile)
}

func (s *providerStore[T]) fillSlot(
	slot *cacheSlot[T],
	key string,
	rv string,
	excKey string,
	compile func() (T, error),
) (T, error) {
	if s.onBeforeSlotLock != nil {
		s.onBeforeSlotLock()
	}
	slot.mu.Lock()

	// Confirm the slot is still active in the map (Evict/Clear may have detached it
	// before slot.mu was acquired) and re-check in case another worker just filled it.
	s.lock.RLock()
	active := s.slots[key] == slot
	var hit *cachedEntry[T]
	if slot.entry != nil && slot.entry.policyRV == rv && slot.entry.exceptionsKey == excKey {
		hit = slot.entry
	}
	s.lock.RUnlock()

	if hit != nil {
		slot.mu.Unlock()
		return hit.provider, hit.err
	}
	if !active {
		// Slot was detached by Evict/Clear while waiting on slot.mu; compile
		// directly without publishing or retrying into a replacement slot so
		// stale revisions cannot overwrite newer entries or resurrect evicted ones.
		slot.mu.Unlock()
		return compile()
	}

	defer slot.mu.Unlock()

	provider, err := compile()

	s.lock.Lock()
	// Only publish if Evict/Clear did not detach this slot while compiling.
	if s.slots[key] == slot {
		slot.entry = &cachedEntry[T]{
			policyRV:      rv,
			exceptionsKey: excKey,
			provider:      provider,
			err:           err,
		}
	}
	s.lock.Unlock()

	return provider, err
}

func (s *providerStore[T]) evict(key string) {
	s.lock.Lock()
	delete(s.slots, key)
	s.lock.Unlock()
}

func (s *providerStore[T]) clear() {
	s.lock.Lock()
	s.slots = make(map[string]*cacheSlot[T])
	s.lock.Unlock()
}

type providerCache struct {
	vpol  *providerStore[vpolengine.Provider]
	mpol  *providerStore[mpolengine.Provider]
	ivpol *providerStore[ivpolengine.Provider]

	vpolCompiler vpolcompiler.Compiler
	mpolCompiler mpolcompiler.Compiler
}

// NewProviderCache constructs an empty, thread-safe ProviderCache.
func NewProviderCache() ProviderCache {
	return &providerCache{
		vpol:         newProviderStore[vpolengine.Provider](),
		mpol:         newProviderStore[mpolengine.Provider](),
		ivpol:        newProviderStore[ivpolengine.Provider](),
		vpolCompiler: vpolcompiler.NewCompiler(),
		mpolCompiler: mpolcompiler.NewCompiler(),
	}
}

func policyIdentifier(kind, namespace, name string) string {
	if namespace != "" {
		return fmt.Sprintf("%s/%s/%s", kind, namespace, name)
	}
	return fmt.Sprintf("%s/%s", kind, name)
}

func policyRevision(meta metav1.Object) string {
	if rv := meta.GetResourceVersion(); rv != "" {
		return rv
	}
	if gen := meta.GetGeneration(); gen != 0 {
		return fmt.Sprintf("gen:%d", gen)
	}
	b, err := json.Marshal(meta)
	if err == nil {
		return fmt.Sprintf("spec-hash:%x", sha256.Sum256(b))
	}
	return ""
}

func matchExceptions(
	kind, namespace, name string,
	exceptions []*policiesv1beta1.PolicyException,
	scope celengine.ExceptionScope,
) ([]*policiesv1beta1.PolicyException, string) {
	var matched []*policiesv1beta1.PolicyException
	var parts []string
	for _, polex := range exceptions {
		if polex == nil {
			continue
		}
		if !scope.Applies(polex, kind, name, namespace) {
			continue
		}
		matched = append(matched, polex)
		rv := polex.GetResourceVersion()
		if rv == "" {
			if gen := polex.GetGeneration(); gen != 0 {
				rv = fmt.Sprintf("gen:%d", gen)
			} else {
				b, _ := json.Marshal(polex.Spec)
				rv = fmt.Sprintf("hash:%x", sha256.Sum256(b))
			}
		}
		parts = append(parts, fmt.Sprintf("%s/%s@%s", polex.GetNamespace(), polex.GetName(), rv))
	}
	if len(parts) == 0 {
		return matched, ""
	}
	sort.Strings(parts)
	return matched, strings.Join(parts, ";")
}

func cacheLookupKey(
	kind, namespace, name string,
	meta metav1.Object,
	exceptions []*policiesv1beta1.PolicyException,
	scope celengine.ExceptionScope,
) (key, rv, excKey string, matched []*policiesv1beta1.PolicyException) {
	key = policyIdentifier(kind, namespace, name)
	rv = policyRevision(meta)
	matched, excKey = matchExceptions(kind, namespace, name, exceptions, scope)
	return key, rv, excKey, matched
}

func (c *providerCache) GetValidatingPolicyProvider(
	policy policiesv1beta1.ValidatingPolicyLike,
	exceptions []*policiesv1beta1.PolicyException,
	scope celengine.ExceptionScope,
) (vpolengine.Provider, error) {
	key, rv, excKey, matchedExceptions := cacheLookupKey(
		policy.GetKind(),
		policy.GetNamespace(),
		policy.GetName(),
		policy,
		exceptions,
		scope,
	)
	return c.vpol.getOrCompile(key, rv, excKey, func() (vpolengine.Provider, error) {
		if HookVpolCompiled != nil {
			HookVpolCompiled()
		}
		return vpolengine.NewProvider(c.vpolCompiler, []policiesv1beta1.ValidatingPolicyLike{policy}, matchedExceptions, scope)
	})
}

func (c *providerCache) GetMutatingPolicyProvider(
	policy policiesv1beta1.MutatingPolicyLike,
	exceptions []*policiesv1beta1.PolicyException,
	libCxt libs.Context,
	scope celengine.ExceptionScope,
) (mpolengine.Provider, error) {
	key, rv, excKey, matchedExceptions := cacheLookupKey(
		policy.GetKind(),
		policy.GetNamespace(),
		policy.GetName(),
		policy,
		exceptions,
		scope,
	)
	return c.mpol.getOrCompile(key, rv, excKey, func() (mpolengine.Provider, error) {
		if HookMpolCompiled != nil {
			HookMpolCompiled()
		}
		if libCxt == nil {
			libCxt = libs.GetLibsCtx()
		}
		return mpolengine.NewProvider(c.mpolCompiler, []policiesv1beta1.MutatingPolicyLike{policy}, matchedExceptions, libCxt, scope)
	})
}

func (c *providerCache) GetImageValidatingPolicyProvider(
	policy policiesv1beta1.ImageValidatingPolicyLike,
	exceptions []*policiesv1beta1.PolicyException,
	secretLister corev1listers.SecretLister,
	scope celengine.ExceptionScope,
) (ivpolengine.Provider, error) {
	key, rv, excKey, matchedExceptions := cacheLookupKey(
		policy.GetKind(),
		policy.GetNamespace(),
		policy.GetName(),
		policy,
		exceptions,
		scope,
	)
	return c.ivpol.getOrCompile(key, rv, excKey, func() (ivpolengine.Provider, error) {
		if HookIvpolCompiled != nil {
			HookIvpolCompiled()
		}
		compiler := iveval.NewCompiler(secretLister)
		return ivpolengine.NewProvider(compiler, []policiesv1beta1.ImageValidatingPolicyLike{policy}, matchedExceptions, scope)
	})
}

func (c *providerCache) Evict(kind, namespace, name string) {
	key := policyIdentifier(kind, namespace, name)
	c.vpol.evict(key)
	c.mpol.evict(key)
	c.ivpol.evict(key)
}

func (c *providerCache) Clear() {
	c.vpol.clear()
	c.mpol.clear()
	c.ivpol.clear()
}
