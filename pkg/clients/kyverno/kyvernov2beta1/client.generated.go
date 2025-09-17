package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2beta1"
	globalcontextentries "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2beta1/globalcontextentries"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
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

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) CleanupPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.CleanupPolicyInterface {
	return c.inner.CleanupPolicies(namespace)
}
func (c *withMetrics) ClusterCleanupPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterCleanupPolicyInterface {
	return c.inner.ClusterCleanupPolicies()
}
func (c *withMetrics) ClusterPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterPolicyInterface {
	return c.inner.ClusterPolicies()
}
func (c *withMetrics) GlobalContextEntries() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.GlobalContextEntryInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "GlobalContextEntry", c.clientType)
	return globalcontextentries.WithMetrics(c.inner.GlobalContextEntries(), recorder)
}
func (c *withMetrics) Policies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyInterface {
	return c.inner.Policies(namespace)
}
func (c *withMetrics) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyExceptionInterface {
	return c.inner.PolicyExceptions(namespace)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) CleanupPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.CleanupPolicyInterface {
	return c.inner.CleanupPolicies(namespace)
}
func (c *withTracing) ClusterCleanupPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterCleanupPolicyInterface {
	return c.inner.ClusterCleanupPolicies()
}
func (c *withTracing) ClusterPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterPolicyInterface {
	return c.inner.ClusterPolicies()
}
func (c *withTracing) GlobalContextEntries() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.GlobalContextEntryInterface {
	return globalcontextentries.WithTracing(c.inner.GlobalContextEntries(), c.client, "GlobalContextEntry")
}
func (c *withTracing) Policies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyInterface {
	return c.inner.Policies(namespace)
}
func (c *withTracing) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyExceptionInterface {
	return c.inner.PolicyExceptions(namespace)
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) CleanupPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.CleanupPolicyInterface {
	return c.inner.CleanupPolicies(namespace)
}
func (c *withLogging) ClusterCleanupPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterCleanupPolicyInterface {
	return c.inner.ClusterCleanupPolicies()
}
func (c *withLogging) ClusterPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterPolicyInterface {
	return c.inner.ClusterPolicies()
}
func (c *withLogging) GlobalContextEntries() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.GlobalContextEntryInterface {
	return globalcontextentries.WithLogging(c.inner.GlobalContextEntries(), c.logger.WithValues("resource", "GlobalContextEntries"))
}
func (c *withLogging) Policies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyInterface {
	return c.inner.Policies(namespace)
}
func (c *withLogging) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyExceptionInterface {
	return c.inner.PolicyExceptions(namespace)
}
