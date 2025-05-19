package client

import (
	"github.com/go-logr/logr"
	ingressclasses "github.com/kyverno/kyverno/pkg/clients/kube/networkingv1beta1/ingressclasses"
	ingresses "github.com/kyverno/kyverno/pkg/clients/kube/networkingv1beta1/ingresses"
	ipaddresses "github.com/kyverno/kyverno/pkg/clients/kube/networkingv1beta1/ipaddresses"
	servicecidrs "github.com/kyverno/kyverno/pkg/clients/kube/networkingv1beta1/servicecidrs"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_networking_v1beta1 "k8s.io/client-go/kubernetes/typed/networking/v1beta1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface, client string) k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) IPAddresses() k8s_io_client_go_kubernetes_typed_networking_v1beta1.IPAddressInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "IPAddress", c.clientType)
	return ipaddresses.WithMetrics(c.inner.IPAddresses(), recorder)
}
func (c *withMetrics) IngressClasses() k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "IngressClass", c.clientType)
	return ingressclasses.WithMetrics(c.inner.IngressClasses(), recorder)
}
func (c *withMetrics) Ingresses(namespace string) k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Ingress", c.clientType)
	return ingresses.WithMetrics(c.inner.Ingresses(namespace), recorder)
}
func (c *withMetrics) ServiceCIDRs() k8s_io_client_go_kubernetes_typed_networking_v1beta1.ServiceCIDRInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ServiceCIDR", c.clientType)
	return servicecidrs.WithMetrics(c.inner.ServiceCIDRs(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) IPAddresses() k8s_io_client_go_kubernetes_typed_networking_v1beta1.IPAddressInterface {
	return ipaddresses.WithTracing(c.inner.IPAddresses(), c.client, "IPAddress")
}
func (c *withTracing) IngressClasses() k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressClassInterface {
	return ingressclasses.WithTracing(c.inner.IngressClasses(), c.client, "IngressClass")
}
func (c *withTracing) Ingresses(namespace string) k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressInterface {
	return ingresses.WithTracing(c.inner.Ingresses(namespace), c.client, "Ingress")
}
func (c *withTracing) ServiceCIDRs() k8s_io_client_go_kubernetes_typed_networking_v1beta1.ServiceCIDRInterface {
	return servicecidrs.WithTracing(c.inner.ServiceCIDRs(), c.client, "ServiceCIDR")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) IPAddresses() k8s_io_client_go_kubernetes_typed_networking_v1beta1.IPAddressInterface {
	return ipaddresses.WithLogging(c.inner.IPAddresses(), c.logger.WithValues("resource", "IPAddresses"))
}
func (c *withLogging) IngressClasses() k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressClassInterface {
	return ingressclasses.WithLogging(c.inner.IngressClasses(), c.logger.WithValues("resource", "IngressClasses"))
}
func (c *withLogging) Ingresses(namespace string) k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressInterface {
	return ingresses.WithLogging(c.inner.Ingresses(namespace), c.logger.WithValues("resource", "Ingresses").WithValues("namespace", namespace))
}
func (c *withLogging) ServiceCIDRs() k8s_io_client_go_kubernetes_typed_networking_v1beta1.ServiceCIDRInterface {
	return servicecidrs.WithLogging(c.inner.ServiceCIDRs(), c.logger.WithValues("resource", "ServiceCIDRs"))
}
