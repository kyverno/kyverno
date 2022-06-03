package generate

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kyverno/kyverno/test/e2e"
	commonE2E "github.com/kyverno/kyverno/test/e2e/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func setup(t *testing.T) {
	RegisterTestingT(t)
	// if os.Getenv("E2E") == "" {
	// 	t.Skip("Skipping E2E Test")
	// }
}

func createClient() *e2e.E2EClient {
	client, err := e2e.NewE2EClient()
	Expect(err).NotTo(HaveOccurred())
	return client
}

func deleteClusteredResource(client *e2e.E2EClient, resource expectedResource) {
	By(fmt.Sprintf("Deleting %s : %s", resource.gvr.String(), resource.name))
	client.DeleteClusteredResource(resource.gvr, resource.name)
	err := e2e.GetWithRetry(1*time.Second, 15, func() error {
		_, err := client.GetClusteredResource(resource.gvr, resource.name)
		if err == nil {
			return fmt.Errorf("resource still exists: %s, %s", resource.gvr.String(), resource.name)
		}
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	Expect(err).NotTo(HaveOccurred())
}

func deleteNamespacedResource(client *e2e.E2EClient, resource expectedResource) {
	By(fmt.Sprintf("Deleting %s : %s/%s", resource.gvr.String(), resource.ns, resource.name))
	client.DeleteNamespacedResource(resource.gvr, resource.ns, resource.name)
	err := e2e.GetWithRetry(1*time.Second, 15, func() error {
		_, err := client.GetNamespacedResource(resource.gvr, resource.ns, resource.name)
		if err == nil {
			return fmt.Errorf("resource still exists: %s, %s/%s", resource.gvr.String(), resource.ns, resource.name)
		}
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	Expect(err).NotTo(HaveOccurred())
}

func deleteResource(client *e2e.E2EClient, resource expectedResource) {
	if resource.ns != "" {
		deleteNamespacedResource(client, resource)
	} else {
		deleteClusteredResource(client, resource)
	}
}

func deleteResources(client *e2e.E2EClient, resources ...expectedResource) {
	for _, resource := range resources {
		deleteResource(client, resource)
	}
}

func createClusteredResource(t *testing.T, client *e2e.E2EClient, resource resource) *unstructured.Unstructured {
	var u unstructured.Unstructured
	Expect(yaml.Unmarshal(resource.raw, &u)).To(Succeed())
	By(fmt.Sprintf("Creating %s : %s", resource.gvr.String(), u.GetName()))
	result, err := client.CreateClusteredResource(resource.gvr, &u)
	Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		deleteResources(client, expectedResource{resource.gvr, result.GetNamespace(), result.GetName()})
	})
	return result
}

func createNamespacedResource(t *testing.T, client *e2e.E2EClient, resource resource) *unstructured.Unstructured {
	var u unstructured.Unstructured
	Expect(yaml.Unmarshal(resource.raw, &u)).To(Succeed())
	By(fmt.Sprintf("Creating %s : %s/%s", resource.gvr.String(), resource.ns, u.GetName()))
	result, err := client.CreateNamespacedResource(resource.gvr, resource.ns, &u)
	Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		deleteResources(client, expectedResource{resource.gvr, result.GetNamespace(), result.GetName()})
	})
	return result
}

func createResource(t *testing.T, client *e2e.E2EClient, resource resource) *unstructured.Unstructured {
	if resource.ns != "" {
		return createNamespacedResource(t, client, resource)
	} else {
		return createClusteredResource(t, client, resource)
	}
}

func createResources(t *testing.T, client *e2e.E2EClient, resources ...resource) {
	for _, resource := range resources {
		createResource(t, client, resource)
	}
}

func expectClusteredResource(client *e2e.E2EClient, resource expectedResource) {
	By(fmt.Sprintf("Expecting %s : %s", resource.gvr.String(), resource.name))
	err := e2e.GetWithRetry(1*time.Second, 30, func() error {
		_, err := client.GetClusteredResource(resource.gvr, resource.name)
		if err != nil {
			return err
		}
		return nil
	})
	Expect(err).NotTo(HaveOccurred())
}

func expectNamespacedResource(client *e2e.E2EClient, resource expectedResource) {
	By(fmt.Sprintf("Expecting %s : %s/%s", resource.gvr.String(), resource.ns, resource.name))
	err := e2e.GetWithRetry(1*time.Second, 30, func() error {
		_, err := client.GetNamespacedResource(resource.gvr, resource.ns, resource.name)
		if err != nil {
			return err
		}
		return nil
	})
	Expect(err).NotTo(HaveOccurred())
}

func expectResource(client *e2e.E2EClient, resource expectedResource) {
	if resource.ns != "" {
		expectNamespacedResource(client, resource)
	} else {
		expectClusteredResource(client, resource)
	}
}

func expectResources(client *e2e.E2EClient, resources ...expectedResource) {
	for _, resource := range resources {
		expectResource(client, resource)
	}
}

func expectClusteredResourceNotExists(client *e2e.E2EClient, resource expectedResource) {
	By(fmt.Sprintf("Expecting not exists %s : %s", resource.gvr.String(), resource.name))
	_, err := client.GetClusteredResource(resource.gvr, resource.name)
	Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func expectNamespacedResourceNotExists(client *e2e.E2EClient, resource expectedResource) {
	By(fmt.Sprintf("Expecting not exists %s : %s/%s", resource.gvr.String(), resource.ns, resource.name))
	_, err := client.GetNamespacedResource(resource.gvr, resource.ns, resource.name)
	Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func expectResourceNotExists(client *e2e.E2EClient, resource expectedResource) {
	if resource.ns != "" {
		expectNamespacedResourceNotExists(client, resource)
	} else {
		expectClusteredResourceNotExists(client, resource)
	}
}

func expectResourcesNotExist(client *e2e.E2EClient, resources ...expectedResource) {
	for _, resource := range resources {
		expectResourceNotExists(client, resource)
	}
}

func Test_ClusterRole_ClusterRoleBinding_Sets(t *testing.T) {
	setup(t)

	for _, tests := range ClusterRoleTests {
		t.Run(tests.TestName, func(t *testing.T) {
			e2eClient := createClient()

			t.Cleanup(func() {
				deleteResources(e2eClient, tests.ExpectedResources...)
			})

			// sanity check
			expectResourcesNotExist(e2eClient, tests.ExpectedResources...)

			// create policy
			policy := createResource(t, e2eClient, tests.ClusterPolicy)
			Expect(commonE2E.PolicyCreated(policy.GetName())).To(Succeed())

			// create source resources
			createResources(t, e2eClient, tests.SourceResources...)

			// create trigger
			createResource(t, e2eClient, tests.TriggerResource)

			// verify expected resources
			expectResources(e2eClient, tests.ExpectedResources...)
		})
	}
}

func Test_Role_RoleBinding_Sets(t *testing.T) {
	setup(t)

	for _, tests := range RoleTests {
		t.Run(tests.TestName, func(t *testing.T) {
			e2eClient := createClient()

			t.Cleanup(func() {
				deleteResources(e2eClient, tests.ExpectedResources...)
			})

			// sanity check
			expectResourcesNotExist(e2eClient, tests.ExpectedResources...)

			// create policy
			policy := createResource(t, e2eClient, tests.ClusterPolicy)
			Expect(commonE2E.PolicyCreated(policy.GetName())).To(Succeed())

			// create source resources
			createResources(t, e2eClient, tests.SourceResources...)

			// create trigger
			createResource(t, e2eClient, tests.TriggerResource)

			// verify expected resources
			expectResources(e2eClient, tests.ExpectedResources...)
		})
	}
}

func Test_Generate_NetworkPolicy(t *testing.T) {
	setup(t)

	// Generate E2E Client ==================
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())
	// ======================================

	// ====== Range Over RuleTest ==================
	for _, test := range NetworkPolicyGenerateTests {
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
		_, err := e2eClient.CreateNamespacedResourceYaml(clPolGVR, npPolNS, test.PolicyName, test.Data)
		Expect(err).NotTo(HaveOccurred())

		err = commonE2E.PolicyCreated(test.PolicyName)
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
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Test %s Completed \n\n\n", test.TestName))
	}
}

func Test_Generate_Namespace_Label_Actions(t *testing.T) {
	setup(t)

	// Generate E2E Client ==================
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())
	// ======================================

	// ====== Range Over RuleTest ==================
	for _, test := range GenerateNetworkPolicyOnNamespaceWithoutLabelTests {
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
		// ============================================

		err = commonE2E.PolicyCreated(test.GeneratePolicyName)
		Expect(err).NotTo(HaveOccurred())

		// Test: when creating the new namespace without the label, there should not have any generated resource
		// ======= Create Namespace ==================
		By(fmt.Sprintf("Creating Namespace which should not triggers generate policy %s", npPolNS))
		_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, namespaceYaml)
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
		Expect(err).To(HaveOccurred())

		_, err := e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
		Expect(err).To(HaveOccurred())
		// ============================================

		// Test: when adding the matched label to the namespace, the target resource should be generated
		By(fmt.Sprintf("Updating Namespace which triggers generate policy %s", npPolNS))
		// add label to the namespace
		_, err = e2eClient.UpdateClusteredResourceYaml(nsGVR, namespaceWithLabelYaml)
		Expect(err).NotTo(HaveOccurred())

		// ======== NetworkPolicy Creation =====
		By(fmt.Sprintf("Verifying NetworkPolicy in the updated Namespace : %s", test.ResourceNamespace))
		// Wait Till Creation of NetworkPolicy
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err = e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
			if err != nil {
				return err
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
		Expect(err).NotTo(HaveOccurred())

		// Test: when changing the content in generate.data, the change should be synced to the generated resource
		// check for metadata.resourceVersion in policy - need to add this feild while updating the policy
		By(fmt.Sprintf("Update generate policy: %s", test.GeneratePolicyName))
		genPolicy, err := e2eClient.GetNamespacedResource(clPolGVR, "", test.GeneratePolicyName)
		Expect(err).NotTo(HaveOccurred())

		resVer := genPolicy.GetResourceVersion()
		unstructGenPol := unstructured.Unstructured{}
		err = yaml.Unmarshal(test.UpdateData, &unstructGenPol)
		Expect(err).NotTo(HaveOccurred())
		unstructGenPol.SetResourceVersion(resVer)

		// ======== Update Generate NetworkPolicy =============
		By("Updating Generate NetworkPolicy")
		_, err = e2eClient.UpdateNamespacedResource(clPolGVR, npPolNS, &unstructGenPol)
		Expect(err).NotTo(HaveOccurred())

		// ======== Check Updated NetworkPolicy =============
		By(fmt.Sprintf("Verifying updated NetworkPolicy in the Namespace : %s", test.ResourceNamespace))

		err = e2e.GetWithRetry(1*time.Second, 30, func() error {
			// get updated network policy
			updatedNetPol, err := e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
			if err != nil {
				return err
			}

			// compare updated network policy and updated generate policy
			element, _, err := unstructured.NestedMap(updatedNetPol.UnstructuredContent(), "spec")
			if err != nil {
				return err
			}
			found := false
			found = loopElement(found, element)
			if found == false {
				return errors.New("not found")
			}

			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		updatedNetPol, err := e2eClient.GetNamespacedResource(npGVR, test.ResourceNamespace, test.NetworkPolicyName)
		Expect(err).NotTo(HaveOccurred())

		element, specFound, err := unstructured.NestedMap(updatedNetPol.UnstructuredContent(), "spec")
		found := loopElement(false, element)
		Expect(specFound).To(Equal(true))
		Expect(found).To(Equal(true))

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
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Test %s Completed \n\n\n", test.TestName))
	}
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
	setup(t)

	// Generate E2E Client ==================
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())
	// ======================================

	// ====== Range Over RuleTest ==================
	for _, tests := range GeneratePolicyDeletionforCloneTests {
		By(fmt.Sprintf("Test to check kyverno flow when generate policy is deleted: %s", tests.TestName))
		By(fmt.Sprintf("synchronize = %v\t clone = %v", tests.Sync, tests.Clone))

		// ======= CleanUp Resources =====
		By("Cleaning Cluster Policies")
		_ = e2eClient.CleanClusterPolicies(clPolGVR)

		// If Clone is true Clear Source Resource and Recreate
		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Deleting Source Resource from Clone Namespace : %s", tests.CloneNamespace))
			// Delete ConfigMap to be cloned
			_ = e2eClient.DeleteNamespacedResource(cmGVR, tests.CloneNamespace, tests.ConfigMapName)
		}

		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace : %s", tests.ResourceNamespace))
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

		// === If Clone is true Create Source Resources ==
		if tests.Clone {
			By(fmt.Sprintf("Clone = true, Creating Cloner Resources in Namespace : %s", tests.CloneNamespace))
			_, err := e2eClient.CreateNamespacedResourceYaml(cmGVR, tests.CloneNamespace, "", tests.CloneSourceConfigMapData)
			Expect(err).NotTo(HaveOccurred())
		}
		// ================================================

		// ======== Create Generate Policy =============
		By(fmt.Sprintf("\nCreating Generate Policy %s", tests.PolicyName))
		_, err = e2eClient.CreateNamespacedResourceYaml(clPolGVR, clPolNS, tests.PolicyName, tests.Data)
		Expect(err).NotTo(HaveOccurred())

		err = commonE2E.PolicyCreated(tests.PolicyName)
		Expect(err).NotTo(HaveOccurred())

		time.Sleep(5 * time.Second)

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
		// ===========================================

		// test: generated resource is not deleted after deletion of generate policy
		// ========== Delete the Generate Policy =============
		time.Sleep(2 * time.Second)
		By(fmt.Sprintf("Delete the generate policy : %s", tests.PolicyName))

		err = e2eClient.DeleteClusteredResource(clPolGVR, tests.PolicyName)
		Expect(err).NotTo(HaveOccurred())

		// Wait till policy is deleted
		err = e2e.GetWithRetry(1*time.Second, 30, func() error {
			_, err := e2eClient.GetClusteredResource(clPolGVR, tests.PolicyName)
			if err != nil {
				return errors.New("policy still exists")
			}
			return nil
		})
		Expect(err).To(HaveOccurred())

		_, err := e2eClient.GetClusteredResource(clPolGVR, tests.PolicyName)
		Expect(err).To(HaveOccurred())
		// ===========================================

		// ======= Check Generated Resources =======
		By(fmt.Sprintf("Checking the generated resource (Configmap) in namespace : %s", tests.ResourceNamespace))
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetNamespacedResource(cmGVR, tests.ResourceNamespace, tests.ConfigMapName)
			if err != nil {
				return err
			}
			return nil
		})

		Expect(err).NotTo(HaveOccurred())

		// ===========================================

		// test: the generated resource is not updated if the source resource is updated
		// ======= Update Configmap in default Namespace ========
		By(fmt.Sprintf("Updating Source Resource(Configmap) in Clone Namespace : %s", tests.CloneNamespace))

		// Get the configmap from default namespace
		sourceRes, err := e2eClient.GetNamespacedResource(cmGVR, tests.CloneNamespace, tests.ConfigMapName)
		Expect(err).NotTo(HaveOccurred())
		Expect(sourceRes.GetName()).To(Equal(tests.ConfigMapName))

		element, _, err := unstructured.NestedMap(sourceRes.UnstructuredContent(), "data")
		Expect(err).NotTo(HaveOccurred())
		element["initial_lives"] = "5"

		_ = unstructured.SetNestedMap(sourceRes.UnstructuredContent(), element, "data")
		_, err = e2eClient.UpdateNamespacedResource(cmGVR, tests.CloneNamespace, sourceRes)
		Expect(err).NotTo(HaveOccurred())
		// ============================================

		// ======= Verifying Configmap Data is Not Replicated in Generated Resource ========
		By("Verifying Configmap Data is Not Replicated in Generated Resource")

		updatedGenRes, err := e2eClient.GetNamespacedResource(cmGVR, tests.ResourceNamespace, tests.ConfigMapName)
		Expect(err).NotTo(HaveOccurred())
		element, _, err = unstructured.NestedMap(updatedGenRes.UnstructuredContent(), "data")
		Expect(err).NotTo(HaveOccurred())
		Expect(element["initial_lives"]).To(Equal("2"))
		// ============================================

		// test: the generated resource is not deleted after the source is deleted
		//=========== Delete the Clone Source Resource ============
		By(fmt.Sprintf("Delete the clone source resource: %s/%s", tests.CloneNamespace, tests.ConfigMapName))

		err = e2eClient.DeleteNamespacedResource(cmGVR, tests.CloneNamespace, tests.ConfigMapName)
		Expect(err).NotTo(HaveOccurred())

		// Wait till policy is deleted
		err = e2e.GetWithRetry(1*time.Second, 30, func() error {
			_, err := e2eClient.GetNamespacedResource(cmGVR, tests.CloneNamespace, tests.ConfigMapName)
			if err == nil {
				return errors.New("configmap not deleted")
			}
			return nil
		})
		Expect(err).ToNot(HaveOccurred())

		_, err = e2eClient.GetNamespacedResource(cmGVR, tests.CloneNamespace, tests.ConfigMapName)
		Expect(err).To(HaveOccurred())
		// ===========================================

		// ======= Check Generated Resources =======
		By(fmt.Sprintf("Checking the generated resource (Configmap) in namespace : %s", tests.ResourceNamespace))
		_, err = e2eClient.GetNamespacedResource(cmGVR, tests.ResourceNamespace, tests.ConfigMapName)
		Expect(err).NotTo(HaveOccurred())
		// ===========================================

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
		Expect(err).ToNot(HaveOccurred())

		// ====================================

		By(fmt.Sprintf("Test %s Completed \n\n\n", tests.TestName))
	}
}
