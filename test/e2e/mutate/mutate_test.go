package mutate

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kyverno/kyverno/test/e2e"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	// Cluster Policy GVR
	clPolGVR = e2e.GetGVR("kyverno.io", "v1", "clusterpolicies")
	// Namespace GVR
	nsGVR = e2e.GetGVR("", "v1", "namespaces")
	// ConfigMap GVR
	cmGVR = e2e.GetGVR("", "v1", "configmaps")

	// ClusterPolicy Namespace
	clPolNS = ""
	// Namespace Name
	// Hardcoded in YAML Definition
	// nspace = "test-mutate"
)

func Test_Mutate_Sets(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}
	// Generate E2E Client
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())

	for _, tests := range MutateTests {
		By(fmt.Sprintf("Test to mutate objects : %s", tests.TestName))

		// Clean up Resources
		By("Cleaning Cluster Policies")
		e2eClient.CleanClusterPolicies(clPolGVR)
		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace : %s", tests.ResourceNamespace))
		e2eClient.DeleteClusteredResource(nsGVR, tests.ResourceNamespace)

		// Wait Till Deletion of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, tests.ResourceNamespace)
			if err != nil {
				return nil
			}
			return errors.New("Deleting Namespace")
		})

		// Create Namespace
		By(fmt.Sprintf("Creating Namespace %s", clPolNS))
		_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, newNamespaceYaml("test-mutate"))
		Expect(err).NotTo(HaveOccurred())

		// Wait Till Creation of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, tests.ResourceNamespace)
			if err != nil {
				return err
			}

			return nil
		})

		// Create source CM
		By(fmt.Sprintf("\nCreating source ConfigMap in %s", tests.ResourceNamespace))
		_, err = e2eClient.CreateNamespacedResourceYaml(cmGVR, tests.ResourceNamespace, sourceConfigMapYaml)
		Expect(err).NotTo(HaveOccurred())

		// Create CM Policy
		By(fmt.Sprintf("\nCreating Mutate ConfigMap Policy in %s", clPolNS))
		_, err = e2eClient.CreateNamespacedResourceYaml(clPolGVR, clPolNS, tests.Data)
		Expect(err).NotTo(HaveOccurred())

		// Create target CM
		By(fmt.Sprintf("\nCreating target ConfigMap in %s", tests.ResourceNamespace))
		_, err = e2eClient.CreateNamespacedResourceYaml(cmGVR, tests.ResourceNamespace, targetConfigMapYaml)
		Expect(err).NotTo(HaveOccurred())

		// Verify created ConfigMap
		By(fmt.Sprintf("Verifying ConfigMap in the Namespace : %s", tests.ResourceNamespace))
		// Wait Till Creation of ConfigMap
		err = e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetNamespacedResource(cmGVR, tests.ResourceNamespace, "target")
			if err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			fmt.Println("1. error occurred while verifing ConfigMap:", err)
		}

		cmRes, err := e2eClient.GetNamespacedResource(cmGVR, tests.ResourceNamespace, "target")
		if err != nil {
			fmt.Println("2. error occurred while verifing ConfigMap:", err)
		}
		Expect(err).NotTo(HaveOccurred())
		Expect(cmRes.GetLabels()["kyverno.key/copy-me"]).To(Equal("sample-value"))

		//CleanUp Resources
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

func Test_Mutate_Ingress(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}

	// Generate E2E Client
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())

	nspace := ingressTests.testNamesapce
	By(fmt.Sprintf("Cleaning Cluster Policies"))
	e2eClient.CleanClusterPolicies(clPolGVR)

	By(fmt.Sprintf("Deleting Namespace : %s", nspace))
	e2eClient.DeleteClusteredResource(nsGVR, nspace)

	// Wait Till Deletion of Namespace
	err = e2e.GetWithRetry(time.Duration(1), 15, func() error {
		_, err := e2eClient.GetClusteredResource(nsGVR, nspace)
		if err != nil {
			return nil
		}
		return errors.New("Deleting Namespace")
	})
	Expect(err).To(BeNil())

	By(fmt.Sprintf("Creating mutate ClusterPolicy "))
	_, err = e2eClient.CreateClusteredResourceYaml(clPolGVR, ingressTests.cpol)
	Expect(err).NotTo(HaveOccurred())

	By(fmt.Sprintf("Creating Namespace %s", nspace))
	_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, newNamespaceYaml(nspace))
	Expect(err).NotTo(HaveOccurred())

	for _, test := range ingressTests.tests {
		By(fmt.Sprintf("\n\nStart testing %s", test.testName))

		gvr := e2e.GetGVR(test.group, test.version, test.rsc)
		By(fmt.Sprintf("Creating Ingress %v in %s", gvr, nspace))
		_, err = e2eClient.CreateNamespacedResourceYaml(gvr, nspace, test.resource)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Verifying Ingress %v in the Namespace : %s", gvr, nspace))
		var mutatedResource *unstructured.Unstructured
		err = e2e.GetWithRetry(time.Duration(1), 15, func() error {
			mutatedResource, err = e2eClient.GetNamespacedResource(gvr, nspace, test.resourceName)
			if err != nil {
				return err
			}
			return nil
		})
		Expect(err).To(BeNil())

		By(fmt.Sprintf("Comparing patched field"))
		rules, ok, err := unstructured.NestedSlice(mutatedResource.UnstructuredContent(), "spec", "rules")
		Expect(err).To(BeNil())
		Expect(ok).To(BeTrue())
		rule := rules[0].(map[string]interface{})
		host := rule["host"].(string)
		Expect(host).To(Equal("kuard.mycompany.com"))
	}
}
