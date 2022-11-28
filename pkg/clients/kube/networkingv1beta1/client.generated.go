package client

import (
	"github.com/go-logr/logr"
	ingressclasses "github.com/kyverno/kyverno/pkg/clients/kube/networkingv1beta1/ingressclasses"
	ingresses "github.com/kyverno/kyverno/pkg/clients/kube/networkingv1beta1/ingresses"
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
func (c *withMetrics) IngressClasses() k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressClassInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "IngressClass", c.clientType)
	return ingressclasses.WithMetrics(c.inner.IngressClasses(), recorder)
}
func (c *withMetrics) Ingresses(namespace string) k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Ingress", c.clientType)
	return ingresses.WithMetrics(c.inner.Ingresses(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) IngressClasses() k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressClassInterface {
	return ingressclasses.WithTracing(c.inner.IngressClasses(), c.client, "IngressClass")
}
func (c *withTracing) Ingresses(namespace string) k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressInterface {
	return ingresses.WithTracing(c.inner.Ingresses(namespace), c.client, "Ingress")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_networking_v1beta1.NetworkingV1beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) IngressClasses() k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressClassInterface {
	return ingressclasses.WithLogging(c.inner.IngressClasses(), c.logger.WithValues("resource", "IngressClasses"))
}
func (c *withLogging) Ingresses(namespace string) k8s_io_client_go_kubernetes_typed_networking_v1beta1.IngressInterface {
	return ingresses.WithLogging(c.inner.Ingresses(namespace), c.logger.WithValues("resource", "Ingresses").WithValues("namespace", namespace))
}
