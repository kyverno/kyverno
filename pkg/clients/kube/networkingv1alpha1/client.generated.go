package client

import (
	"github.com/go-logr/logr"
	clustercidrs "github.com/kyverno/kyverno/pkg/clients/kube/networkingv1alpha1/clustercidrs"
	ipaddresses "github.com/kyverno/kyverno/pkg/clients/kube/networkingv1alpha1/ipaddresses"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_networking_v1alpha1 "k8s.io/client-go/kubernetes/typed/networking/v1alpha1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface, client string) k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) ClusterCIDRs() k8s_io_client_go_kubernetes_typed_networking_v1alpha1.ClusterCIDRInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterCIDR", c.clientType)
	return clustercidrs.WithMetrics(c.inner.ClusterCIDRs(), recorder)
}
func (c *withMetrics) IPAddresses() k8s_io_client_go_kubernetes_typed_networking_v1alpha1.IPAddressInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "IPAddress", c.clientType)
	return ipaddresses.WithMetrics(c.inner.IPAddresses(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) ClusterCIDRs() k8s_io_client_go_kubernetes_typed_networking_v1alpha1.ClusterCIDRInterface {
	return clustercidrs.WithTracing(c.inner.ClusterCIDRs(), c.client, "ClusterCIDR")
}
func (c *withTracing) IPAddresses() k8s_io_client_go_kubernetes_typed_networking_v1alpha1.IPAddressInterface {
	return ipaddresses.WithTracing(c.inner.IPAddresses(), c.client, "IPAddress")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_networking_v1alpha1.NetworkingV1alpha1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) ClusterCIDRs() k8s_io_client_go_kubernetes_typed_networking_v1alpha1.ClusterCIDRInterface {
	return clustercidrs.WithLogging(c.inner.ClusterCIDRs(), c.logger.WithValues("resource", "ClusterCIDRs"))
}
func (c *withLogging) IPAddresses() k8s_io_client_go_kubernetes_typed_networking_v1alpha1.IPAddressInterface {
	return ipaddresses.WithLogging(c.inner.IPAddresses(), c.logger.WithValues("resource", "IPAddresses"))
}
