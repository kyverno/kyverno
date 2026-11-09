package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
)

const (
	defaultTTL = 1 * time.Hour
	// DefaultMaxSize is the default cache budget in bytes (10 MiB).
	DefaultMaxSize = 10 << 20
	// expectedEntryCost is the average entry cost assumed when sizing
	// ristretto's admission counters: presence-only entries cost far less,
	// entries carrying attestation payloads usually a few times more.
	expectedEntryCost = 1 << 10
)

type cache struct {
	logger  logr.Logger
	maxSize int64
	ttl     time.Duration

	// keyed on a combination of attestation hash, attestor hash and image digest.
	// its goal is to save work in two cases. this exact attestor hash has verified
	// signature of this image digest within ttl, or this exact attestor has verified
	// this exact group of attestations on that image within ttl.
	cache *ristretto.Cache[string, any]
}

type Option = func(*cache) error

func New(options ...Option) (Client, error) {
	cache := &cache{}
	for _, opt := range options {
		if err := opt(cache); err != nil {
			return nil, err
		}
	}
	config := ristretto.Config[string, any]{
		MaxCost:     cache.maxSize,
		NumCounters: numCounters(cache.maxSize),
		BufferItems: 64,
	}
	rcache, err := ristretto.NewCache(&config)
	if err != nil {
		return nil, err
	}
	cache.cache = rcache
	return cache, nil
}

// numCounters follows ristretto's advice of ten counters per entry the cache is
// expected to hold. Ten counters per unit of the byte budget would allocate far
// more memory up front than the budget itself.
func numCounters(maxCost int64) int64 {
	return 10 * max(maxCost/expectedEntryCost, 100)
}

func WithLogger(l logr.Logger) Option {
	return func(c *cache) error {
		c.logger = l
		return nil
	}
}

func WithMaxSize(s int64) Option {
	return func(c *cache) error {
		if s == 0 {
			s = DefaultMaxSize
		}
		c.maxSize = s
		return nil
	}
}

func WithTTLDuration(t time.Duration) Option {
	return func(c *cache) error {
		if t == 0 {
			t = defaultTTL
		}
		c.ttl = t
		return nil
	}
}

func generateKey(imageRef string, attestors []v1beta1.Attestor, attestation *v1beta1.Attestation) (string, error) {
	digests := make([]string, 0, len(attestors))
	for _, attestor := range attestors {
		// The name is a policy-local label; only the resolved material decides
		// the verdict, so policies naming the same material differently share it.
		material := attestor
		material.Name = ""
		digest, err := hashJSON(material)
		if err != nil {
			return "", fmt.Errorf("failed to encode attestor %q: %w", attestor.GetKey(), err)
		}
		digests = append(digests, digest)
	}
	sort.Strings(digests)
	// The attestation slot is always written, empty for a signature entry, so a
	// signature entry can never share a key with an attestation entry.
	var attestationDigest string
	if attestation != nil {
		definition := *attestation
		definition.Name = ""
		digest, err := hashJSON(definition)
		if err != nil {
			return "", fmt.Errorf("failed to encode attestation %q: %w", attestation.Name, err)
		}
		attestationDigest = digest
	}
	var b strings.Builder
	writeKeyPart(&b, attestationDigest)
	for _, digest := range digests {
		writeKeyPart(&b, digest)
	}
	writeKeyPart(&b, imageRef)
	return b.String(), nil
}

func hashJSON(v any) (string, error) {
	encoded, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// writeKeyPart length-prefixes each part so no two part sequences can produce
// the same key.
func writeKeyPart(b *strings.Builder, part string) {
	fmt.Fprintf(b, "%d:%s|", len(part), part)
}

// we want to be able to get a hash that represents the exact identity of an attestor.
// if the attestor has a KMS url, then the data we have is undeterministic and we cannot
// build a cache rule for that attestor
func hasUnbindableKeyMaterial(attestors []v1beta1.Attestor) bool {
	for _, attestor := range attestors {
		if attestor.Cosign != nil && attestor.Cosign.Key != nil && attestor.Cosign.Key.KMS != "" {
			return true
		}
	}
	return false
}

// key returns the entry key, or "" when the result must not be cached.
func (c *cache) key(imageRef string, attestors []v1beta1.Attestor, attestation *v1beta1.Attestation) (string, error) {
	if len(attestors) == 0 {
		return "", nil
	}
	if !isDigestRef(imageRef) {
		// A verdict keyed on a mutable reference could outlive what the
		// reference points to: the registry may move a tag at any time.
		return "", nil
	}
	if hasUnbindableKeyMaterial(attestors) {
		c.logger.V(2).Info("caching disabled: attestor verifies with key material the cache key cannot bind", "image", imageRef)
		return "", nil
	}
	return generateKey(imageRef, attestors, attestation)
}

// isDigestRef reports whether imageRef is pinned to a digest (either
// "repo@sha256:d" or "repo:tag@sha256:d"). Only such references are immutable
// enough to key a verification verdict; a bare tag can be moved by the
// registry after the entry is written.
func isDigestRef(imageRef string) bool {
	_, err := name.NewDigest(imageRef)
	return err == nil
}

func (c *cache) Set(imageRef string, attestors []v1beta1.Attestor, attestation *v1beta1.Attestation, payloads map[string][]byte) (bool, error) {
	key, err := c.key(imageRef, attestors, attestation)
	if err != nil || key == "" {
		return false, err
	}
	cost := payloadCost(payloads)

	// ristretto replaces an existing key without checking MaxCost, so drop the
	// old entry first and let the new one go through the normal admission.
	c.cache.Del(key)
	c.cache.SetWithTTL(key, clonePayloads(payloads), cost, c.ttl)
	c.cache.Wait()
	// SetWithTTL returning true only means ristretto queued the write. Its
	// policy can still drop the entry afterwards, for example when the entry
	// costs more than the whole budget, so report what the cache holds.
	_, stored := c.cache.GetTTL(key)
	if !stored {
		// Callers don't act on the result, so this is the only sign that the
		// image will be verified again on its next admission.
		c.logger.V(3).Info("image verify cache did not keep the entry, the cache is full or the entry is larger than --imageVerifyCacheMaxSize",
			"image", imageRef, "cost", cost)
	}
	return stored, nil
}

func (c *cache) Get(imageRef string, attestors []v1beta1.Attestor, attestation *v1beta1.Attestation) (bool, map[string][]byte, error) {
	key, err := c.key(imageRef, attestors, attestation)
	if err != nil || key == "" {
		return false, nil, err
	}
	val, found := c.cache.Get(key)
	if !found {
		return false, nil, nil
	}
	payloads, _ := val.(map[string][]byte)
	return true, clonePayloads(payloads), nil
}

// payloadCost estimates the memory footprint of a cache entry so ristretto's
// MaxCost (--imageVerifyCacheMaxSize) bounds actual memory rather than just
// entry count: a presence-only entry (nil payloads) costs 1, while an entry
// carrying attestation payload bytes costs roughly its real size. ristretto
// adds its own per-item cost on top of both.
func payloadCost(payloads map[string][]byte) int64 {
	cost := int64(1)
	for _, v := range payloads {
		cost += int64(len(v))
	}
	return cost
}

func clonePayloads(in map[string][]byte) map[string][]byte {
	if in == nil {
		return nil
	}
	out := make(map[string][]byte, len(in))
	for k, v := range in {
		if v == nil {
			out[k] = nil
			continue
		}
		cp := make([]byte, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}
