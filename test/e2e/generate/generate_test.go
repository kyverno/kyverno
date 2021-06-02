package generate

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kyverno/kyverno/test/e2e"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	// Cluster Polict GVR
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
	// ConfigMap GVR
	cmGVR = e2e.GetGVR("", "v1", "configmaps")

	// ClusterPolicy Namespace
	clPolNS = ""
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

func Test_Source_Resource_Update_Replication(t *testing.T) {
	RegisterTestingT(t)
	// if os.Getenv("E2E") == "" {
	// 	t.Skip("Skipping E2E Test")
	// }
	// Generate E2E Client ==================
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())
	// ======================================

	// ====== Range Over RuleTest ==================
	for _, tests := range SourceResourceUpdateReplicationTests {
		By(fmt.Sprintf("Test to check replication of clone source resource: %s", tests.TestName))
		By(fmt.Sprintf("synchronize = %v\t clone = %v", tests.Sync, tests.Clone))

		// ======= CleanUp Resources =====
		By("Cleaning Cluster Policies")
		e2eClient.CleanClusterPolicies(clPolGVR)
		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace : %s", tests.ResourceNamespace))
		e2eClient.DeleteClusteredResource(nsGVR, tests.ResourceNamespace)
		// If Clone is true Clear Source Resource and Recreate
		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Deleting Source Resource from Clone Namespace : %s", tests.CloneNamespace))
			// Delete ConfigMap to be cloned
			e2eClient.DeleteNamespacedResource(cmGVR, tests.CloneNamespace, tests.ConfigMapName)
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

		// === If Clone is true Create Source Resources ==
		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Creating Cloner Resources in Namespace : %s", tests.CloneNamespace))
			_, err := e2eClient.CreateNamespacedResourceYaml(cmGVR, tests.CloneNamespace, tests.CloneSourceConfigMapData)
			Expect(err).NotTo(HaveOccurred())
		}
		// ================================================

		// ======== Create Generate Policy =============
		By(fmt.Sprintf("\nCreating Generate Policy in %s", clPolNS))
		_, err = e2eClient.CreateNamespacedResourceYaml(clPolGVR, clPolNS, tests.Data)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

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

		// ======== Verify Configmap Creation =====
		By(fmt.Sprintf("Verifying Configmap in the Namespace : %s", tests.ResourceNamespace))
		// Wait Till Creation of Configmap
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetNamespacedResource(cmGVR, tests.ResourceNamespace, tests.ConfigMapName)
			if err != nil {
				return err
			}
			return nil
		})

		// test: when a source clone resource is updated, the same changes should be replicated in the generated resource
		// ======= Update Configmap in default Namespace ========
		By(fmt.Sprintf("Updating Source Resource(Configmap) in Clone Namespace : %s", tests.CloneNamespace))

		// Get the configmap from default namespace
		sourceRes, err := e2eClient.GetNamespacedResource(cmGVR, tests.CloneNamespace, tests.ConfigMapName)
		Expect(err).NotTo(HaveOccurred())
		Expect(sourceRes.GetName()).To(Equal(tests.ConfigMapName))

		element, _, err := unstructured.NestedMap(sourceRes.UnstructuredContent(), "data")
		Expect(err).NotTo(HaveOccurred())
		element["initial_lives"] = "5"

		unstructured.SetNestedMap(sourceRes.UnstructuredContent(), element, "data")
		_, err = e2eClient.UpdateNamespacedResource(cmGVR, sourceRes, tests.CloneNamespace)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

		// ======= Verifying Configmap Data Replication in Namespace ========
		By(fmt.Sprintf("Verifying Configmap Data Replication in the Namespace : %s", tests.ResourceNamespace))
		e2e.GetWithRetry(time.Duration(2), 15, func() error {
			// get updated configmap in test namespace
			updatedGenRes, err := e2eClient.GetNamespacedResource(cmGVR, tests.ResourceNamespace, tests.ConfigMapName)
			if err != nil {
				return err
			}

			// compare updated configmapdata
			element, _, err := unstructured.NestedMap(updatedGenRes.UnstructuredContent(), "data")
			if err != nil {
				return err
			}
			if element["initial_lives"] != "5" {
				return errors.New("not updated")
			}

			return nil
		})

		updatedGenRes, err := e2eClient.GetNamespacedResource(cmGVR, tests.ResourceNamespace, tests.ConfigMapName)
		Expect(err).NotTo(HaveOccurred())
		element, _, err = unstructured.NestedMap(updatedGenRes.UnstructuredContent(), "data")
		Expect(err).NotTo(HaveOccurred())
		Expect(element["initial_lives"]).To(Equal("5"))
		// ============================================

		// test: when a generated resource is edited with some conflicting changes (with respect to the
		// clone source resource or generate data), kyverno will regenerate the resource
		// ======= Update Configmap in test Namespace ========
		By(fmt.Sprintf("Updating Generated ConfigMap in Resource Namespace : %s", tests.ResourceNamespace))

		// Get the configmap from test namespace
		genRes, err := e2eClient.GetNamespacedResource(cmGVR, tests.ResourceNamespace, tests.ConfigMapName)
		Expect(err).NotTo(HaveOccurred())
		Expect(genRes.GetName()).To(Equal(tests.ConfigMapName))

		element, _, err = unstructured.NestedMap(genRes.UnstructuredContent(), "data")
		Expect(err).NotTo(HaveOccurred())
		element["initial_lives"] = "15"

		unstructured.SetNestedMap(genRes.UnstructuredContent(), element, "data")
		_, err = e2eClient.UpdateNamespacedResource(cmGVR, genRes, tests.ResourceNamespace)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

		// ======= Verifying Configmap Data in Namespace ========
		By(fmt.Sprintf("Verifying Configmap Data in the Namespace : %s", tests.ResourceNamespace))
		e2e.GetWithRetry(time.Duration(2), 15, func() error {
			// get updated configmap in test namespace
			updatedGenRes, err := e2eClient.GetNamespacedResource(cmGVR, tests.ResourceNamespace, tests.ConfigMapName)
			if err != nil {
				return err
			}

			// compare updated configmapdata
			element, _, err := unstructured.NestedMap(updatedGenRes.UnstructuredContent(), "data")
			if err != nil {
				return err
			}
			if element["initial_lives"] != "5" {
				return errors.New("not updated")
			}

			return nil
		})

		updatedGenRes, err = e2eClient.GetNamespacedResource(cmGVR, tests.ResourceNamespace, tests.ConfigMapName)
		Expect(err).NotTo(HaveOccurred())
		element, _, err = unstructured.NestedMap(updatedGenRes.UnstructuredContent(), "data")
		Expect(err).NotTo(HaveOccurred())
		Expect(element["initial_lives"]).To(Equal("5"))
		// ============================================

		// ======= CleanUp Resources =====
		e2eClient.CleanClusterPolicies(clPolGVR)

		// === If Clone is true Delete Source Resources ==
		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Deleting Cloner Resources in Namespace : %s", tests.CloneNamespace))
			e2eClient.DeleteNamespacedResource(cmGVR, tests.CloneNamespace, tests.ConfigMapName)
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
