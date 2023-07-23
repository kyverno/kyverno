package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2beta1"
	clusterpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2beta1/clusterpolicies"
	policies "github.com/kyverno/kyverno/pkg/clients/kyverno/kyvernov2beta1/policies"
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
func (c *withMetrics) ClusterPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterPolicy", c.clientType)
	return clusterpolicies.WithMetrics(c.inner.ClusterPolicies(), recorder)
}
func (c *withMetrics) Policies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Policy", c.clientType)
	return policies.WithMetrics(c.inner.Policies(namespace), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) ClusterPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterPolicyInterface {
	return clusterpolicies.WithTracing(c.inner.ClusterPolicies(), c.client, "ClusterPolicy")
}
func (c *withTracing) Policies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyInterface {
	return policies.WithTracing(c.inner.Policies(namespace), c.client, "Policy")
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) ClusterPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterPolicyInterface {
	return clusterpolicies.WithLogging(c.inner.ClusterPolicies(), c.logger.WithValues("resource", "ClusterPolicies"))
}
func (c *withLogging) Policies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyInterface {
	return policies.WithLogging(c.inner.Policies(namespace), c.logger.WithValues("resource", "Policies").WithValues("namespace", namespace))
}
