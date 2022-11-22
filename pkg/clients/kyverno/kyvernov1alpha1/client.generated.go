package client

import (
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha1"
	cleanuppolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1alpha1/cleanuppolicies"
	clustercleanuppolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1alpha1/clustercleanuppolicies"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface, client string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface {
	return &withTracing{inner, client}
}

type withMetrics struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) CleanupPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.CleanupPolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "CleanupPolicy", c.clientType)
	return cleanuppolicies.WithMetrics(c.inner.CleanupPolicies(namespace), recorder)
}
func (c *withMetrics) ClusterCleanupPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.ClusterCleanupPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterCleanupPolicy", c.clientType)
	return clustercleanuppolicies.WithMetrics(c.inner.ClusterCleanupPolicies(), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) CleanupPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.CleanupPolicyInterface {
	return cleanuppolicies.WithTracing(c.inner.CleanupPolicies(namespace), c.client, "CleanupPolicy")
}
func (c *withTracing) ClusterCleanupPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.ClusterCleanupPolicyInterface {
	return clustercleanuppolicies.WithTracing(c.inner.ClusterCleanupPolicies(), c.client, "ClusterCleanupPolicy")
}
