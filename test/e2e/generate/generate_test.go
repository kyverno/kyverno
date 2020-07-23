package generate

import (
	. "github.com/onsi/gomega"
	"testing"
	"time"
)

var (
	// Cluster Polict GVR
	clPolGVR = GetGVR("kyverno.io", "v1", "clusterpolicies")
	// Namespace GVR
	nsGVR = GetGVR("", "v1", "namespaces")
	// ClusterRole GVR
	crGVR = GetGVR("rbac.authorization.k8s.io", "v1", "clusterroles")
	// ClusterRoleBinding GVR
	crbGVR = GetGVR("rbac.authorization.k8s.io", "v1", "clusterrolebindings")
	// Role GVR
	rGVR = GetGVR("rbac.authorization.k8s.io", "v1", "roles")
	// RoleBinding GVR
	rbGVR = GetGVR("rbac.authorization.k8s.io", "v1", "rolebindings")

	// ClusterPolicy Namespace
	clPolNS = ""
	// Namespace Name
	// Hardcoded in YAML Definition
	nspace = "test"
	// ClusterRole Name
	// Hardcoded in YAML Definition
	clusterRoleName = "ns-cluster-role"
	// ClusterRoleBindingName
	// Hardcoded in YAML Definition
	clusterRoleBindingName = "ns-cluster-role-binding"
	// Role Name
	// Hardcoded in YAML Definition
	roleName = "ns-role"
	// RoleBindingName
	// Hardcoded in YAML Definition
	roleBindingName = "ns-role-binding"
)

func CleanUpResources(e2eClient *E2EClient) {
	// Clear ClusterPolicies
	e2eClient.CleanClusterPolicies(clPolGVR, clPolNS)
	// Clear Namespace
	e2eClient.CleanupNamespaces(nsGVR, nspace)
	// Clear ClusterRole
	e2eClient.DeleteClusteredResource(crGVR, clusterRoleName)
	// Clear ClusterRoleBinding
	e2eClient.DeleteClusteredResource(crbGVR, clusterRoleBindingName)
	// Clear Role
	e2eClient.DeleteNamespacedResource(rGVR, nspace, roleName)
	// Clear RoleBinding
	e2eClient.DeleteNamespacedResource(rbGVR, nspace, roleBindingName)
}

func Test_ClusterRole_ClusterRoleBinding(t *testing.T) {
	RegisterTestingT(t)
	// Generate E2E Client
	e2eClient, err := NewE2EClient()
	Expect(err).To(BeNil())

	// ======= Cleanup Resources ==========================
	CleanUpResources(e2eClient)
	// Wait to Delete Resources
	time.Sleep(20 * time.Second)
	// ====================================================

	// ======== Create Cluster Policy =============
	_, err = e2eClient.CreateNamespacedResources(clPolGVR, clPolNS, genClusterRoleYaml)
	Expect(err).To(BeNil())
	// ============================================

	// ======= Create Namespace ==================
	_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, namespaceYaml)
	Expect(err).To(BeNil())
	// ===========================================

	// Wait to Create Resources
	time.Sleep(10 * time.Second)

	// ======== Verify Cluster Role Creation =====
	cRes, err := e2eClient.GetClusteredResource(crGVR, clusterRoleName)
	Expect(err).To(BeNil())
	Expect(cRes.GetName()).To(Equal(clusterRoleName))
	// ============================================

	// == Verify Cluster Role Binding Creation ====
	cbRes, err := e2eClient.GetClusteredResource(crbGVR, clusterRoleBindingName)
	Expect(err).To(BeNil())
	Expect(cbRes.GetName()).To(Equal(clusterRoleBindingName))
	// ============================================

	// ======= Cleanup Resources ==========================
	// CleanUpResources(e2eClient)
	// Wait to Delete Resources
	time.Sleep(20 * time.Second)
	// ====================================================

}

func Test_Role_RoleBinding(t *testing.T) {
	RegisterTestingT(t)
	// Generate E2E Client ==================
	e2eClient, err := NewE2EClient()
	Expect(err).To(BeNil())
	// ======================================

	// ======= Cleanup Resources ==========================
	CleanUpResources(e2eClient)
	// Wait to Delete Resources
	time.Sleep(10 * time.Second)
	// ====================================================

	// ======== Create Role Policy =============
	_, err = e2eClient.CreateNamespacedResources(clPolGVR, clPolNS, genRoleYaml)
	Expect(err).To(BeNil())
	// ============================================

	// ======= Create Namespace ==================
	_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, namespaceYaml)
	Expect(err).To(BeNil())
	// ===========================================

	// Wait to Create Resources
	time.Sleep(10 * time.Second)

	// ======== Verify Role Creation =====
	rRes, err := e2eClient.GetNamespacedResource(rGVR, nspace, roleName)
	Expect(err).To(BeNil())
	Expect(rRes.GetName()).To(Equal(roleName))
	// ============================================

	// ======= Verify RoleBinding Creation ========
	rbRes, err := e2eClient.GetNamespacedResource(rbGVR, nspace, roleBindingName)
	Expect(err).To(BeNil())
	Expect(rbRes.GetName()).To(Equal(roleBindingName))
	// ============================================
}
