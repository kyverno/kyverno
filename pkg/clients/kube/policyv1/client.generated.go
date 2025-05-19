package client

import (
	"github.com/go-logr/logr"
	evictions "github.com/kyverno/kyverno/pkg/clients/kube/policyv1/evictions"
	poddisruptionbudgets "github.com/kyverno/kyverno/pkg/clients/kube/policyv1/poddisruptionbudgets"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_policy_v1 "k8s.io/client-go/kubernetes/typed/policy/v1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface, client string) k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) Evictions(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Eviction", c.clientType)
	return evictions.WithMetrics(c.inner.Evictions(namespace), recorder)
}
func (c *withMetrics) PodDisruptionBudgets(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1.PodDisruptionBudgetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PodDisruptionBudget", c.clientType)
	return poddisruptionbudgets.WithMetrics(c.inner.PodDisruptionBudgets(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) Evictions(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface {
	return evictions.WithTracing(c.inner.Evictions(namespace), c.client, "Eviction")
}
func (c *withTracing) PodDisruptionBudgets(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1.PodDisruptionBudgetInterface {
	return poddisruptionbudgets.WithTracing(c.inner.PodDisruptionBudgets(namespace), c.client, "PodDisruptionBudget")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_policy_v1.PolicyV1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) Evictions(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1.EvictionInterface {
	return evictions.WithLogging(c.inner.Evictions(namespace), c.logger.WithValues("resource", "Evictions").WithValues("namespace", namespace))
}
func (c *withLogging) PodDisruptionBudgets(namespace string) k8s_io_client_go_kubernetes_typed_policy_v1.PodDisruptionBudgetInterface {
	return poddisruptionbudgets.WithLogging(c.inner.PodDisruptionBudgets(namespace), c.logger.WithValues("resource", "PodDisruptionBudgets").WithValues("namespace", namespace))
}
