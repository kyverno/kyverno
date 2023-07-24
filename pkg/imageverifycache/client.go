package imageverifycache

import (
	"context"

	"github.com/go-logr/logr"
)

type cache struct {
	logger         logr.Logger
	isCacheEnabled bool
	maxSize        int64
	TTL            int64
}

type Option = func(*cache) error

func New(options ...Option) (Client, error) {
	cache := &cache{}
	for _, opt := range options {
		if err := opt(cache); err != nil {
			return nil, err
		}
	}

	return cache, nil
}

func DisabledImageVerifyCache() Client {
	return &cache{
		logger:         logr.Discard(),
		isCacheEnabled: false,
		maxSize:        0,
		TTL:            0,
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

func WithTTLDuration(t int64) Option {
	return func(c *cache) error {
		c.TTL = t
		return nil
	}
}

func (c *cache) Set(ctx context.Context, policyId string, ruleName string, imageRef string) (bool, error) {
	c.logger.Info("Setting cache", "policyId", policyId, "ruleName", ruleName, "imageRef", imageRef)
	if !c.isCacheEnabled {
		return false, nil
	}
	c.logger.Info("Successfully set cache", "policyId", policyId, "ruleName", ruleName, "imageRef", imageRef)
	return false, nil
}

func (c *cache) Get(ctx context.Context, policyId string, ruleName string, imageRef string) (bool, error) {
	c.logger.Info("Searching in cache", "policyId", policyId, "ruleName", ruleName, "imageRef", imageRef)
	if !c.isCacheEnabled {
		return false, nil
	}
	c.logger.Info("Cache entry not found", "policyId", policyId, "ruleName", ruleName, "imageRef", imageRef)
	c.logger.Info("Cache entry found", "policyId", policyId, "ruleName", ruleName, "imageRef", imageRef)
	return false, nil
}

func (c *cache) Delete(ctx context.Context, policyId string, ruleName string, imageRef string) (bool, error) {
	c.logger.Info("Deleting cache entry", "policyId", policyId, "ruleName", ruleName, "imageRef", imageRef)
	if !c.isCacheEnabled {
		return false, nil
	}
	c.logger.Info("Successfully deleted cache entry", "policyId", policyId, "ruleName", ruleName, "imageRef", imageRef)
	return false, nil
}

func (c *cache) DeleteForRule(ctx context.Context, policyId string, ruleName string) (bool, error) {
	c.logger.Info("Deleting cache for rule", "policyId", policyId, "ruleName", ruleName)
	if !c.isCacheEnabled {
		return false, nil
	}
	c.logger.Info("Successfully deleted cache for rule", "policyId", policyId, "ruleName", ruleName)
	return false, nil
}

func (c *cache) DeleteForPolicy(ctx context.Context, policyId string) (bool, error) {
	c.logger.Info("Deleting cache for policy", "policyId", policyId)
	if !c.isCacheEnabled {
		return false, nil
	}
	c.logger.Info("Successfully deleted cache for policy", "policyId", policyId)
	return false, nil
}

func (c *cache) Clear(ctx context.Context) (bool, error) {
	c.logger.Info("Clearing cache")
	c.logger.Info("Cleared cache")
	return false, nil
}
