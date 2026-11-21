package client

import (
	"github.com/go-logr/logr"
	evictionrequests "github.com/kyverno/kyverno/pkg/clients/kube/lifecyclev1alpha1/evictionrequests"
	evictions "github.com/kyverno/kyverno/pkg/clients/kube/lifecyclev1alpha1/evictions"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1 "k8s.io/client-go/kubernetes/typed/lifecycle/v1alpha1"
	k8sclientgorest "k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1.LifecycleV1alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1.LifecycleV1alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1.LifecycleV1alpha1Interface, client string) k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1.LifecycleV1alpha1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1.LifecycleV1alpha1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1.LifecycleV1alpha1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1.LifecycleV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() k8sclientgorest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) EvictionRequests(namespace string) k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1.EvictionRequestInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "EvictionRequest", c.clientType)
	return evictionrequests.WithMetrics(c.inner.EvictionRequests(namespace), recorder)
}
func (c *withMetrics) Evictions(namespace string) k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1.EvictionInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Eviction", c.clientType)
	return evictions.WithMetrics(c.inner.Evictions(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1.LifecycleV1alpha1Interface
	client string
}

func (c *withTracing) RESTClient() k8sclientgorest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) EvictionRequests(namespace string) k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1.EvictionRequestInterface {
	return evictionrequests.WithTracing(c.inner.EvictionRequests(namespace), c.client, "EvictionRequest")
}
func (c *withTracing) Evictions(namespace string) k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1.EvictionInterface {
	return evictions.WithTracing(c.inner.Evictions(namespace), c.client, "Eviction")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1.LifecycleV1alpha1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() k8sclientgorest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) EvictionRequests(namespace string) k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1.EvictionRequestInterface {
	return evictionrequests.WithLogging(c.inner.EvictionRequests(namespace), c.logger.WithValues("resource", "EvictionRequests").WithValues("namespace", namespace))
}
func (c *withLogging) Evictions(namespace string) k8s_io_client_go_kubernetes_typed_lifecycle_v1alpha1.EvictionInterface {
	return evictions.WithLogging(c.inner.Evictions(namespace), c.logger.WithValues("resource", "Evictions").WithValues("namespace", namespace))
}
