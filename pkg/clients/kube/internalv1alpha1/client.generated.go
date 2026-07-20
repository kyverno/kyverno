package client

import (
	"github.com/go-logr/logr"
	storageversions "github.com/kyverno/kyverno/pkg/clients/kube/internalv1alpha1/storageversions"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1 "k8s.io/client-go/kubernetes/typed/apiserverinternal/v1alpha1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface, client string) k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) StorageVersions() k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.StorageVersionInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "StorageVersion", c.clientType)
	return storageversions.WithMetrics(c.inner.StorageVersions(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) StorageVersions() k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.StorageVersionInterface {
	return storageversions.WithTracing(c.inner.StorageVersions(), c.client, "StorageVersion")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.InternalV1alpha1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) StorageVersions() k8s_io_client_go_kubernetes_typed_apiserverinternal_v1alpha1.StorageVersionInterface {
	return storageversions.WithLogging(c.inner.StorageVersions(), c.logger.WithValues("resource", "StorageVersions"))
}
