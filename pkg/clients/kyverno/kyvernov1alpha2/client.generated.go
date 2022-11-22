package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	admissionreports "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1alpha2/admissionreports"
	backgroundscanreports "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1alpha2/backgroundscanreports"
	clusteradmissionreports "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1alpha2/clusteradmissionreports"
	clusterbackgroundscanreports "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1alpha2/clusterbackgroundscanreports"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface, client string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface, logger logr.Logger) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) AdmissionReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.AdmissionReportInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "AdmissionReport", c.clientType)
	return admissionreports.WithMetrics(c.inner.AdmissionReports(namespace), recorder)
}
func (c *withMetrics) BackgroundScanReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.BackgroundScanReportInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "BackgroundScanReport", c.clientType)
	return backgroundscanreports.WithMetrics(c.inner.BackgroundScanReports(namespace), recorder)
}
func (c *withMetrics) ClusterAdmissionReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterAdmissionReportInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterAdmissionReport", c.clientType)
	return clusteradmissionreports.WithMetrics(c.inner.ClusterAdmissionReports(), recorder)
}
func (c *withMetrics) ClusterBackgroundScanReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterBackgroundScanReportInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterBackgroundScanReport", c.clientType)
	return clusterbackgroundscanreports.WithMetrics(c.inner.ClusterBackgroundScanReports(), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) AdmissionReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.AdmissionReportInterface {
	return admissionreports.WithTracing(c.inner.AdmissionReports(namespace), c.client, "AdmissionReport")
}
func (c *withTracing) BackgroundScanReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.BackgroundScanReportInterface {
	return backgroundscanreports.WithTracing(c.inner.BackgroundScanReports(namespace), c.client, "BackgroundScanReport")
}
func (c *withTracing) ClusterAdmissionReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterAdmissionReportInterface {
	return clusteradmissionreports.WithTracing(c.inner.ClusterAdmissionReports(), c.client, "ClusterAdmissionReport")
}
func (c *withTracing) ClusterBackgroundScanReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterBackgroundScanReportInterface {
	return clusterbackgroundscanreports.WithTracing(c.inner.ClusterBackgroundScanReports(), c.client, "ClusterBackgroundScanReport")
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) AdmissionReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.AdmissionReportInterface {
	return admissionreports.WithLogging(c.inner.AdmissionReports(namespace), c.logger.WithValues("resource", "AdmissionReports").WithValues("namespace", namespace))
}
func (c *withLogging) BackgroundScanReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.BackgroundScanReportInterface {
	return backgroundscanreports.WithLogging(c.inner.BackgroundScanReports(namespace), c.logger.WithValues("resource", "BackgroundScanReports").WithValues("namespace", namespace))
}
func (c *withLogging) ClusterAdmissionReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterAdmissionReportInterface {
	return clusteradmissionreports.WithLogging(c.inner.ClusterAdmissionReports(), c.logger.WithValues("resource", "ClusterAdmissionReports"))
}
func (c *withLogging) ClusterBackgroundScanReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterBackgroundScanReportInterface {
	return clusterbackgroundscanreports.WithLogging(c.inner.ClusterBackgroundScanReports(), c.logger.WithValues("resource", "ClusterBackgroundScanReports"))
}
