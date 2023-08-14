package imageverifycache

import (
	"context"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

type cache struct {
	logger         logr.Logger
	isCacheEnabled bool
	maxSize        int64
	ttl            time.Duration
	lock           sync.Mutex
	cache          *ristretto.Cache
}

type Option = func(*cache) error

func New(options ...Option) (Client, error) {
	cache := &cache{}
	for _, opt := range options {
		if err := opt(cache); err != nil {
			return nil, err
		}
	}
	config := ristretto.Config{
		MaxCost:     cache.maxSize,
		NumCounters: 10 * cache.maxSize,
		BufferItems: 64,
	}
	rcache, err := ristretto.NewCache(&config)
	if err != nil {
		return nil, err
	}
	cache.cache = rcache
	return cache, nil
}

func DisabledImageVerifyCache() Client {
	return &cache{
		logger:         logr.Discard(),
		isCacheEnabled: false,
		maxSize:        0,
		ttl:            0,
	}
}

func WithLogger(l logr.Logger) Option {
	return func(c *cache) error {
		c.logger = l
		return nil
	}
}

func WithCacheEnableFlag(b bool) Option {
	return func(c *cache) error {
		c.isCacheEnabled = b
		return nil
	}
}

func WithMaxSize(s int64) Option {
	return func(c *cache) error {
		c.maxSize = s
		return nil
	}
}

func WithTTLDuration(t time.Duration) Option {
	return func(c *cache) error {
		c.ttl = t * time.Minute
		return nil
	}
}

func generateKey(policy kyvernov1.PolicyInterface, ruleName string, imageRef string) string {
	return string(policy.GetUID()) + ";" + policy.GetResourceVersion() + ";" + ruleName + ";" + imageRef
}

func (c *cache) Set(ctx context.Context, policy kyvernov1.PolicyInterface, ruleName string, imageRef string) (bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.logger.WithValues("Setting cache", "namespace", policy.GetNamespace(), "policy", policy.GetName(), "ruleName", ruleName, "imageRef", imageRef)
	if !c.isCacheEnabled {
		return false, nil
	}
	key := generateKey(policy, ruleName, imageRef)

	stored := c.cache.SetWithTTL(key, nil, 1, c.ttl)
	c.cache.Wait()
	if stored {
		c.logger.WithValues("Successfully set cache", "namespace", policy.GetNamespace(), "policy", policy.GetName(), "ruleName", ruleName, "imageRef", imageRef)
		return true, nil
	}
	return false, nil
}

func (c *cache) Get(ctx context.Context, policy kyvernov1.PolicyInterface, ruleName string, imageRef string) (bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.logger.WithValues("Searching in cache", "namespace", policy.GetNamespace(), "policy", policy.GetName(), "ruleName", ruleName, "imageRef", imageRef)
	if !c.isCacheEnabled {
		return false, nil
	}
	key := generateKey(policy, ruleName, imageRef)
	_, found := c.cache.Get(key)
	if found {
		c.logger.WithValues("Cache entry found", "namespace", policy.GetNamespace(), policy.GetName(), "ruleName", ruleName, "imageRef", imageRef)
		return true, nil
	}
	c.logger.WithValues("Cache entry not found", "namespace", policy.GetNamespace(), "policy", policy.GetName(), "ruleName", ruleName, "imageRef", imageRef)
	return false, nil
}
