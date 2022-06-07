package generate

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kyverno/kyverno/test/e2e"
	commonE2E "github.com/kyverno/kyverno/test/e2e/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func runTestCases(t *testing.T, testCases ...testCase) {
	setup(t)

	for _, test := range testCases {
		t.Run(test.TestName, func(t *testing.T) {
			e2eClient := createClient()

			t.Cleanup(func() {
				deleteResources(e2eClient, test.ExpectedResources...)
			})

			// sanity check
			expectResourcesNotExist(e2eClient, test.ExpectedResources...)

			// create policy
			policy := createResource(t, e2eClient, test.ClusterPolicy)
			Expect(commonE2E.PolicyCreated(policy.GetName())).To(Succeed())

			// create source resources
			createResources(t, e2eClient, test.SourceResources...)

			// create trigger
			createResource(t, e2eClient, test.TriggerResource)

			time.Sleep(time.Second * 5)

			for _, step := range test.Steps {
				Expect(step(e2eClient)).To(Succeed())
			}

			// verify expected resources
			expectResources(e2eClient, test.ExpectedResources...)
		})
	}
}

func Test_ClusterRole_ClusterRoleBinding_Sets(t *testing.T) {
	runTestCases(t, ClusterRoleTests...)
}

func Test_Role_RoleBinding_Sets(t *testing.T) {
	runTestCases(t, RoleTests...)
}

func Test_Generate_NetworkPolicy(t *testing.T) {
	runTestCases(t, NetworkPolicyGenerateTests...)
}

func Test_Generate_Namespace_Label_Actions(t *testing.T) {
	runTestCases(t, GenerateNetworkPolicyOnNamespaceWithoutLabelTests...)
}

func loopElement(found bool, elementObj interface{}) bool {
	if found == true {
		return found
	}
	switch typedelementObj := elementObj.(type) {
	case map[string]interface{}:
		for k, v := range typedelementObj {
			if k == "protocol" {
				if v == "TCP" {
					found = true
					return found
				}
			} else {
				found = loopElement(found, v)
			}
		}
	case []interface{}:
		found = loopElement(found, typedelementObj[0])
	case string:
		return found
	case int64:
		return found
	default:
		fmt.Println("unexpected type :", fmt.Sprintf("%T", elementObj))
		return found
	}
	return found
}

func Test_Generate_Synchronize_Flag(t *testing.T) {
	setup(t)

	// Generate E2E Client ==================
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())
	// ======================================

	// ====== Range Over RuleTest ==================
	for _, test := range GenerateSynchronizeFlagTests {
		By(fmt.Sprintf("Test to generate NetworkPolicy : %s", test.TestName))
		By(fmt.Sprintf("synchronize = %v\t clone = %v", test.Sync, test.Clone))

		// ======= CleanUp Resources =====
		By("Cleaning Cluster Policies")
		_ = e2eClient.CleanClusterPolicies(clPolGVR)

		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace : %s", test.ResourceNamespace))
		_ = e2eClient.DeleteClusteredResource(nsGVR, test.ResourceNamespace)

		// Wait Till Deletion of Namespace
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, test.ResourceNamespace)
			if err != nil {
				return nil
			}
			return errors.New("deleting Namespace")
		})
		Expect(err).NotTo(HaveOccurred())

		// ======== Create Generate NetworkPolicy Policy =============
		By("Creating Generate NetworkPolicy Policy")
		_, err = e2eClient.CreateNamespacedResourceYaml(clPolGVR, npPolNS, test.GeneratePolicyName, test.Data)
		Expect(err).NotTo(HaveOccurred())

		err = commonE2E.PolicyCreated(test.GeneratePolicyName)
		Expect(err).NotTo(HaveOccurred())

		// ======= Create Namespace ==================
		By(fmt.Sprintf("Creating Namespace which triggers generate %s", npPolNS))
		_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, namespaceWithLabelYaml)
		Expect(err).NotTo(HaveOccurred())

		// Wait Till Creation of Namespace
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, test.ResourceNamespace)
			if err != nil {
				return err
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// ======== NetworkPolicy Creation =====
		By(fmt.Sprintf("Verifying NetworkPolicy in the Namespace : %s", test.ResourceNamespace))
		// Wait Till Creation of NetworkPolicy
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
			if err != nil {
				return err
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		npRes, err := e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
		Expect(err).NotTo(HaveOccurred())
		Expect(npRes.GetName()).To(Equal(test.NetworkPolicyName))
		// ============================================

		// Test: when synchronize flag is set to true in the policy and someone deletes the generated resource, kyverno generates back the resource
		// ======= Delete Networkpolicy =====
		By(fmt.Sprintf("Deleting NetworkPolicy %s in the Namespace : %s", test.NetworkPolicyName, test.ResourceNamespace))
		err = e2eClient.DeleteNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

		// ======= Check Networkpolicy =====
		By(fmt.Sprintf("Checking NetworkPolicy %s in the Namespace : %s", test.NetworkPolicyName, test.ResourceNamespace))
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
			if err != nil {
				return err
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

		// Test: change synchronize to false in the policy, the label in generated resource should be updated to policy.kyverno.io/synchronize: disable
		// check for metadata.resourceVersion in policy - need to add this feild while updating the policy
		By(fmt.Sprintf("Update synchronize to true in generate policy: %s", test.GeneratePolicyName))
		genPolicy, err := e2eClient.GetNamespacedResource(clPolGVR, "", test.GeneratePolicyName)
		Expect(err).NotTo(HaveOccurred())

		resVer := genPolicy.GetResourceVersion()
		unstructGenPol := unstructured.Unstructured{}
		err = yaml.Unmarshal(test.UpdateData, &unstructGenPol)
		Expect(err).NotTo(HaveOccurred())
		unstructGenPol.SetResourceVersion(resVer)

		// ======== Update Generate NetworkPolicy =============
		_, err = e2eClient.UpdateNamespacedResource(clPolGVR, npPolNS, &unstructGenPol)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

		By(fmt.Sprintf("Verify the label in the updated network policy: %s", test.NetworkPolicyName))
		// get updated network policy and verify the label
		synchronizeFlagValueGotUpdated := false
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			netpol, err := e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
			if err != nil {
				return err
			}
			netPolLabels := netpol.GetLabels()
			if netPolLabels["policy.kyverno.io/synchronize"] != "disable" {
				return errors.New("still enabled")
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		netpol, err := e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
		Expect(err).NotTo(HaveOccurred())
		netPolLabels := netpol.GetLabels()
		if netPolLabels["policy.kyverno.io/synchronize"] == "disable" {
			synchronizeFlagValueGotUpdated = true
		}

		Expect(synchronizeFlagValueGotUpdated).To(Equal(true))
		// ============================================

		// Test: with synchronize is false, one should be able to delete the generated resource
		// ======= Delete Networkpolicy =====
		By(fmt.Sprintf("Deleting NetworkPolicy %s in the Namespace : %s", test.NetworkPolicyName, test.ResourceNamespace))
		err = e2eClient.DeleteNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

		// ======= Check Networkpolicy =====
		By(fmt.Sprintf("Checking NetworkPolicy %s in the Namespace : %s", test.NetworkPolicyName, test.ResourceNamespace))

		netpolGotDeleted := false
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
			if err != nil {
				netpolGotDeleted = true
			} else {
				return errors.New("network policy still exists")
			}
			return nil
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(netpolGotDeleted).To(Equal(true))

		// ======= CleanUp Resources =====
		_ = e2eClient.CleanClusterPolicies(clPolGVR)

		// Clear Namespace
		_ = e2eClient.DeleteClusteredResource(nsGVR, test.ResourceNamespace)

		// Wait Till Deletion of Namespace
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, test.ResourceNamespace)
			if err != nil {
				return nil
			}
			return errors.New("deleting Namespace")
		})

		// Do not fail if waiting fails. Sometimes namespace needs time to be deleted.
		if err != nil {
			By(err.Error())
		}

		By(fmt.Sprintf("Test %s Completed \n\n\n", test.TestName))
	}
}

func Test_Source_Resource_Update_Replication(t *testing.T) {
	setup(t)

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
		_ = e2eClient.CleanClusterPolicies(clPolGVR)

		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace : %s", tests.ResourceNamespace))
		_ = e2eClient.DeleteClusteredResource(nsGVR, tests.ResourceNamespace)

		// If Clone is true Clear Source Resource and Recreate
		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Deleting Source Resource from Clone Namespace : %s", tests.CloneNamespace))
			// Delete ConfigMap to be cloned
			_ = e2eClient.DeleteNamespacedResource(cmGVR, tests.CloneNamespace, tests.ConfigMapName)
		}

		// Wait Till Deletion of Namespace
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, tests.ResourceNamespace)
			if err != nil {
				return nil
			}
			return fmt.Errorf("failed to delete namespace: %v", err)
		})
		Expect(err).NotTo(HaveOccurred())

		// === If Clone is true Create Source Resources ==
		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Creating Cloner Resources in Namespace : %s", tests.CloneNamespace))
			_, err := e2eClient.CreateNamespacedResourceYaml(cmGVR, tests.CloneNamespace, "", tests.CloneSourceConfigMapData)
			Expect(err).NotTo(HaveOccurred())
		}
		// ================================================

		// ======== Create Generate Policy =============
		By(fmt.Sprintf("\nCreating Generate Policy in %s", clPolNS))
		_, err = e2eClient.CreateNamespacedResourceYaml(clPolGVR, clPolNS, tests.PolicyName, tests.Data)
		Expect(err).NotTo(HaveOccurred())

		err = commonE2E.PolicyCreated(tests.PolicyName)
		Expect(err).NotTo(HaveOccurred())

		// ======= Create Namespace ==================
		By(fmt.Sprintf("Creating Namespace which triggers generate %s", clPolNS))
		_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, namespaceYaml)
		Expect(err).NotTo(HaveOccurred())

		// Wait Till Creation of Namespace
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, tests.ResourceNamespace)
			if err != nil {
				return err
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// ======== Verify Configmap Creation =====
		By(fmt.Sprintf("Verifying Configmap in the Namespace : %s", tests.ResourceNamespace))

		// Wait Till Creation of Configmap
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetNamespacedResource(cmGVR, tests.ResourceNamespace, tests.ConfigMapName)
			if err != nil {
				return err
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// test: when a source clone resource is updated, the same changes should be replicated in the generated resource
		// ======= Update Configmap in default Namespace ========
		By(fmt.Sprintf("Updating Source Resource(Configmap) in Clone Namespace : %s", tests.CloneNamespace))

		// Update the configmap in default namespace
		sourceRes, err := e2eClient.GetNamespacedResource(cmGVR, tests.CloneNamespace, tests.ConfigMapName)
		Expect(err).NotTo(HaveOccurred())
		Expect(sourceRes.GetName()).To(Equal(tests.ConfigMapName))

		element, _, err := unstructured.NestedMap(sourceRes.UnstructuredContent(), "data")
		Expect(err).NotTo(HaveOccurred())
		element["initial_lives"] = "5"

		err = unstructured.SetNestedMap(sourceRes.UnstructuredContent(), element, "data")
		Expect(err).NotTo(HaveOccurred())

		_, err = e2eClient.UpdateNamespacedResource(cmGVR, tests.CloneNamespace, sourceRes)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

		// ======= Verifying Configmap Data Replication in Namespace ========
		By(fmt.Sprintf("Verifying Configmap Data Replication in the Namespace : %s", tests.ResourceNamespace))
		err = e2e.GetWithRetry(1*time.Second, 30, func() error {
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
				return fmt.Errorf("config map value not updated, found %v expected map[initial_lives:5]", element)
			}

			return nil
		})
		Expect(err).NotTo(HaveOccurred())

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

		_ = unstructured.SetNestedMap(genRes.UnstructuredContent(), element, "data")
		_, err = e2eClient.UpdateNamespacedResource(cmGVR, tests.ResourceNamespace, genRes)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

		// ======= Verifying Configmap Data in Namespace ========
		By(fmt.Sprintf("Verifying Configmap Data in the Namespace : %s", tests.ResourceNamespace))
		err = e2e.GetWithRetry(1*time.Second, 30, func() error {
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
				return fmt.Errorf("config map value not reset, found %v expected map[initial_lives:5]", element)
			}

			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		updatedGenRes, err = e2eClient.GetNamespacedResource(cmGVR, tests.ResourceNamespace, tests.ConfigMapName)
		Expect(err).NotTo(HaveOccurred())
		element, _, err = unstructured.NestedMap(updatedGenRes.UnstructuredContent(), "data")
		Expect(err).NotTo(HaveOccurred())
		Expect(element["initial_lives"]).To(Equal("5"))
		// ============================================

		// ======= CleanUp Resources =====
		_ = e2eClient.CleanClusterPolicies(clPolGVR)

		// === If Clone is true Delete Source Resources ==
		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Deleting Cloner Resources in Namespace : %s", tests.CloneNamespace))
			_ = e2eClient.DeleteNamespacedResource(cmGVR, tests.CloneNamespace, tests.ConfigMapName)
		}
		// ================================================

		// Clear Namespace
		_ = e2eClient.DeleteClusteredResource(nsGVR, tests.ResourceNamespace)

		// Wait Till Deletion of Namespace
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, tests.ResourceNamespace)
			if err != nil {
				return nil
			}
			return fmt.Errorf("failed to delete namespace: %v", err)
		})
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Test %s Completed \n\n\n", tests.TestName))
	}

}

func Test_Generate_Policy_Deletion_for_Clone(t *testing.T) {
	runTestCases(t, GeneratePolicyDeletionforCloneTests...)
}
