package client

import (
	"github.com/go-logr/logr"
	certificatesigningrequests "github.com/kyverno/kyverno/pkg/clients/kube/certificatesv1beta1/certificatesigningrequests"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_certificates_v1beta1 "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface, client string) k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) CertificateSigningRequests() k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificateSigningRequestInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "CertificateSigningRequest", c.clientType)
	return certificatesigningrequests.WithMetrics(c.inner.CertificateSigningRequests(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) CertificateSigningRequests() k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificateSigningRequestInterface {
	return certificatesigningrequests.WithTracing(c.inner.CertificateSigningRequests(), c.client, "CertificateSigningRequest")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificatesV1beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) CertificateSigningRequests() k8s_io_client_go_kubernetes_typed_certificates_v1beta1.CertificateSigningRequestInterface {
	return certificatesigningrequests.WithLogging(c.inner.CertificateSigningRequests(), c.logger.WithValues("resource", "CertificateSigningRequests"))
}
