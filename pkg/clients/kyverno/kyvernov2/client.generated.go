package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2"
	globalcontextentries "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2/globalcontextentries"
	updaterequests "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2/updaterequests"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8sclientgorest "k8s.io/client-go/rest"
)

func WithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface, client string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface, logger logr.Logger) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() k8sclientgorest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) GlobalContextEntries() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.GlobalContextEntryInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "GlobalContextEntry", c.clientType)
	return globalcontextentries.WithMetrics(c.inner.GlobalContextEntries(), recorder)
}
func (c *withMetrics) UpdateRequests(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.UpdateRequestInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "UpdateRequest", c.clientType)
	return updaterequests.WithMetrics(c.inner.UpdateRequests(namespace), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface
	client string
}

func (c *withTracing) RESTClient() k8sclientgorest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) GlobalContextEntries() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.GlobalContextEntryInterface {
	return globalcontextentries.WithTracing(c.inner.GlobalContextEntries(), c.client, "GlobalContextEntry")
}
func (c *withTracing) UpdateRequests(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.UpdateRequestInterface {
	return updaterequests.WithTracing(c.inner.UpdateRequests(namespace), c.client, "UpdateRequest")
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() k8sclientgorest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) GlobalContextEntries() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.GlobalContextEntryInterface {
	return globalcontextentries.WithLogging(c.inner.GlobalContextEntries(), c.logger.WithValues("resource", "GlobalContextEntries"))
}
func (c *withLogging) UpdateRequests(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.UpdateRequestInterface {
	return updaterequests.WithLogging(c.inner.UpdateRequests(namespace), c.logger.WithValues("resource", "UpdateRequests").WithValues("namespace", namespace))
}
