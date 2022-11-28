package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	clusterpolicyreports "github.com/kyverno/kyverno/pkg/clients/kyverno/wgpolicyk8sv1alpha2/clusterpolicyreports"
	policyreports "github.com/kyverno/kyverno/pkg/clients/kyverno/wgpolicyk8sv1alpha2/policyreports"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface, client string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface, logger logr.Logger) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) ClusterPolicyReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.ClusterPolicyReportInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterPolicyReport", c.clientType)
	return clusterpolicyreports.WithMetrics(c.inner.ClusterPolicyReports(), recorder)
}
func (c *withMetrics) PolicyReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.PolicyReportInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PolicyReport", c.clientType)
	return policyreports.WithMetrics(c.inner.PolicyReports(namespace), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) ClusterPolicyReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.ClusterPolicyReportInterface {
	return clusterpolicyreports.WithTracing(c.inner.ClusterPolicyReports(), c.client, "ClusterPolicyReport")
}
func (c *withTracing) PolicyReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.PolicyReportInterface {
	return policyreports.WithTracing(c.inner.PolicyReports(namespace), c.client, "PolicyReport")
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) ClusterPolicyReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.ClusterPolicyReportInterface {
	return clusterpolicyreports.WithLogging(c.inner.ClusterPolicyReports(), c.logger.WithValues("resource", "ClusterPolicyReports"))
}
func (c *withLogging) PolicyReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.PolicyReportInterface {
	return policyreports.WithLogging(c.inner.PolicyReports(namespace), c.logger.WithValues("resource", "PolicyReports").WithValues("namespace", namespace))
}
