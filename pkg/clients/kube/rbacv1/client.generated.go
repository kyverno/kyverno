package client

import (
	clusterrolebindings "github.com/kyverno/kyverno/pkg/clients/kube/rbacv1/clusterrolebindings"
	clusterroles "github.com/kyverno/kyverno/pkg/clients/kube/rbacv1/clusterroles"
	rolebindings "github.com/kyverno/kyverno/pkg/clients/kube/rbacv1/rolebindings"
	roles "github.com/kyverno/kyverno/pkg/clients/kube/rbacv1/roles"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_client_go_kubernetes_typed_rbac_v1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_rbac_v1.RbacV1Interface, metrics metrics.MetricsConfigManager, clientType metrics.ClientType) k8s_io_client_go_kubernetes_typed_rbac_v1.RbacV1Interface {
	return &withMetrics{inner, metrics, clientType}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_rbac_v1.RbacV1Interface, client string) k8s_io_client_go_kubernetes_typed_rbac_v1.RbacV1Interface {
	return &withTracing{inner, client}
}

type withMetrics struct {
	inner      k8s_io_client_go_kubernetes_typed_rbac_v1.RbacV1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func (c *withMetrics) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withMetrics) ClusterRoleBindings() k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleBindingInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterRoleBinding", c.clientType)
	return clusterrolebindings.WithMetrics(c.inner.ClusterRoleBindings(), recorder)
}
func (c *withMetrics) ClusterRoles() k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterRole", c.clientType)
	return clusterroles.WithMetrics(c.inner.ClusterRoles(), recorder)
}
func (c *withMetrics) RoleBindings(namespace string) k8s_io_client_go_kubernetes_typed_rbac_v1.RoleBindingInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "RoleBinding", c.clientType)
	return rolebindings.WithMetrics(c.inner.RoleBindings(namespace), recorder)
}
func (c *withMetrics) Roles(namespace string) k8s_io_client_go_kubernetes_typed_rbac_v1.RoleInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Role", c.clientType)
	return roles.WithMetrics(c.inner.Roles(namespace), recorder)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_rbac_v1.RbacV1Interface
	client string
}

func (c *withTracing) RESTClient() rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) ClusterRoleBindings() k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleBindingInterface {
	return clusterrolebindings.WithTracing(c.inner.ClusterRoleBindings(), c.client, "ClusterRoleBinding")
}
func (c *withTracing) ClusterRoles() k8s_io_client_go_kubernetes_typed_rbac_v1.ClusterRoleInterface {
	return clusterroles.WithTracing(c.inner.ClusterRoles(), c.client, "ClusterRole")
}
func (c *withTracing) RoleBindings(namespace string) k8s_io_client_go_kubernetes_typed_rbac_v1.RoleBindingInterface {
	return rolebindings.WithTracing(c.inner.RoleBindings(namespace), c.client, "RoleBinding")
}
func (c *withTracing) Roles(namespace string) k8s_io_client_go_kubernetes_typed_rbac_v1.RoleInterface {
	return roles.WithTracing(c.inner.Roles(namespace), c.client, "Role")
}
