package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/openreports.io/v1alpha1"
	clusterreports "github.com/kyverno/kyverno/pkg/clients/kyverno/openreportsv1alpha1/clusterreports"
	reports "github.com/kyverno/kyverno/pkg/clients/kyverno/openreportsv1alpha1/reports"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1.OpenreportsV1alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1.OpenreportsV1alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1.OpenreportsV1alpha1Interface, client string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1.OpenreportsV1alpha1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1.OpenreportsV1alpha1Interface, logger logr.Logger) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1.OpenreportsV1alpha1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1.OpenreportsV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) ClusterReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1.ClusterReportInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterReport", c.clientType)
	return clusterreports.WithMetrics(c.inner.ClusterReports(), recorder)
}
func (c *withMetrics) Reports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1.ReportInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Report", c.clientType)
	return reports.WithMetrics(c.inner.Reports(namespace), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1.OpenreportsV1alpha1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) ClusterReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1.ClusterReportInterface {
	return clusterreports.WithTracing(c.inner.ClusterReports(), c.client, "ClusterReport")
}
func (c *withTracing) Reports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1.ReportInterface {
	return reports.WithTracing(c.inner.Reports(namespace), c.client, "Report")
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1.OpenreportsV1alpha1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) ClusterReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1.ClusterReportInterface {
	return clusterreports.WithLogging(c.inner.ClusterReports(), c.logger.WithValues("resource", "ClusterReports"))
}
func (c *withLogging) Reports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_openreports_io_v1alpha1.ReportInterface {
	return reports.WithLogging(c.inner.Reports(namespace), c.logger.WithValues("resource", "Reports").WithValues("namespace", namespace))
}
