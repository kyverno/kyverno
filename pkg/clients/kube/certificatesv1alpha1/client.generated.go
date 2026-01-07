package client

import (
	"github.com/go-logr/logr"
	clustertrustbundles "github.com/kyverno/kyverno/pkg/clients/kube/certificatesv1alpha1/clustertrustbundles"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_certificates_v1alpha1 "k8s.io/client-go/kubernetes/typed/certificates/v1alpha1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_certificates_v1alpha1.CertificatesV1alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_certificates_v1alpha1.CertificatesV1alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_certificates_v1alpha1.CertificatesV1alpha1Interface, client string) k8s_io_client_go_kubernetes_typed_certificates_v1alpha1.CertificatesV1alpha1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_certificates_v1alpha1.CertificatesV1alpha1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_certificates_v1alpha1.CertificatesV1alpha1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_certificates_v1alpha1.CertificatesV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) ClusterTrustBundles() k8s_io_client_go_kubernetes_typed_certificates_v1alpha1.ClusterTrustBundleInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterTrustBundle", c.clientType)
	return clustertrustbundles.WithMetrics(c.inner.ClusterTrustBundles(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_certificates_v1alpha1.CertificatesV1alpha1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) ClusterTrustBundles() k8s_io_client_go_kubernetes_typed_certificates_v1alpha1.ClusterTrustBundleInterface {
	return clustertrustbundles.WithTracing(c.inner.ClusterTrustBundles(), c.client, "ClusterTrustBundle")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_certificates_v1alpha1.CertificatesV1alpha1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) ClusterTrustBundles() k8s_io_client_go_kubernetes_typed_certificates_v1alpha1.ClusterTrustBundleInterface {
	return clustertrustbundles.WithLogging(c.inner.ClusterTrustBundles(), c.logger.WithValues("resource", "ClusterTrustBundles"))
}
