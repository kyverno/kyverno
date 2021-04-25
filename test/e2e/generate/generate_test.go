package generate

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kyverno/kyverno/test/e2e"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	// Cluster Policy GVR
	clPolGVR = e2e.GetGVR("kyverno.io", "v1", "clusterpolicies")
	// Namespace GVR
	nsGVR = e2e.GetGVR("", "v1", "namespaces")
	// ClusterRole GVR
	crGVR = e2e.GetGVR("rbac.authorization.k8s.io", "v1", "clusterroles")
	// ClusterRoleBinding GVR
	crbGVR = e2e.GetGVR("rbac.authorization.k8s.io", "v1", "clusterrolebindings")
	// Role GVR
	rGVR = e2e.GetGVR("rbac.authorization.k8s.io", "v1", "roles")
	// RoleBinding GVR
	rbGVR = e2e.GetGVR("rbac.authorization.k8s.io", "v1", "rolebindings")
	// NetworkPolicy GVR
	npGVR = e2e.GetGVR("networking.k8s.io", "v1", "networkpolicies")

	// ClusterPolicy Namespace
	clPolNS = ""
	// NetworkPolicy Namespace
	npPolNS = ""
	// Namespace Name
	// Hardcoded in YAML Definition
	nspace = "test"
)

func Test_ClusterRole_ClusterRoleBinding_Sets(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}
	// Generate E2E Client ==================
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())
	// ======================================

	// ====== Range Over ClusterRoleTests ==================
	for _, tests := range ClusterRoleTests {
		By(fmt.Sprintf("Test to generate ClusterRole and ClusterRoleBinding : %s", tests.TestName))
		By(fmt.Sprintf("synchronize = %v\t clone = %v", tests.Sync, tests.Clone))

		// ======= CleanUp Resources =====
		By(fmt.Sprintf("Cleaning Cluster Policies"))
		e2eClient.CleanClusterPolicies(clPolGVR)

		// If Clone is true Clear Source Resource and Recreate
		if tests.Clone {
			By("Clone = true, Deleting Source ClusterRole and ClusterRoleBinding")
			// Delete ClusterRole to be cloned
			e2eClient.DeleteClusteredResource(crGVR, tests.ClonerClusterRoleName)
			// Delete ClusterRoleBinding to be cloned
			e2eClient.DeleteClusteredResource(crbGVR, tests.ClonerClusterRoleBindingName)
		}
		// ====================================

		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace : %s\n", tests.ResourceNamespace))
		e2eClient.DeleteClusteredResource(nsGVR, tests.ResourceNamespace)

		// Wait Till Deletion of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, tests.ResourceNamespace)
			if err != nil {
				return nil
			}
			return errors.New("Deleting Namespace")
		})

		// =====================================================

		// ======== Create ClusterRole Policy =============
		By(fmt.Sprintf("Creating Generate Role Policy in %s", clPolNS))
		_, err = e2eClient.CreateNamespacedResourceYaml(clPolGVR, clPolNS, tests.Data)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

		// == If Clone is true Create Source Resources ======
		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Creating Cloner Resources in Namespace : %s", tests.CloneNamespace))
			// Create ClusterRole to be cloned
			_, err := e2eClient.CreateClusteredResourceYaml(crGVR, tests.CloneSourceClusterRoleData)
			Expect(err).NotTo(HaveOccurred())
			// Create ClusterRoleBinding to be cloned
			_, err = e2eClient.CreateClusteredResourceYaml(crbGVR, tests.CloneSourceClusterRoleBindingData)
			Expect(err).NotTo(HaveOccurred())
		}

		// =================================================

		// ======= Create Namespace ==================
		By(fmt.Sprintf("Creating Namespace which triggers generate %s \n", clPolNS))
		_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, namespaceYaml)
		Expect(err).NotTo(HaveOccurred())

		// Wait Till Creation of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, tests.ResourceNamespace)
			if err != nil {
				return err
			}
			return nil
		})
		// ===========================================

		// ======== Verify ClusterRole Creation =====
		By("Verifying ClusterRole")
		// Wait Till Creation of ClusterRole
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(crGVR, tests.ClusterRoleName)
			if err != nil {
				return err
			}
			return nil
		})
		rRes, err := e2eClient.GetClusteredResource(crGVR, tests.ClusterRoleName)
		Expect(err).NotTo(HaveOccurred())
		Expect(rRes.GetName()).To(Equal(tests.ClusterRoleName))
		// ============================================

		// ======= Verify ClusterRoleBinding Creation ========
		By("Verifying ClusterRoleBinding")
		rbRes, err := e2eClient.GetClusteredResource(crbGVR, tests.ClusterRoleBindingName)
		Expect(err).NotTo(HaveOccurred())
		Expect(rbRes.GetName()).To(Equal(tests.ClusterRoleBindingName))
		// ============================================

		// ======= CleanUp Resources =====
		e2eClient.CleanClusterPolicies(clPolGVR)
		// Clear Namespace
		e2eClient.DeleteClusteredResource(nsGVR, tests.ResourceNamespace)
		// Wait Till Deletion of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, tests.ResourceNamespace)
			if err != nil {
				return nil
			}
			return errors.New("Deleting Namespace")
		})
		By(fmt.Sprintf("Test %s Completed \n\n\n", tests.TestName))
	}

}

func Test_Role_RoleBinding_Sets(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}
	// Generate E2E Client ==================
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())
	// ======================================

	// ====== Range Over RuleTest ==================
	for _, tests := range RoleTests {
		By(fmt.Sprintf("Test to generate Role and RoleBinding : %s", tests.TestName))
		By(fmt.Sprintf("synchronize = %v\t clone = %v", tests.Sync, tests.Clone))

		// ======= CleanUp Resources =====
		By(fmt.Sprintf("Cleaning Cluster Policies"))
		e2eClient.CleanClusterPolicies(clPolGVR)
		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace : %s", tests.ResourceNamespace))
		e2eClient.DeleteClusteredResource(nsGVR, tests.ResourceNamespace)
		// If Clone is true Clear Source Resource and Recreate
		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Deleting Source Role and RoleBinding from Clone Namespace : %s", tests.CloneNamespace))
			// Delete Role to be cloned
			e2eClient.DeleteNamespacedResource(rGVR, tests.CloneNamespace, tests.RoleName)
			// Delete RoleBinding to be cloned
			e2eClient.DeleteNamespacedResource(rbGVR, tests.CloneNamespace, tests.RoleBindingName)
		}

		// Wait Till Deletion of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, tests.ResourceNamespace)
			if err != nil {
				return nil
			}
			return errors.New("Deleting Namespace")
		})
		// ====================================

		// ======== Create Role Policy =============
		By(fmt.Sprintf("\nCreating Generate Role Policy in %s", clPolNS))
		_, err = e2eClient.CreateNamespacedResourceYaml(clPolGVR, clPolNS, tests.Data)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

		// === If Clone is true Create Source Resources ==
		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Creating Cloner Resources in Namespace : %s", tests.CloneNamespace))
			_, err := e2eClient.CreateNamespacedResourceYaml(rGVR, tests.CloneNamespace, tests.CloneSourceRoleData)
			Expect(err).NotTo(HaveOccurred())
			_, err = e2eClient.CreateNamespacedResourceYaml(rbGVR, tests.CloneNamespace, tests.CloneSourceRoleBindingData)
			Expect(err).NotTo(HaveOccurred())
		}
		// ================================================

		// ======= Create Namespace ==================
		By(fmt.Sprintf("Creating Namespace which triggers generate %s", clPolNS))
		_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, namespaceYaml)
		Expect(err).NotTo(HaveOccurred())

		// Wait Till Creation of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, tests.ResourceNamespace)
			if err != nil {
				return err
			}
			return nil
		})
		// ===========================================

		// ======== Verify Role Creation =====
		By(fmt.Sprintf("Verifying Role in the Namespace : %s", tests.ResourceNamespace))
		// Wait Till Creation of Role
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetNamespacedResource(rGVR, tests.ResourceNamespace, tests.RoleName)
			if err != nil {
				return err
			}
			return nil
		})
		rRes, err := e2eClient.GetNamespacedResource(rGVR, tests.ResourceNamespace, tests.RoleName)
		Expect(err).NotTo(HaveOccurred())
		Expect(rRes.GetName()).To(Equal(tests.RoleName))
		// ============================================

		// ======= Verify RoleBinding Creation ========
		By(fmt.Sprintf("Verifying RoleBinding in the Namespace : %s", tests.ResourceNamespace))
		rbRes, err := e2eClient.GetNamespacedResource(rbGVR, tests.ResourceNamespace, tests.RoleBindingName)
		Expect(err).NotTo(HaveOccurred())
		Expect(rbRes.GetName()).To(Equal(tests.RoleBindingName))
		// ============================================

		// ======= CleanUp Resources =====
		e2eClient.CleanClusterPolicies(clPolGVR)

		// === If Clone is true Delete Source Resources ==
		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Deleting Cloner Resources in Namespace : %s", tests.CloneNamespace))
			e2eClient.DeleteNamespacedResource(rGVR, tests.CloneNamespace, tests.RoleName)
			e2eClient.DeleteNamespacedResource(rbGVR, tests.CloneNamespace, tests.RoleBindingName)
		}
		// ================================================

		// Clear Namespace
		e2eClient.DeleteClusteredResource(nsGVR, tests.ResourceNamespace)
		// Wait Till Deletion of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, tests.ResourceNamespace)
			if err != nil {
				return nil
			}
			return errors.New("Deleting Namespace")
		})
		// ====================================

		By(fmt.Sprintf("Test %s Completed \n\n\n", tests.TestName))
	}

}

func Test_Generate_NetworkPolicy(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}
	// Generate E2E Client ==================
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())
	// ======================================

	// ====== Range Over RuleTest ==================
	for _, test := range NetworkPolicyGenerateTests {
		By(fmt.Sprintf("Test to generate NetworkPolicy : %s", test.TestName))
		By(fmt.Sprintf("synchronize = %v\t clone = %v", test.Sync, test.Clone))

		// ======= CleanUp Resources =====
		By(fmt.Sprintf("Cleaning Cluster Policies"))
		e2eClient.CleanClusterPolicies(clPolGVR)
		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace : %s", test.ResourceNamespace))
		e2eClient.DeleteClusteredResource(nsGVR, test.ResourceNamespace)
		// If Clone is true Clear Source Resource and Recreate
		if test.Clone {
			By(fmt.Sprintf("Clone = true, Deleting NetworkPolicy from Clone Namespace : %s", test.CloneNamespace))
			// Delete NetworkPolicy to be cloned
			e2eClient.DeleteNamespacedResource(npGVR, test.CloneNamespace, test.NetworkPolicyName)

		}

		// Wait Till Deletion of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, test.ResourceNamespace)
			if err != nil {
				return nil
			}
			return errors.New("deleting Namespace")
		})
		// ====================================
		// ======== Create Generate NetworkPolicy Policy =============
		By(fmt.Sprintf("\nCreating Generate NetworkPolicy Policy"))
		_, err = e2eClient.CreateNamespacedResourceYaml(clPolGVR, npPolNS, test.Data)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

		// === If Clone is true Create Source Resources ==
		//if test.Clone {
		//	By(fmt.Sprintf("Clone = true, Creating Cloner Resources in Namespace : %s", test.CloneNamespace))
		//	_, err := e2eClient.CreateNamespacedResourceYaml(rGVR, test.CloneNamespace, test.CloneSourceRoleData)
		//	Expect(err).NotTo(HaveOccurred())
		//	_, err = e2eClient.CreateNamespacedResourceYaml(rbGVR, test.CloneNamespace, test.CloneSourceRoleBindingData)
		//	Expect(err).NotTo(HaveOccurred())
		//}
		// ================================================

		// ======= Create Namespace ==================
		By(fmt.Sprintf("Creating Namespace which triggers generate %s", npPolNS))
		_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, namespaceWithLabelYaml)
		Expect(err).NotTo(HaveOccurred())

		// Wait Till Creation of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, test.ResourceNamespace)
			if err != nil {
				return err
			}
			return nil
		})
		// ===========================================

		// ======== NetworkPolicy Creation =====
		By(fmt.Sprintf("Verifying NetworkPolicy in the Namespace : %s", test.ResourceNamespace))
		// Wait Till Creation of NetworkPolicy
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
			if err != nil {
				return err
			}
			return nil
		})
		npRes, err := e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)

		// TODO: remove this later
		//fmt.Println("=======open: listing all network policies =======")
		//fmt.Println(npGVR)
		//fmt.Println(test.ResourceNamespace)
		//lnr, err := e2eClient.ListNamespacedResources(npGVR, test.ResourceNamespace)
		//if err != nil {
		//	fmt.Println(err)
		//}
		//fmt.Println(lnr)
		//fmt.Println("=======close: listing all network policies =======")

		Expect(err).NotTo(HaveOccurred())
		Expect(npRes.GetName()).To(Equal(test.NetworkPolicyName))
		// ============================================

		// ======= CleanUp Resources =====
		e2eClient.CleanClusterPolicies(clPolGVR)

		// === If Clone is true Delete Source Resources ==
		// TODO: check this later
		//if test.Clone {
		//	By(fmt.Sprintf("Clone = true, Deleting Cloner Resources in Namespace : %s", test.CloneNamespace))
		//	e2eClient.DeleteNamespacedResource(rGVR, test.CloneNamespace, test.RoleName)
		//	e2eClient.DeleteNamespacedResource(rbGVR, test.CloneNamespace, test.RoleBindingName)
		//}
		// ================================================

		// Clear Namespace
		e2eClient.DeleteClusteredResource(nsGVR, test.ResourceNamespace)
		// Wait Till Deletion of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, test.ResourceNamespace)
			if err != nil {
				return nil
			}
			return errors.New("deleting Namespace")
		})
		// ====================================

		By(fmt.Sprintf("Test %s Completed \n\n\n", test.TestName))
	}

}

func Test_Generate_Namespace_Without_Label(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}
	// Generate E2E Client ==================
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())
	// ======================================

	// ====== Range Over RuleTest ==================
	for _, test := range GenerateNetworkPolicyOnNamespaceWithoutLabelTests {
		By(fmt.Sprintf("Test to generate NetworkPolicy : %s", test.TestName))
		By(fmt.Sprintf("synchronize = %v\t clone = %v", test.Sync, test.Clone))

		// ======= CleanUp Resources =====
		By(fmt.Sprintf("Cleaning Cluster Policies"))
		e2eClient.CleanClusterPolicies(clPolGVR)
		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace : %s", test.ResourceNamespace))
		e2eClient.DeleteClusteredResource(nsGVR, test.ResourceNamespace)
		// // If Clone is true Clear Source Resource and Recreate
		// if test.Clone {
		// 	By(fmt.Sprintf("Clone = true, Deleting NetworkPolicy from Clone Namespace : %s", test.CloneNamespace))
		// 	// Delete NetworkPolicy to be cloned
		// 	e2eClient.DeleteNamespacedResource(npGVR, test.CloneNamespace, test.NetworkPolicyName)

		// }

		// Wait Till Deletion of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, test.ResourceNamespace)
			if err != nil {
				return nil
			}
			return errors.New("deleting Namespace")
		})
		// ====================================
		// ======== Create Generate NetworkPolicy Policy =============
		By(fmt.Sprintf("\nCreating Generate NetworkPolicy Policy"))
		_, err = e2eClient.CreateNamespacedResourceYaml(clPolGVR, npPolNS, test.Data)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

		// === If Clone is true Create Source Resources ==
		//if test.Clone {
		//	By(fmt.Sprintf("Clone = true, Creating Cloner Resources in Namespace : %s", test.CloneNamespace))
		//	_, err := e2eClient.CreateNamespacedResourceYaml(rGVR, test.CloneNamespace, test.CloneSourceRoleData)
		//	Expect(err).NotTo(HaveOccurred())
		//	_, err = e2eClient.CreateNamespacedResourceYaml(rbGVR, test.CloneNamespace, test.CloneSourceRoleBindingData)
		//	Expect(err).NotTo(HaveOccurred())
		//}
		// ================================================

		// ======= Create Namespace ==================
		By(fmt.Sprintf("Creating Namespace which triggers generate %s", npPolNS))
		_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, namespaceYaml)
		Expect(err).NotTo(HaveOccurred())

		// Wait Till Creation of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, test.ResourceNamespace)
			if err != nil {
				return err
			}
			return nil
		})
		// ===========================================

		// ======== NetworkPolicy Creation =====
		By(fmt.Sprintf("Verifying NetworkPolicy in the Namespace : %s", test.ResourceNamespace))
		// Wait Till Creation of NetworkPolicy
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
			if err != nil {
				return err
			}
			return nil
		})

		_, err := e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
		Expect(err).To(HaveOccurred())
		// ============================================

		// ======= CleanUp Resources =====
		e2eClient.CleanClusterPolicies(clPolGVR)

		// === If Clone is true Delete Source Resources ==
		// TODO: check this later
		//if test.Clone {
		//	By(fmt.Sprintf("Clone = true, Deleting Cloner Resources in Namespace : %s", test.CloneNamespace))
		//	e2eClient.DeleteNamespacedResource(rGVR, test.CloneNamespace, test.RoleName)
		//	e2eClient.DeleteNamespacedResource(rbGVR, test.CloneNamespace, test.RoleBindingName)
		//}
		// ================================================

		// Clear Namespace
		e2eClient.DeleteClusteredResource(nsGVR, test.ResourceNamespace)
		// Wait Till Deletion of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, test.ResourceNamespace)
			if err != nil {
				return nil
			}
			return errors.New("deleting Namespace")
		})
		// ====================================

		By(fmt.Sprintf("Test %s Completed \n\n\n", test.TestName))
	}

}
