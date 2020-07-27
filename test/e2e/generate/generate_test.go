package generate

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"
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
	// e2eClient.CleanupNamespaces(nsGVR, nspace)
	e2eClient.DeleteClusteredResource(nsGVR, nspace)
	// Clear ClusterRole
	e2eClient.DeleteClusteredResource(crGVR, clusterRoleName)
	// Clear ClusterRoleBinding
	e2eClient.DeleteClusteredResource(crbGVR, clusterRoleBindingName)
	// Clear Role
	e2eClient.DeleteNamespacedResource(rGVR, nspace, roleName)
	// Clear RoleBinding
	e2eClient.DeleteNamespacedResource(rbGVR, nspace, roleBindingName)
}

func Test_ClusterRole_ClusterRoleBinding_Sets(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}
	// Generate E2E Client ==================
	e2eClient, err := NewE2EClient()
	Expect(err).To(BeNil())
	// ======================================

	// ====== Range Over ClusterRoleTests ==================
	for _, tests := range ClusterRoleTests {
		By(fmt.Sprintf("Test to generate ClusterRole and ClusterRoleBinding : %s", tests.TestName))
		By(fmt.Sprintf("synchronize = %v\t clone = %v", tests.Sync, tests.Clone))

		// ======= CleanUp Resources =====
		By(fmt.Sprintf("Cleaning Cluster Policies from Namespace : %s", clPolNS))
		e2eClient.CleanClusterPolicies(clPolGVR, clPolNS)
		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace : %s\n", tests.ResourceNamespace))
		e2eClient.DeleteClusteredResource(nsGVR, tests.ResourceNamespace)
		// If Clone is true Clear Source Resource and Recreate
		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Deleting Source ClusterRole and ClusterRoleBinding from Clone Namespace : %s\n", tests.CloneNamespace))
			e2eClient.DeleteNamespacedResource(crGVR, tests.CloneNamespace, tests.ClonerClusterRoleName)
			e2eClient.DeleteNamespacedResource(crbGVR, tests.CloneNamespace, tests.ClonerRoleBindingName)
		}

		// Wait to Delete Resources
		time.Sleep(5 * time.Second)
		// ====================================

		// ======== Create ClusterRole Policy =============
		By(fmt.Sprintf("Creating Generate Role Policy in %s", clPolNS))
		_, err = e2eClient.CreateNamespacedResourceYaml(clPolGVR, clPolNS, tests.Data)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

		// ======= Create Namespace ==================
		By(fmt.Sprintf("Creating Namespace which triggers generate %s \n", clPolNS))
		_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, namespaceYaml)
		Expect(err).NotTo(HaveOccurred())
		// ===========================================

		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Creating Cloner Resources in Namespace : %s", tests.CloneNamespace))
			e2eClient.CreateClusteredResourceYaml(crGVR, tests.CloneSourceRoleData)
			e2eClient.CreateClusteredResourceYaml(crbGVR, tests.CloneSourceRoleBindingData)
		}

		// Wait to Create Resources
		time.Sleep(5 * time.Second)

		// ======== Verify ClusterRole Creation =====
		rRes, err := e2eClient.GetClusteredResource(crGVR, tests.ClusterRoleName)
		Expect(err).NotTo(HaveOccurred())
		Expect(rRes.GetName()).To(Equal(tests.ClusterRoleName))
		// ============================================

		// ======= Verify ClusterRoleBinding Creation ========
		rbRes, err := e2eClient.GetClusteredResource(crbGVR, tests.ClusterRoleBindingName)
		Expect(err).NotTo(HaveOccurred())
		Expect(rbRes.GetName()).To(Equal(tests.ClusterRoleBindingName))
		// ============================================

		if tests.Sync {
			err = e2eClient.DeleteClusteredResource(rbGVR, tests.ClusterRoleBindingName)
			Expect(err).To(HaveOccurred())
			err = e2eClient.DeleteClusteredResource(rGVR, tests.ClusterRoleName)
			Expect(err).To(HaveOccurred())

			time.Sleep(5 * time.Second)
		}

		// ======= CleanUp Resources =====
		e2eClient.CleanClusterPolicies(clPolGVR, clPolNS)
		// Clear Namespace
		e2eClient.DeleteClusteredResource(nsGVR, tests.ResourceNamespace)
		// Wait to Delete Resources
		time.Sleep(5 * time.Second)
		// ====================================
		By(fmt.Sprintf("Test %s Completed \n\n\n", tests.TestName))
	}

}

func Test_Role_RoleBinding_Sets(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}
	// Generate E2E Client ==================
	e2eClient, err := NewE2EClient()
	Expect(err).To(BeNil())
	// ======================================

	// ====== Range Over RuleTest ==================
	for _, tests := range RoleTests {
		By(fmt.Sprintf("Test to generate Role and RoleBinding : %s", tests.TestName))
		By(fmt.Sprintf("synchronize = %v\t clone = %v", tests.Sync, tests.Clone))

		// ======= CleanUp Resources =====
		By(fmt.Sprintf("Cleaning Cluster Policies from Namespace : %s", clPolNS))
		e2eClient.CleanClusterPolicies(clPolGVR, clPolNS)
		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace : %s", tests.ResourceNamespace))
		e2eClient.DeleteClusteredResource(nsGVR, tests.ResourceNamespace)
		// If Clone is true Clear Source Resource and Recreate
		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Deleting Source Role and RoleBinding from Clone Namespace : %s", tests.CloneNamespace))
			e2eClient.DeleteNamespacedResource(rGVR, tests.CloneNamespace, tests.RoleName)
			e2eClient.DeleteNamespacedResource(rbGVR, tests.CloneNamespace, tests.RoleBindingName)
		}

		// Wait to Delete Resources
		time.Sleep(5 * time.Second)
		// ====================================

		// ======== Create Role Policy =============
		By(fmt.Sprintf("\nCreating Generate Role Policy in %s", clPolNS))
		_, err = e2eClient.CreateNamespacedResourceYaml(clPolGVR, clPolNS, tests.Data)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

		// ======= Create Namespace ==================
		By(fmt.Sprintf("Creating Namespace wich triggers generate %s", clPolNS))
		_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, namespaceYaml)
		Expect(err).NotTo(HaveOccurred())
		// ===========================================

		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Creating Cloner Resources in Namespace : %s", tests.CloneNamespace))
			e2eClient.CreateNamespacedResourceYaml(rGVR, tests.CloneNamespace, tests.CloneSourceRoleData)
			e2eClient.CreateNamespacedResourceYaml(rbGVR, tests.CloneNamespace, tests.CloneSourceRoleBindingData)
		}

		// Wait to Create Resources
		time.Sleep(2 * time.Second)

		// ======== Verify Role Creation =====
		rRes, err := e2eClient.GetNamespacedResource(rGVR, tests.ResourceNamespace, tests.RoleName)
		Expect(err).NotTo(HaveOccurred())
		Expect(rRes.GetName()).To(Equal(tests.RoleName))
		// ============================================

		// ======= Verify RoleBinding Creation ========
		rbRes, err := e2eClient.GetNamespacedResource(rbGVR, "default", tests.RoleBindingName)
		Expect(err).NotTo(HaveOccurred())
		Expect(rbRes.GetName()).To(Equal(tests.RoleBindingName))
		// ============================================

		if tests.Sync {
			err = e2eClient.DeleteNamespacedResource(rbGVR, tests.ResourceNamespace, tests.RoleBindingName)
			Expect(err).To(HaveOccurred())
			err = e2eClient.DeleteNamespacedResource(rGVR, tests.ResourceNamespace, tests.RoleName)
			Expect(err).To(HaveOccurred())

			time.Sleep(2 * time.Second)
		}

		// ======= CleanUp Resources =====
		e2eClient.CleanClusterPolicies(clPolGVR, clPolNS)
		// Clear Namespace
		e2eClient.DeleteClusteredResource(nsGVR, tests.ResourceNamespace)
		// Wait to Delete Resources
		time.Sleep(5 * time.Second)
		// ====================================
		By(fmt.Sprintf("Test %s Completed \n\n\n", tests.TestName))
	}

}
