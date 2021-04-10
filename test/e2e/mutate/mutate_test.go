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
)

var (
	// Cluster Polict GVR
	clPolGVR = e2e.GetGVR("kyverno.io", "v1", "clusterpolicies")
	// Namespace GVR
	nsGVR = e2e.GetGVR("", "v1", "namespaces")
	// ConfigMap GVR
	cmGVR = e2e.GetGVR("", "v1", "configmaps")

	// ClusterPolicy Namespace
	clPolNS = ""
	// Namespace Name
	// Hardcoded in YAML Definition
	nspace = "test-mutate"
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
		By(fmt.Sprintf("Cleaning Cluster Policies"))
		e2eClient.CleanClusterPolicies(clPolGVR)
		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace : %s", nspace))
		e2eClient.DeleteClusteredResource(nsGVR, nspace)

		// Wait Till Deletion of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, nspace)
			if err != nil {
				return nil
			}
			return errors.New("Deleting Namespace")
		})

		// Create Namespace
		By(fmt.Sprintf("Creating Namespace %s", clPolNS))
		_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, namespaceYaml)
		Expect(err).NotTo(HaveOccurred())

		// Create source CM
		By(fmt.Sprintf("\nCreating source ConfigMap in %s", nspace))
		_, err = e2eClient.CreateNamespacedResourceYaml(cmGVR, nspace, sourceConfigMapYaml)
		Expect(err).NotTo(HaveOccurred())

		// Create CM Policy
		By(fmt.Sprintf("\nCreating Mutate ConfigMap Policy in %s", clPolNS))
		_, err = e2eClient.CreateNamespacedResourceYaml(clPolGVR, clPolNS, tests.Data)
		Expect(err).NotTo(HaveOccurred())

		// Create target CM
		By(fmt.Sprintf("\nCreating target ConfigMap in %s", nspace))
		_, err = e2eClient.CreateNamespacedResourceYaml(cmGVR, nspace, targetConfigMapYaml)
		Expect(err).NotTo(HaveOccurred())

		// Verify created ConfigMap
		By(fmt.Sprintf("Verifying ConfigMap in the Namespace : %s", nspace))
		// Wait Till Creation of ConfigMap
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetNamespacedResource(cmGVR, nspace, "target")
			if err != nil {
				return err
			}
			return nil
		})
		cmRes, err := e2eClient.GetNamespacedResource(cmGVR, nspace, "target")
		Expect(err).NotTo(HaveOccurred())
		Expect(cmRes.GetLabels()["kyverno.key/copy-me"]).To(Equal("sample-value"))

		//CleanUp Resources
		e2eClient.CleanClusterPolicies(clPolGVR)

		// Clear Namespace
		e2eClient.DeleteClusteredResource(nsGVR, nspace)
		// Wait Till Deletion of Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, nspace)
			if err != nil {
				return nil
			}
			return errors.New("Deleting Namespace")
		})

		By(fmt.Sprintf("Test %s Completed \n\n\n", tests.TestName))
	}

}
