package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2"
	admissionreports "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2/admissionreports"
	backgroundscanreports "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2/backgroundscanreports"
	cleanuppolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2/cleanuppolicies"
	clusteradmissionreports "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2/clusteradmissionreports"
	clusterbackgroundscanreports "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2/clusterbackgroundscanreports"
	clustercleanuppolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2/clustercleanuppolicies"
	policyexceptions "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2/policyexceptions"
	updaterequests "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2/updaterequests"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
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

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) AdmissionReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.AdmissionReportInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "AdmissionReport", c.clientType)
	return admissionreports.WithMetrics(c.inner.AdmissionReports(namespace), recorder)
}
func (c *withMetrics) BackgroundScanReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.BackgroundScanReportInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "BackgroundScanReport", c.clientType)
	return backgroundscanreports.WithMetrics(c.inner.BackgroundScanReports(namespace), recorder)
}
func (c *withMetrics) CleanupPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.CleanupPolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "CleanupPolicy", c.clientType)
	return cleanuppolicies.WithMetrics(c.inner.CleanupPolicies(namespace), recorder)
}
func (c *withMetrics) ClusterAdmissionReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.ClusterAdmissionReportInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterAdmissionReport", c.clientType)
	return clusteradmissionreports.WithMetrics(c.inner.ClusterAdmissionReports(), recorder)
}
func (c *withMetrics) ClusterBackgroundScanReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.ClusterBackgroundScanReportInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterBackgroundScanReport", c.clientType)
	return clusterbackgroundscanreports.WithMetrics(c.inner.ClusterBackgroundScanReports(), recorder)
}
func (c *withMetrics) ClusterCleanupPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.ClusterCleanupPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterCleanupPolicy", c.clientType)
	return clustercleanuppolicies.WithMetrics(c.inner.ClusterCleanupPolicies(), recorder)
}
func (c *withMetrics) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.PolicyExceptionInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PolicyException", c.clientType)
	return policyexceptions.WithMetrics(c.inner.PolicyExceptions(namespace), recorder)
}
func (c *withMetrics) UpdateRequests(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.UpdateRequestInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "UpdateRequest", c.clientType)
	return updaterequests.WithMetrics(c.inner.UpdateRequests(namespace), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) AdmissionReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.AdmissionReportInterface {
	return admissionreports.WithTracing(c.inner.AdmissionReports(namespace), c.client, "AdmissionReport")
}
func (c *withTracing) BackgroundScanReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.BackgroundScanReportInterface {
	return backgroundscanreports.WithTracing(c.inner.BackgroundScanReports(namespace), c.client, "BackgroundScanReport")
}
func (c *withTracing) CleanupPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.CleanupPolicyInterface {
	return cleanuppolicies.WithTracing(c.inner.CleanupPolicies(namespace), c.client, "CleanupPolicy")
}
func (c *withTracing) ClusterAdmissionReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.ClusterAdmissionReportInterface {
	return clusteradmissionreports.WithTracing(c.inner.ClusterAdmissionReports(), c.client, "ClusterAdmissionReport")
}
func (c *withTracing) ClusterBackgroundScanReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.ClusterBackgroundScanReportInterface {
	return clusterbackgroundscanreports.WithTracing(c.inner.ClusterBackgroundScanReports(), c.client, "ClusterBackgroundScanReport")
}
func (c *withTracing) ClusterCleanupPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.ClusterCleanupPolicyInterface {
	return clustercleanuppolicies.WithTracing(c.inner.ClusterCleanupPolicies(), c.client, "ClusterCleanupPolicy")
}
func (c *withTracing) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.PolicyExceptionInterface {
	return policyexceptions.WithTracing(c.inner.PolicyExceptions(namespace), c.client, "PolicyException")
}
func (c *withTracing) UpdateRequests(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.UpdateRequestInterface {
	return updaterequests.WithTracing(c.inner.UpdateRequests(namespace), c.client, "UpdateRequest")
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.KyvernoV2Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) AdmissionReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.AdmissionReportInterface {
	return admissionreports.WithLogging(c.inner.AdmissionReports(namespace), c.logger.WithValues("resource", "AdmissionReports").WithValues("namespace", namespace))
}
func (c *withLogging) BackgroundScanReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.BackgroundScanReportInterface {
	return backgroundscanreports.WithLogging(c.inner.BackgroundScanReports(namespace), c.logger.WithValues("resource", "BackgroundScanReports").WithValues("namespace", namespace))
}
func (c *withLogging) CleanupPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.CleanupPolicyInterface {
	return cleanuppolicies.WithLogging(c.inner.CleanupPolicies(namespace), c.logger.WithValues("resource", "CleanupPolicies").WithValues("namespace", namespace))
}
func (c *withLogging) ClusterAdmissionReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.ClusterAdmissionReportInterface {
	return clusteradmissionreports.WithLogging(c.inner.ClusterAdmissionReports(), c.logger.WithValues("resource", "ClusterAdmissionReports"))
}
func (c *withLogging) ClusterBackgroundScanReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.ClusterBackgroundScanReportInterface {
	return clusterbackgroundscanreports.WithLogging(c.inner.ClusterBackgroundScanReports(), c.logger.WithValues("resource", "ClusterBackgroundScanReports"))
}
func (c *withLogging) ClusterCleanupPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.ClusterCleanupPolicyInterface {
	return clustercleanuppolicies.WithLogging(c.inner.ClusterCleanupPolicies(), c.logger.WithValues("resource", "ClusterCleanupPolicies"))
}
func (c *withLogging) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.PolicyExceptionInterface {
	return policyexceptions.WithLogging(c.inner.PolicyExceptions(namespace), c.logger.WithValues("resource", "PolicyExceptions").WithValues("namespace", namespace))
}
func (c *withLogging) UpdateRequests(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2.UpdateRequestInterface {
	return updaterequests.WithLogging(c.inner.UpdateRequests(namespace), c.logger.WithValues("resource", "UpdateRequests").WithValues("namespace", namespace))
}
