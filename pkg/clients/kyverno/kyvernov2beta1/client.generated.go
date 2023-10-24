package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2beta1"
	cleanuppolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2beta1/cleanuppolicies"
	clustercleanuppolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2beta1/clustercleanuppolicies"
	clusterpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2beta1/clusterpolicies"
	policies "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2beta1/policies"
	policyexceptions "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2beta1/policyexceptions"
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
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "CleanupPolicy", c.clientType)
	return cleanuppolicies.WithMetrics(c.inner.CleanupPolicies(namespace), recorder)
}
func (c *withMetrics) ClusterCleanupPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterCleanupPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterCleanupPolicy", c.clientType)
	return clustercleanuppolicies.WithMetrics(c.inner.ClusterCleanupPolicies(), recorder)
}
func (c *withMetrics) ClusterPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterPolicy", c.clientType)
	return clusterpolicies.WithMetrics(c.inner.ClusterPolicies(), recorder)
}
func (c *withMetrics) Policies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Policy", c.clientType)
	return policies.WithMetrics(c.inner.Policies(namespace), recorder)
}
func (c *withMetrics) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyExceptionInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PolicyException", c.clientType)
	return policyexceptions.WithMetrics(c.inner.PolicyExceptions(namespace), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) CleanupPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.CleanupPolicyInterface {
	return cleanuppolicies.WithTracing(c.inner.CleanupPolicies(namespace), c.client, "CleanupPolicy")
}
func (c *withTracing) ClusterCleanupPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterCleanupPolicyInterface {
	return clustercleanuppolicies.WithTracing(c.inner.ClusterCleanupPolicies(), c.client, "ClusterCleanupPolicy")
}
func (c *withTracing) ClusterPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterPolicyInterface {
	return clusterpolicies.WithTracing(c.inner.ClusterPolicies(), c.client, "ClusterPolicy")
}
func (c *withTracing) Policies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyInterface {
	return policies.WithTracing(c.inner.Policies(namespace), c.client, "Policy")
}
func (c *withTracing) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyExceptionInterface {
	return policyexceptions.WithTracing(c.inner.PolicyExceptions(namespace), c.client, "PolicyException")
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) CleanupPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.CleanupPolicyInterface {
	return cleanuppolicies.WithLogging(c.inner.CleanupPolicies(namespace), c.logger.WithValues("resource", "CleanupPolicies").WithValues("namespace", namespace))
}
func (c *withLogging) ClusterCleanupPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterCleanupPolicyInterface {
	return clustercleanuppolicies.WithLogging(c.inner.ClusterCleanupPolicies(), c.logger.WithValues("resource", "ClusterCleanupPolicies"))
}
func (c *withLogging) ClusterPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterPolicyInterface {
	return clusterpolicies.WithLogging(c.inner.ClusterPolicies(), c.logger.WithValues("resource", "ClusterPolicies"))
}
func (c *withLogging) Policies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyInterface {
	return policies.WithLogging(c.inner.Policies(namespace), c.logger.WithValues("resource", "Policies").WithValues("namespace", namespace))
}
func (c *withLogging) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyExceptionInterface {
	return policyexceptions.WithLogging(c.inner.PolicyExceptions(namespace), c.logger.WithValues("resource", "PolicyExceptions").WithValues("namespace", namespace))
}
