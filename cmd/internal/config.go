package internal

import "flag"

type Configuration interface {
	UsesTracing() bool
	UsesProfiling() bool
	UsesKubeconfig() bool
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

func WithFlagSets(flagsets ...*flag.FlagSet) ConfigurationOption {
	return func(c *configuration) {
		c.flagSets = append(c.flagSets, flagsets...)
	}
}

type configuration struct {
	usesTracing    bool
	usesProfiling  bool
	usesKubeconfig bool
	flagSets       []*flag.FlagSet
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

func (c *configuration) FlagSets() []*flag.FlagSet {
	return c.flagSets
}
