package client

import (
	"github.com/go-logr/logr"
	leases "github.com/kyverno/kyverno/pkg/clients/kube/coordinationv1/leases"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_coordination_v1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface, client string) k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) Leases(namespace string) k8s_io_client_go_kubernetes_typed_coordination_v1.LeaseInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Lease", c.clientType)
	return leases.WithMetrics(c.inner.Leases(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) Leases(namespace string) k8s_io_client_go_kubernetes_typed_coordination_v1.LeaseInterface {
	return leases.WithTracing(c.inner.Leases(namespace), c.client, "Lease")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_coordination_v1.CoordinationV1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) Leases(namespace string) k8s_io_client_go_kubernetes_typed_coordination_v1.LeaseInterface {
	return leases.WithLogging(c.inner.Leases(namespace), c.logger.WithValues("resource", "Leases").WithValues("namespace", namespace))
}
