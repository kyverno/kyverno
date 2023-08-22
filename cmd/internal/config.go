package internal

import (
	"flag"
)

type Configuration interface {
	UsesMetrics() bool
	UsesTracing() bool
	UsesProfiling() bool
	UsesKubeconfig() bool
	UsesPolicyExceptions() bool
	UsesConfigMapCaching() bool
	UsesDeferredLoading() bool
	UsesCosign() bool
	UsesRegistryClient() bool
	UsesImageVerifyCache() bool
	UsesLeaderElection() bool
	UsesKyvernoClient() bool
	UsesDynamicClient() bool
	UsesApiServerClient() bool
	UsesMetadataClient() bool
	UsesKyvernoDynamicClient() bool
	FlagSets() []*flag.FlagSet
}

func NewConfiguration(options ...ConfigurationOption) Configuration {
	c := &configuration{}
	for _, option := range options {
		option(c)
	}
	return c
}

type ConfigurationOption func(c *configuration)

func WithMetrics() ConfigurationOption {
	return func(c *configuration) {
		c.usesMetrics = true
	}
}

func WithTracing() ConfigurationOption {
	return func(c *configuration) {
		c.usesTracing = true
	}
}

func WithProfiling() ConfigurationOption {
	return func(c *configuration) {
		c.usesProfiling = true
	}
}

func WithKubeconfig() ConfigurationOption {
	return func(c *configuration) {
		c.usesKubeconfig = true
	}
}

func WithPolicyExceptions() ConfigurationOption {
	return func(c *configuration) {
		c.usesPolicyExceptions = true
	}
}

func WithConfigMapCaching() ConfigurationOption {
	return func(c *configuration) {
		c.usesConfigMapCaching = true
	}
}

func WithDeferredLoading() ConfigurationOption {
	return func(c *configuration) {
		c.usesDeferredLoading = true
	}
}

func WithCosign() ConfigurationOption {
	return func(c *configuration) {
		c.usesCosign = true
	}
}

func WithRegistryClient() ConfigurationOption {
	return func(c *configuration) {
		c.usesRegistryClient = true
	}
}

func WithImageVerifyCache() ConfigurationOption {
	return func(c *configuration) {
		c.usesImageVerifyCache = true
	}
}

func WithLeaderElection() ConfigurationOption {
	return func(c *configuration) {
		c.usesLeaderElection = true
	}
}

func WithKyvernoClient() ConfigurationOption {
	return func(c *configuration) {
		c.usesKyvernoClient = true
	}
}

func WithDynamicClient() ConfigurationOption {
	return func(c *configuration) {
		c.usesDynamicClient = true
	}
}

func WithApiServerClient() ConfigurationOption {
	return func(c *configuration) {
		c.usesApiServerClient = true
	}
}

func WithMetadataClient() ConfigurationOption {
	return func(c *configuration) {
		c.usesMetadataClient = true
	}
}

func WithKyvernoDynamicClient() ConfigurationOption {
	return func(c *configuration) {
		// requires dynamic client
		c.usesDynamicClient = true
		c.usesKyvernoDynamicClient = true
	}
}

func WithFlagSets(flagsets ...*flag.FlagSet) ConfigurationOption {
	return func(c *configuration) {
		c.flagSets = append(c.flagSets, flagsets...)
	}
}

type configuration struct {
	usesMetrics              bool
	usesTracing              bool
	usesProfiling            bool
	usesKubeconfig           bool
	usesPolicyExceptions     bool
	usesConfigMapCaching     bool
	usesDeferredLoading      bool
	usesCosign               bool
	usesRegistryClient       bool
	usesImageVerifyCache     bool
	usesLeaderElection       bool
	usesKyvernoClient        bool
	usesDynamicClient        bool
	usesApiServerClient      bool
	usesMetadataClient       bool
	usesKyvernoDynamicClient bool
	flagSets                 []*flag.FlagSet
}

func (c *configuration) UsesMetrics() bool {
	return c.usesMetrics
}

func (c *configuration) UsesTracing() bool {
	return c.usesTracing
}

func (c *configuration) UsesProfiling() bool {
	return c.usesProfiling
}

func (c *configuration) UsesKubeconfig() bool {
	return c.usesKubeconfig
}

func (c *configuration) UsesPolicyExceptions() bool {
	return c.usesPolicyExceptions
}

func (c *configuration) UsesConfigMapCaching() bool {
	return c.usesConfigMapCaching
}

func (c *configuration) UsesDeferredLoading() bool {
	return c.usesDeferredLoading
}

func (c *configuration) UsesCosign() bool {
	return c.usesCosign
}

func (c *configuration) UsesRegistryClient() bool {
	return c.usesRegistryClient
}

func (c *configuration) UsesImageVerifyCache() bool {
	return c.usesImageVerifyCache
}

func (c *configuration) UsesLeaderElection() bool {
	return c.usesLeaderElection
}

func (c *configuration) UsesKyvernoClient() bool {
	return c.usesKyvernoClient
}

func (c *configuration) UsesDynamicClient() bool {
	return c.usesDynamicClient
}

func (c *configuration) UsesApiServerClient() bool {
	return c.usesApiServerClient
}

func (c *configuration) UsesMetadataClient() bool {
	return c.usesMetadataClient
}

func (c *configuration) UsesKyvernoDynamicClient() bool {
	return c.usesKyvernoDynamicClient
}

func (c *configuration) FlagSets() []*flag.FlagSet {
	return c.flagSets
}
