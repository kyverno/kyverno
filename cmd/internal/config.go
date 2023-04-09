package internal

import (
	"context"
	"flag"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	genericconfigmapcontroller "github.com/kyverno/kyverno/pkg/controllers/generic/configmap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	resyncPeriod = 15 * time.Minute
)

type Configuration interface {
	UsesMetrics() bool
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

func WithFlagSets(flagsets ...*flag.FlagSet) ConfigurationOption {
	return func(c *configuration) {
		c.flagSets = append(c.flagSets, flagsets...)
	}
}

type configuration struct {
	usesMetrics    bool
	usesTracing    bool
	usesProfiling  bool
	usesKubeconfig bool
	flagSets       []*flag.FlagSet
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

func (c *configuration) FlagSets() []*flag.FlagSet {
	return c.flagSets
}

func StartConfigController(ctx context.Context, logger logr.Logger, client kubernetes.Interface, skipResourceFilters bool) config.Configuration {
	configuration := config.NewDefaultConfiguration(skipResourceFilters)
	configurationController := genericconfigmapcontroller.NewController(
		"config-controller",
		client,
		resyncPeriod,
		config.KyvernoNamespace(),
		config.KyvernoConfigMapName(),
		func(ctx context.Context, cm *corev1.ConfigMap) error {
			configuration.Load(cm)
			return nil
		},
	)
	checkError(logger, configurationController.WarmUp(ctx), "failed to init config controller")
	return configuration
}
