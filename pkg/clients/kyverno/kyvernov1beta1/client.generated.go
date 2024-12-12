package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	updaterequests "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1beta1/updaterequests"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface, client string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface, logger logr.Logger) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) UpdateRequests(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.UpdateRequestInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "UpdateRequest", c.clientType)
	return updaterequests.WithMetrics(c.inner.UpdateRequests(namespace), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) UpdateRequests(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.UpdateRequestInterface {
	return updaterequests.WithTracing(c.inner.UpdateRequests(namespace), c.client, "UpdateRequest")
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) UpdateRequests(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.UpdateRequestInterface {
	return updaterequests.WithLogging(c.inner.UpdateRequests(namespace), c.logger.WithValues("resource", "UpdateRequests").WithValues("namespace", namespace))
}
