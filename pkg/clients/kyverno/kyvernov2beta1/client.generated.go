package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2beta1"
	globalcontextentries "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2beta1/globalcontextentries"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8sclientgorest "k8s.io/client-go/rest"
)

func WithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface, client string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface, logger logr.Logger) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() k8sclientgorest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) GlobalContextEntries() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.GlobalContextEntryInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "GlobalContextEntry", c.clientType)
	return globalcontextentries.WithMetrics(c.inner.GlobalContextEntries(), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface
	client string
}

func (c *withTracing) RESTClient() k8sclientgorest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) GlobalContextEntries() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.GlobalContextEntryInterface {
	return globalcontextentries.WithTracing(c.inner.GlobalContextEntries(), c.client, "GlobalContextEntry")
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() k8sclientgorest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) GlobalContextEntries() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.GlobalContextEntryInterface {
	return globalcontextentries.WithLogging(c.inner.GlobalContextEntries(), c.logger.WithValues("resource", "GlobalContextEntries"))
}
