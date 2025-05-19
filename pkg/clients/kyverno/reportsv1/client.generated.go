package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/reports/v1"
	clusterephemeralreports "github.com/kyverno/kyverno/pkg/clients/kyverno/reportsv1/clusterephemeralreports"
	ephemeralreports "github.com/kyverno/kyverno/pkg/clients/kyverno/reportsv1/ephemeralreports"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1.ReportsV1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1.ReportsV1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1.ReportsV1Interface, client string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1.ReportsV1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1.ReportsV1Interface, logger logr.Logger) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1.ReportsV1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1.ReportsV1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) ClusterEphemeralReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1.ClusterEphemeralReportInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterEphemeralReport", c.clientType)
	return clusterephemeralreports.WithMetrics(c.inner.ClusterEphemeralReports(), recorder)
}
func (c *withMetrics) EphemeralReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1.EphemeralReportInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "EphemeralReport", c.clientType)
	return ephemeralreports.WithMetrics(c.inner.EphemeralReports(namespace), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1.ReportsV1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) ClusterEphemeralReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1.ClusterEphemeralReportInterface {
	return clusterephemeralreports.WithTracing(c.inner.ClusterEphemeralReports(), c.client, "ClusterEphemeralReport")
}
func (c *withTracing) EphemeralReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1.EphemeralReportInterface {
	return ephemeralreports.WithTracing(c.inner.EphemeralReports(namespace), c.client, "EphemeralReport")
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1.ReportsV1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) ClusterEphemeralReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1.ClusterEphemeralReportInterface {
	return clusterephemeralreports.WithLogging(c.inner.ClusterEphemeralReports(), c.logger.WithValues("resource", "ClusterEphemeralReports"))
}
func (c *withLogging) EphemeralReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_reports_v1.EphemeralReportInterface {
	return ephemeralreports.WithLogging(c.inner.EphemeralReports(namespace), c.logger.WithValues("resource", "EphemeralReports").WithValues("namespace", namespace))
}
