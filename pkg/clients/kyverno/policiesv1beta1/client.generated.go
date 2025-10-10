package client

import (
	"github.com/go-logr/logr"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policies.kyverno.io/v1beta1"
	deletingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1beta1/deletingpolicies"
	namespaceddeletingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1beta1/namespaceddeletingpolicies"
	validatingpolicies "github.com/kyverno/kyverno/pkg/clients/kyverno/policiesv1beta1/validatingpolicies"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface, client string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface, logger logr.Logger) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) DeletingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.DeletingPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "DeletingPolicy", c.clientType)
	return deletingpolicies.WithMetrics(c.inner.DeletingPolicies(), recorder)
}
func (c *withMetrics) NamespacedDeletingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedDeletingPolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "NamespacedDeletingPolicy", c.clientType)
	return namespaceddeletingpolicies.WithMetrics(c.inner.NamespacedDeletingPolicies(namespace), recorder)
}
func (c *withMetrics) ValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.ValidatingPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ValidatingPolicy", c.clientType)
	return validatingpolicies.WithMetrics(c.inner.ValidatingPolicies(), recorder)
}

type withTracing struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) DeletingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.DeletingPolicyInterface {
	return deletingpolicies.WithTracing(c.inner.DeletingPolicies(), c.client, "DeletingPolicy")
}
func (c *withTracing) NamespacedDeletingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedDeletingPolicyInterface {
	return namespaceddeletingpolicies.WithTracing(c.inner.NamespacedDeletingPolicies(namespace), c.client, "NamespacedDeletingPolicy")
}
func (c *withTracing) ValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.ValidatingPolicyInterface {
	return validatingpolicies.WithTracing(c.inner.ValidatingPolicies(), c.client, "ValidatingPolicy")
}

type withLogging struct {
	inner  github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.PoliciesV1beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) DeletingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.DeletingPolicyInterface {
	return deletingpolicies.WithLogging(c.inner.DeletingPolicies(), c.logger.WithValues("resource", "DeletingPolicies"))
}
func (c *withLogging) NamespacedDeletingPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.NamespacedDeletingPolicyInterface {
	return namespaceddeletingpolicies.WithLogging(c.inner.NamespacedDeletingPolicies(namespace), c.logger.WithValues("resource", "NamespacedDeletingPolicies").WithValues("namespace", namespace))
}
func (c *withLogging) ValidatingPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policies_kyverno_io_v1beta1.ValidatingPolicyInterface {
	return validatingpolicies.WithLogging(c.inner.ValidatingPolicies(), c.logger.WithValues("resource", "ValidatingPolicies"))
}
