package clientset

import (
	"net/http"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_metadata "k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
)

type Interface interface {
	k8s_io_client_go_metadata.Interface
	WithMetrics(metrics.MetricsConfigManager, metrics.ClientType) Interface
	WithTracing() Interface
	WithLogging(logr.Logger) Interface
}

func From(inner k8s_io_client_go_metadata.Interface, opts ...NewOption) Interface {
	i := from(inner)
	for _, opt := range opts {
		i = opt(i)
	}
	return i
}

type NewOption func(Interface) Interface

func WithMetrics(m metrics.MetricsConfigManager, t metrics.ClientType) NewOption {
	return func(i Interface) Interface {
		return i.WithMetrics(m, t)
	}
}

func WithTracing() NewOption {
	return func(i Interface) Interface {
		return i.WithTracing()
	}
}

func WithLogging(logger logr.Logger) NewOption {
	return func(i Interface) Interface {
		return i.WithLogging(logger)
	}
}

func NewForConfig(c *rest.Config, opts ...NewOption) (Interface, error) {
	inner, err := k8s_io_client_go_metadata.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	return From(inner, opts...), nil
}

func NewForConfigAndClient(c *rest.Config, httpClient *http.Client, opts ...NewOption) (Interface, error) {
	inner, err := k8s_io_client_go_metadata.NewForConfigAndClient(c, httpClient)
	if err != nil {
		return nil, err
	}
	return From(inner, opts...), nil
}

func NewForConfigOrDie(c *rest.Config, opts ...NewOption) Interface {
	return From(k8s_io_client_go_metadata.NewForConfigOrDie(c), opts...)
}

type wrapper struct {
	k8s_io_client_go_metadata.Interface
}

func from(inner k8s_io_client_go_metadata.Interface, opts ...NewOption) Interface {
	return &wrapper{inner}
}

func (i *wrapper) WithMetrics(m metrics.MetricsConfigManager, t metrics.ClientType) Interface {
	return from(WrapWithMetrics(i, m, t))
}

func (i *wrapper) WithTracing() Interface {
	return from(WrapWithTracing(i))
}

func (i *wrapper) WithLogging(logger logr.Logger) Interface {
	return from(WrapWithLogging(i, logger))
}
