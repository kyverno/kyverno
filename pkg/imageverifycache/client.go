package imageverifycache

import (
	"context"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

const (
	defaultTTL     = 1 * time.Hour
	deafultMaxSize = 1000
)

type cache struct {
	logger         logr.Logger
	isCacheEnabled bool
	maxSize        int64
	ttl            time.Duration
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
<<<<<<< HEAD
		BufferItems: 64,
=======
>>>>>>> 44c9196f1 (added ristretto_cache impl)
	}
	rcache, err := ristretto.NewCache(&config)
	if err != nil {
		return nil, err
	}
<<<<<<< HEAD
	cache.cache = rcache
=======
	cache.Cache = rcache
>>>>>>> 44c9196f1 (added ristretto_cache impl)
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
		if s == 0 {
			s = deafultMaxSize
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

func generateKey(policy kyvernov1.PolicyInterface, ruleName string, imageRef string) string {
	return string(policy.GetUID()) + ";" + policy.GetResourceVersion() + ";" + ruleName + ";" + imageRef
}
<<<<<<< HEAD
=======

func (c *cache) Set(ctx context.Context, policy kyvernov1.PolicyInterface, ruleName string, imageRef string) (bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
>>>>>>> 44c9196f1 (added ristretto_cache impl)

func (c *cache) Set(ctx context.Context, policy kyvernov1.PolicyInterface, ruleName string, imageRef string) (bool, error) {
	if !c.isCacheEnabled {
		return false, nil
	}
	key := generateKey(policy, ruleName, imageRef)

<<<<<<< HEAD
	stored := c.cache.SetWithTTL(key, nil, 1, c.ttl)
	if stored {
=======
	stored := c.Cache.SetWithTTL(key, nil, 1, c.ttl)
	if stored {
		c.logger.Info("Successfully set cache", "policy", policy.GetName(), "ruleName", ruleName, "imageRef", imageRef)
>>>>>>> 44c9196f1 (added ristretto_cache impl)
		return true, nil
	}
	return false, nil
}

func (c *cache) Get(ctx context.Context, policy kyvernov1.PolicyInterface, ruleName string, imageRef string) (bool, error) {
<<<<<<< HEAD
=======
	c.lock.Lock()
	defer c.lock.Unlock()
	c.logger.Info("Searching in cache", "policy", policy.GetName(), "ruleName", ruleName, "imageRef", imageRef)
>>>>>>> 44c9196f1 (added ristretto_cache impl)
	if !c.isCacheEnabled {
		return false, nil
	}
	key := generateKey(policy, ruleName, imageRef)
<<<<<<< HEAD
	_, found := c.cache.Get(key)
	if found {
		return true, nil
	}
=======
	_, found := c.Cache.Get(key)
	if found {
		c.logger.Info("Cache entry found", "policy", policy.GetName(), "ruleName", ruleName, "imageRef", imageRef)
		return true, nil
	}
	c.logger.Info("Cache entry not found", "policy", policy.GetName(), "ruleName", ruleName, "imageRef", imageRef)
>>>>>>> 44c9196f1 (added ristretto_cache impl)
	return false, nil
}
