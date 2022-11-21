package client

import (
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	clusterpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1/clusterpolicies"
	generaterequests "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1/generaterequests"
	policies "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov1/policies"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface, client string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface {
	return &withTracing{inner, client}
}

type withMetrics struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) ClusterPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.ClusterPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterPolicy", c.clientType)
	return clusterpolicies.WithMetrics(c.inner.ClusterPolicies(), recorder)
}
func (c *withMetrics) GenerateRequests(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.GenerateRequestInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "GenerateRequest", c.clientType)
	return generaterequests.WithMetrics(c.inner.GenerateRequests(namespace), recorder)
}
func (c *withMetrics) Policies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.PolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Policy", c.clientType)
	return policies.WithMetrics(c.inner.Policies(namespace), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) ClusterPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.ClusterPolicyInterface {
	return clusterpolicies.WithTracing(c.inner.ClusterPolicies(), c.client, "ClusterPolicy")
}
func (c *withTracing) GenerateRequests(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.GenerateRequestInterface {
	return generaterequests.WithTracing(c.inner.GenerateRequests(namespace), c.client, "GenerateRequest")
}
func (c *withTracing) Policies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.PolicyInterface {
	return policies.WithTracing(c.inner.Policies(namespace), c.client, "Policy")
}
