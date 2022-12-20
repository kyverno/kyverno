package client

import (
	"github.com/go-logr/logr"
	runtimeclasses "github.com/kyverno/kyverno/pkg/clients/kube/nodev1/runtimeclasses"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_node_v1 "k8s.io/client-go/kubernetes/typed/node/v1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface, client string) k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) RuntimeClasses() k8s_io_client_go_kubernetes_typed_node_v1.RuntimeClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "RuntimeClass", c.clientType)
	return runtimeclasses.WithMetrics(c.inner.RuntimeClasses(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) RuntimeClasses() k8s_io_client_go_kubernetes_typed_node_v1.RuntimeClassInterface {
	return runtimeclasses.WithTracing(c.inner.RuntimeClasses(), c.client, "RuntimeClass")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_node_v1.NodeV1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) RuntimeClasses() k8s_io_client_go_kubernetes_typed_node_v1.RuntimeClassInterface {
	return runtimeclasses.WithLogging(c.inner.RuntimeClasses(), c.logger.WithValues("resource", "RuntimeClasses"))
}
