package client

import (
	"github.com/go-logr/logr"
	daemonsets "github.com/kyverno/kyverno/pkg/clients/kube/extensionsv1beta1/daemonsets"
	deployments "github.com/kyverno/kyverno/pkg/clients/kube/extensionsv1beta1/deployments"
	ingresses "github.com/kyverno/kyverno/pkg/clients/kube/extensionsv1beta1/ingresses"
	networkpolicies "github.com/kyverno/kyverno/pkg/clients/kube/extensionsv1beta1/networkpolicies"
	podsecuritypolicies "github.com/kyverno/kyverno/pkg/clients/kube/extensionsv1beta1/podsecuritypolicies"
	replicasets "github.com/kyverno/kyverno/pkg/clients/kube/extensionsv1beta1/replicasets"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_extensions_v1beta1 "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface, client string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface {
	return &withTracing{inner, client}
}

func WithLogging(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface {
	return &withLogging{inner, logger}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) DaemonSets(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DaemonSetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "DaemonSet", c.clientType)
	return daemonsets.WithMetrics(c.inner.DaemonSets(namespace), recorder)
}
func (c *withMetrics) Deployments(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Deployment", c.clientType)
	return deployments.WithMetrics(c.inner.Deployments(namespace), recorder)
}
func (c *withMetrics) Ingresses(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.IngressInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Ingress", c.clientType)
	return ingresses.WithMetrics(c.inner.Ingresses(namespace), recorder)
}
func (c *withMetrics) NetworkPolicies(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.NetworkPolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "NetworkPolicy", c.clientType)
	return networkpolicies.WithMetrics(c.inner.NetworkPolicies(namespace), recorder)
}
func (c *withMetrics) PodSecurityPolicies() k8s_io_client_go_kubernetes_typed_extensions_v1beta1.PodSecurityPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "PodSecurityPolicy", c.clientType)
	return podsecuritypolicies.WithMetrics(c.inner.PodSecurityPolicies(), recorder)
}
func (c *withMetrics) ReplicaSets(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ReplicaSetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "ReplicaSet", c.clientType)
	return replicasets.WithMetrics(c.inner.ReplicaSets(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) DaemonSets(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DaemonSetInterface {
	return daemonsets.WithTracing(c.inner.DaemonSets(namespace), c.client, "DaemonSet")
}
func (c *withTracing) Deployments(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface {
	return deployments.WithTracing(c.inner.Deployments(namespace), c.client, "Deployment")
}
func (c *withTracing) Ingresses(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.IngressInterface {
	return ingresses.WithTracing(c.inner.Ingresses(namespace), c.client, "Ingress")
}
func (c *withTracing) NetworkPolicies(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.NetworkPolicyInterface {
	return networkpolicies.WithTracing(c.inner.NetworkPolicies(namespace), c.client, "NetworkPolicy")
}
func (c *withTracing) PodSecurityPolicies() k8s_io_client_go_kubernetes_typed_extensions_v1beta1.PodSecurityPolicyInterface {
	return podsecuritypolicies.WithTracing(c.inner.PodSecurityPolicies(), c.client, "PodSecurityPolicy")
}
func (c *withTracing) ReplicaSets(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ReplicaSetInterface {
	return replicasets.WithTracing(c.inner.ReplicaSets(namespace), c.client, "ReplicaSet")
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ExtensionsV1beta1Interface
	logger logr.Logger
}

func (c *withLogging) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withLogging) DaemonSets(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DaemonSetInterface {
	return daemonsets.WithLogging(c.inner.DaemonSets(namespace), c.logger.WithValues("resource", "DaemonSets").WithValues("namespace", namespace))
}
func (c *withLogging) Deployments(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface {
	return deployments.WithLogging(c.inner.Deployments(namespace), c.logger.WithValues("resource", "Deployments").WithValues("namespace", namespace))
}
func (c *withLogging) Ingresses(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.IngressInterface {
	return ingresses.WithLogging(c.inner.Ingresses(namespace), c.logger.WithValues("resource", "Ingresses").WithValues("namespace", namespace))
}
func (c *withLogging) NetworkPolicies(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.NetworkPolicyInterface {
	return networkpolicies.WithLogging(c.inner.NetworkPolicies(namespace), c.logger.WithValues("resource", "NetworkPolicies").WithValues("namespace", namespace))
}
func (c *withLogging) PodSecurityPolicies() k8s_io_client_go_kubernetes_typed_extensions_v1beta1.PodSecurityPolicyInterface {
	return podsecuritypolicies.WithLogging(c.inner.PodSecurityPolicies(), c.logger.WithValues("resource", "PodSecurityPolicies"))
}
func (c *withLogging) ReplicaSets(namespace string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.ReplicaSetInterface {
	return replicasets.WithLogging(c.inner.ReplicaSets(namespace), c.logger.WithValues("resource", "ReplicaSets").WithValues("namespace", namespace))
}
