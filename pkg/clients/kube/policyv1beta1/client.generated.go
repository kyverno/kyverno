package client

import (
	"github.com/go-logr/logr"
	evictions "github.com/kyverno/kyverno/pkg/clients/kube/policyv1beta1/evictions"
	poddisruptionbudgets "github.com/kyverno/kyverno/pkg/clients/kube/policyv1beta1/poddisruptionbudgets"
	podsecuritypolicies "github.com/kyverno/kyverno/pkg/clients/kube/policyv1beta1/podsecuritypolicies"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_policy_v1beta1 "k8s.io/client-go/kubernetes/typed/policy/v1beta1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface, client string) k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) Evictions(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Eviction", c.clientType)
	return evictions.WithMetrics(c.inner.Evictions(namespace), recorder)
}
func (c *withMetrics) PodDisruptionBudgets(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodDisruptionBudgetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PodDisruptionBudget", c.clientType)
	return poddisruptionbudgets.WithMetrics(c.inner.PodDisruptionBudgets(namespace), recorder)
}
func (c *withMetrics) PodSecurityPolicies() k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodSecurityPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PodSecurityPolicy", c.clientType)
	return podsecuritypolicies.WithMetrics(c.inner.PodSecurityPolicies(), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) Evictions(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface {
	return evictions.WithTracing(c.inner.Evictions(namespace), c.client, "Eviction")
}
func (c *withTracing) PodDisruptionBudgets(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodDisruptionBudgetInterface {
	return poddisruptionbudgets.WithTracing(c.inner.PodDisruptionBudgets(namespace), c.client, "PodDisruptionBudget")
}
func (c *withTracing) PodSecurityPolicies() k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodSecurityPolicyInterface {
	return podsecuritypolicies.WithTracing(c.inner.PodSecurityPolicies(), c.client, "PodSecurityPolicy")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_policy_v1beta1.PolicyV1beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) Evictions(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1beta1.EvictionInterface {
	return evictions.WithLogging(c.inner.Evictions(namespace), c.logger.WithValues("resource", "Evictions").WithValues("namespace", namespace))
}
func (c *withLogging) PodDisruptionBudgets(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodDisruptionBudgetInterface {
	return poddisruptionbudgets.WithLogging(c.inner.PodDisruptionBudgets(namespace), c.logger.WithValues("resource", "PodDisruptionBudgets").WithValues("namespace", namespace))
}
func (c *withLogging) PodSecurityPolicies() k8s_io_client_go_kubernetes_typed_policy_v1beta1.PodSecurityPolicyInterface {
	return podsecuritypolicies.WithLogging(c.inner.PodSecurityPolicies(), c.logger.WithValues("resource", "PodSecurityPolicies"))
}
