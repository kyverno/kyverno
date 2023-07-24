package imageverifycache

import (
	"context"
)

type cache struct {
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

func DisabledImageVerfiyCache() Client {
	return &cache{
		isCacheEnabled: false,
		maxSize:        0,
		TTL:            0,
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
	if !c.isCacheEnabled {
		return false, nil
	}
	return false, nil
}

func (c *cache) Get(ctx context.Context, policyId string, ruleName string, imageRef string) (bool, error) {
	if !c.isCacheEnabled {
		return false, nil
	}
	return false, nil
}

func (c *cache) Delete(ctx context.Context, policyId string, ruleName string, imageRef string) (bool, error) {
	if !c.isCacheEnabled {
		return false, nil
	}
	return false, nil
}

func (c *cache) DeleteForRule(ctx context.Context, policyId string, ruleName string) (bool, error) {
	if !c.isCacheEnabled {
		return false, nil
	}
	return false, nil
}

func (c *cache) DeleteForPolicy(ctx context.Context, policyId string) (bool, error) {
	if !c.isCacheEnabled {
		return false, nil
	}
	return false, nil
}

func (c *cache) Clear(ctx context.Context) (bool, error) {
	return false, nil
}
