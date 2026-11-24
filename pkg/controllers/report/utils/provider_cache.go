package utils

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
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
	) (vpolengine.Provider, error)

	GetMutatingPolicyProvider(
		policy policiesv1beta1.MutatingPolicyLike,
		exceptions []*policiesv1beta1.PolicyException,
		libCxt libs.Context,
	) (mpolengine.Provider, error)

	GetImageValidatingPolicyProvider(
		policy policiesv1beta1.ImageValidatingPolicyLike,
		exceptions []*policiesv1beta1.PolicyException,
		secretLister corev1listers.SecretLister,
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

type providerCache struct {
	lock sync.RWMutex

	vpol  map[string]*cachedEntry[vpolengine.Provider]
	mpol  map[string]*cachedEntry[mpolengine.Provider]
	ivpol map[string]*cachedEntry[ivpolengine.Provider]

	vpolCompiler vpolcompiler.Compiler
	mpolCompiler mpolcompiler.Compiler
}

// NewProviderCache constructs an empty, thread-safe ProviderCache.
func NewProviderCache() ProviderCache {
	return &providerCache{
		vpol:         make(map[string]*cachedEntry[vpolengine.Provider]),
		mpol:         make(map[string]*cachedEntry[mpolengine.Provider]),
		ivpol:        make(map[string]*cachedEntry[ivpolengine.Provider]),
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

func matchExceptions(kind, name string, exceptions []*policiesv1beta1.PolicyException) ([]*policiesv1beta1.PolicyException, string) {
	var matched []*policiesv1beta1.PolicyException
	var parts []string
	for _, polex := range exceptions {
		if polex == nil {
			continue
		}
		for _, ref := range polex.Spec.PolicyRefs {
			if ref.Name == name && ref.Kind == kind {
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
				break
			}
		}
	}
	if len(parts) == 0 {
		return matched, ""
	}
	sort.Strings(parts)
	return matched, strings.Join(parts, ";")
}

func (c *providerCache) GetValidatingPolicyProvider(
	policy policiesv1beta1.ValidatingPolicyLike,
	exceptions []*policiesv1beta1.PolicyException,
) (vpolengine.Provider, error) {
	key := policyIdentifier(policy.GetKind(), policy.GetNamespace(), policy.GetName())
	rv := policyRevision(policy)
	matchedExceptions, excKey := matchExceptions(policy.GetKind(), policy.GetName(), exceptions)

	c.lock.RLock()
	entry, found := c.vpol[key]
	if found && entry.policyRV == rv && entry.exceptionsKey == excKey {
		provider, err := entry.provider, entry.err
		c.lock.RUnlock()
		return provider, err
	}
	c.lock.RUnlock()

	if HookVpolCompiled != nil {
		HookVpolCompiled()
	}
	provider, err := vpolengine.NewProvider(c.vpolCompiler, []policiesv1beta1.ValidatingPolicyLike{policy}, matchedExceptions)

	c.lock.Lock()
	c.vpol[key] = &cachedEntry[vpolengine.Provider]{
		policyRV:      rv,
		exceptionsKey: excKey,
		provider:      provider,
		err:           err,
	}
	c.lock.Unlock()

	return provider, err
}

func (c *providerCache) GetMutatingPolicyProvider(
	policy policiesv1beta1.MutatingPolicyLike,
	exceptions []*policiesv1beta1.PolicyException,
	libCxt libs.Context,
) (mpolengine.Provider, error) {
	key := policyIdentifier(policy.GetKind(), policy.GetNamespace(), policy.GetName())
	rv := policyRevision(policy)
	matchedExceptions, excKey := matchExceptions(policy.GetKind(), policy.GetName(), exceptions)

	c.lock.RLock()
	entry, found := c.mpol[key]
	if found && entry.policyRV == rv && entry.exceptionsKey == excKey {
		provider, err := entry.provider, entry.err
		c.lock.RUnlock()
		return provider, err
	}
	c.lock.RUnlock()

	if HookMpolCompiled != nil {
		HookMpolCompiled()
	}
	if libCxt == nil {
		libCxt = libs.GetLibsCtx()
	}
	provider, err := mpolengine.NewProvider(c.mpolCompiler, []policiesv1beta1.MutatingPolicyLike{policy}, matchedExceptions, libCxt)

	c.lock.Lock()
	c.mpol[key] = &cachedEntry[mpolengine.Provider]{
		policyRV:      rv,
		exceptionsKey: excKey,
		provider:      provider,
		err:           err,
	}
	c.lock.Unlock()

	return provider, err
}

func (c *providerCache) GetImageValidatingPolicyProvider(
	policy policiesv1beta1.ImageValidatingPolicyLike,
	exceptions []*policiesv1beta1.PolicyException,
	secretLister corev1listers.SecretLister,
) (ivpolengine.Provider, error) {
	key := policyIdentifier(policy.GetKind(), policy.GetNamespace(), policy.GetName())
	rv := policyRevision(policy)
	matchedExceptions, excKey := matchExceptions(policy.GetKind(), policy.GetName(), exceptions)

	c.lock.RLock()
	entry, found := c.ivpol[key]
	if found && entry.policyRV == rv && entry.exceptionsKey == excKey {
		provider, err := entry.provider, entry.err
		c.lock.RUnlock()
		return provider, err
	}
	c.lock.RUnlock()

	if HookIvpolCompiled != nil {
		HookIvpolCompiled()
	}
	compiler := iveval.NewCompiler(secretLister)
	provider, err := ivpolengine.NewProvider(compiler, []policiesv1beta1.ImageValidatingPolicyLike{policy}, matchedExceptions)

	c.lock.Lock()
	c.ivpol[key] = &cachedEntry[ivpolengine.Provider]{
		policyRV:      rv,
		exceptionsKey: excKey,
		provider:      provider,
		err:           err,
	}
	c.lock.Unlock()

	return provider, err
}

func (c *providerCache) Evict(kind, namespace, name string) {
	key := policyIdentifier(kind, namespace, name)
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.vpol, key)
	delete(c.mpol, key)
	delete(c.ivpol, key)
}

func (c *providerCache) Clear() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.vpol = make(map[string]*cachedEntry[vpolengine.Provider])
	c.mpol = make(map[string]*cachedEntry[mpolengine.Provider])
	c.ivpol = make(map[string]*cachedEntry[ivpolengine.Provider])
}
