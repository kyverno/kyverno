package gvr

import "github.com/kyverno/kyverno/test/e2e"

var (
	ClusterPolicy      = e2e.GetGVR("kyverno.io", "v1", "clusterpolicies")
	Namespace          = e2e.GetGVR("", "v1", "namespaces")
	ClusterRole        = e2e.GetGVR("rbac.authorization.k8s.io", "v1", "clusterroles")
	ClusterRoleBinding = e2e.GetGVR("rbac.authorization.k8s.io", "v1", "clusterrolebindings")
	Role               = e2e.GetGVR("rbac.authorization.k8s.io", "v1", "roles")
	RoleBinding        = e2e.GetGVR("rbac.authorization.k8s.io", "v1", "rolebindings")
	ConfigMap          = e2e.GetGVR("", "v1", "configmaps")
	NetworkPolicy      = e2e.GetGVR("networking.k8s.io", "v1", "networkpolicies")
)
