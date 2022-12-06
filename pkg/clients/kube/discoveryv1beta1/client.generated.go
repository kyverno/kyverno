package client

import (
	"github.com/go-logr/logr"
	endpointslices "github.com/kyverno/kyverno/pkg/clients/kube/discoveryv1beta1/endpointslices"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_discovery_v1beta1 "k8s.io/client-go/kubernetes/typed/discovery/v1beta1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface, client string) k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) EndpointSlices(namespace string) k8s_io_client_go_kubernetes_typed_discovery_v1beta1.EndpointSliceInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "EndpointSlice", c.clientType)
	return endpointslices.WithMetrics(c.inner.EndpointSlices(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) EndpointSlices(namespace string) k8s_io_client_go_kubernetes_typed_discovery_v1beta1.EndpointSliceInterface {
	return endpointslices.WithTracing(c.inner.EndpointSlices(namespace), c.client, "EndpointSlice")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_discovery_v1beta1.DiscoveryV1beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) EndpointSlices(namespace string) k8s_io_client_go_kubernetes_typed_discovery_v1beta1.EndpointSliceInterface {
	return endpointslices.WithLogging(c.inner.EndpointSlices(namespace), c.logger.WithValues("resource", "EndpointSlices").WithValues("namespace", namespace))
}
