package client

import (
	controllerrevisions "github.com/kyverno/kyverno/pkg/clients/kube/appsv1/controllerrevisions"
	daemonsets "github.com/kyverno/kyverno/pkg/clients/kube/appsv1/daemonsets"
	deployments "github.com/kyverno/kyverno/pkg/clients/kube/appsv1/deployments"
	replicasets "github.com/kyverno/kyverno/pkg/clients/kube/appsv1/replicasets"
	statefulsets "github.com/kyverno/kyverno/pkg/clients/kube/appsv1/statefulsets"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_apps_v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_apps_v1.AppsV1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_apps_v1.AppsV1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_apps_v1.AppsV1Interface, client string) k8s_io_client_go_kubernetes_typed_apps_v1.AppsV1Interface {
	return &withTracing{inner, client}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_apps_v1.AppsV1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) ControllerRevisions(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1.ControllerRevisionInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "ControllerRevision", c.clientType)
	return controllerrevisions.WithMetrics(c.inner.ControllerRevisions(namespace), recorder)
}
func (c *withMetrics) DaemonSets(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1.DaemonSetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "DaemonSet", c.clientType)
	return daemonsets.WithMetrics(c.inner.DaemonSets(namespace), recorder)
}
func (c *withMetrics) Deployments(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1.DeploymentInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Deployment", c.clientType)
	return deployments.WithMetrics(c.inner.Deployments(namespace), recorder)
}
func (c *withMetrics) ReplicaSets(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "ReplicaSet", c.clientType)
	return replicasets.WithMetrics(c.inner.ReplicaSets(namespace), recorder)
}
func (c *withMetrics) StatefulSets(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1.StatefulSetInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "StatefulSet", c.clientType)
	return statefulsets.WithMetrics(c.inner.StatefulSets(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_apps_v1.AppsV1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) ControllerRevisions(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1.ControllerRevisionInterface {
	return controllerrevisions.WithTracing(c.inner.ControllerRevisions(namespace), c.client, "ControllerRevision")
}
func (c *withTracing) DaemonSets(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1.DaemonSetInterface {
	return daemonsets.WithTracing(c.inner.DaemonSets(namespace), c.client, "DaemonSet")
}
func (c *withTracing) Deployments(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1.DeploymentInterface {
	return deployments.WithTracing(c.inner.Deployments(namespace), c.client, "Deployment")
}
func (c *withTracing) ReplicaSets(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface {
	return replicasets.WithTracing(c.inner.ReplicaSets(namespace), c.client, "ReplicaSet")
}
func (c *withTracing) StatefulSets(namespace string) k8s_io_client_go_kubernetes_typed_apps_v1.StatefulSetInterface {
	return statefulsets.WithTracing(c.inner.StatefulSets(namespace), c.client, "StatefulSet")
}
