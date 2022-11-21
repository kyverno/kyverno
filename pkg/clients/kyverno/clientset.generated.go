package client

import (
	net_http "net/http"

	github_com_kyverno_kyverno_pkg_client_clientset_versioned "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	github_com_kyverno_kyverno_pkg_clients_middleware_metrics_kyverno "github.com/kyverno/kyverno/pkg/clients/middleware/metrics/kyverno"
	github_com_kyverno_kyverno_pkg_clients_middleware_tracing_kyverno "github.com/kyverno/kyverno/pkg/clients/middleware/tracing/kyverno"
	github_com_kyverno_kyverno_pkg_metrics "github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_rest "k8s.io/client-go/rest"
)

type Interface interface {
	github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface
	WithMetrics(m github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager, t github_com_kyverno_kyverno_pkg_metrics.ClientType) Interface
	WithTracing() Interface
}

type wrapper struct {
	github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface
}

type NewOption func(Interface) Interface

func NewForConfig(c *k8s_io_client_go_rest.Config, opts ...NewOption) (Interface, error) {
	inner, err := github_com_kyverno_kyverno_pkg_client_clientset_versioned.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	return From(inner, opts...), nil
}

func NewForConfigAndClient(c *k8s_io_client_go_rest.Config, httpClient *net_http.Client, opts ...NewOption) (Interface, error) {
	inner, err := github_com_kyverno_kyverno_pkg_client_clientset_versioned.NewForConfigAndClient(c, httpClient)
	if err != nil {
		return nil, err
	}
	return From(inner, opts...), nil
}

func NewForConfigOrDie(c *k8s_io_client_go_rest.Config, opts ...NewOption) Interface {
	return From(github_com_kyverno_kyverno_pkg_client_clientset_versioned.NewForConfigOrDie(c), opts...)
}

func New(c k8s_io_client_go_rest.Interface, opts ...NewOption) Interface {
	return From(github_com_kyverno_kyverno_pkg_client_clientset_versioned.New(c), opts...)
}

func from(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface, opts ...NewOption) Interface {
	return &wrapper{inner}
}

func From(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface, opts ...NewOption) Interface {
	i := from(inner)
	for _, opt := range opts {
		i = opt(i)
	}
	return i
}

func (i *wrapper) WithMetrics(m github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager, t github_com_kyverno_kyverno_pkg_metrics.ClientType) Interface {
	return from(github_com_kyverno_kyverno_pkg_clients_middleware_metrics_kyverno.Wrap(i, m, t))
}

func WithMetrics(m github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager, t github_com_kyverno_kyverno_pkg_metrics.ClientType) NewOption {
	return func(i Interface) Interface {
		return i.WithMetrics(m, t)
	}
}

func (i *wrapper) WithTracing() Interface {
	return from(github_com_kyverno_kyverno_pkg_clients_middleware_tracing_kyverno.Wrap(i))
}

func WithTracing() NewOption {
	return func(i Interface) Interface {
		return i.WithTracing()
	}
}
