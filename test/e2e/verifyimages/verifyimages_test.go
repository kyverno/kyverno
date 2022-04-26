package verifyimages

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
	policyGVR = e2e.GetGVR("kyverno.io", "v1", "clusterpolicies")
	// Namespace GVR
	namespaceGVR = e2e.GetGVR("", "v1", "namespaces")

	crdGVR = e2e.GetGVR("apiextensions.k8s.io", "v1", "customresourcedefinitions")

	// ClusterPolicy Namespace
	policyNamespace = ""
	// Namespace Name
	// Hardcoded in YAML Definition
	nspace  = "test-image-verify"
	crdName = "tasks.tekton.dev"
)

func TestImageVerify(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}

	// Generate E2E Client
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())
	for _, test := range VerifyImagesTests {
		By(fmt.Sprintf("Test to validate objects: \"%s\"", test.TestName))

		// Clean up Resources
		By(string("Cleaning Cluster Policies"))
		e2eClient.CleanClusterPolicies(policyGVR)
		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace: \"%s\"", nspace))
		e2eClient.DeleteClusteredResource(namespaceGVR, nspace)
		//CleanUp CRDs
		e2eClient.DeleteClusteredResource(crdGVR, crdName)

		// Wait Till Deletion of Namespace
		e2e.GetWithRetry(time.Duration(1*time.Second), 15, func() error {
			_, err := e2eClient.GetClusteredResource(namespaceGVR, nspace)
			if err != nil {
				return nil
			}
			return errors.New("Deleting Namespace")
		})

		// Create Namespace
		By(fmt.Sprintf("Creating namespace \"%s\"", nspace))
		_, err = e2eClient.CreateClusteredResourceYaml(namespaceGVR, namespaceYaml)
		Expect(err).NotTo(HaveOccurred())

		// Create Tekton CRD
		By(fmt.Sprintf("Creating Tekton CRD in \"%s\"", nspace))
		_, err = e2eClient.CreateClusteredResourceYaml(crdGVR, tektonTaskCRD)
		Expect(err).NotTo(HaveOccurred())

		// Wait till CRD is created
		e2e.GetWithRetry(time.Duration(1*time.Second), 15, func() error {
			_, err := e2eClient.GetClusteredResource(crdGVR, crdName)
			if err == nil {
				return nil
			}
			return errors.New("Waiting for CRD to be created...")
		})

		// Created CRD is not a garantee that we already can create new resources
		time.Sleep(3 * time.Second)

		// Create policy
		By(fmt.Sprintf("Creating policy in \"%s\"", policyNamespace))
		_, err = e2eClient.CreateNamespacedResourceYaml(policyGVR, policyNamespace, test.PolicyName, test.PolicyRaw)
		Expect(err).NotTo(HaveOccurred())

		if test.MustSucceed {
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
		}

		// Clean up policies
		By("Deleting Cluster Policies...")
		err = e2eClient.CleanClusterPolicies(policyGVR)
		Expect(err).NotTo(HaveOccurred())

		//CleanUp CRDs
		e2eClient.DeleteClusteredResource(crdGVR, crdName)

		// Clear Namespace
		e2eClient.DeleteClusteredResource(namespaceGVR, nspace)
		// Wait Till Deletion of Namespace
		e2e.GetWithRetry(time.Duration(1*time.Second), 15, func() error {
			_, err := e2eClient.GetClusteredResource(namespaceGVR, nspace)
			if err != nil {
				return nil
			}
			return errors.New("Deleting Namespace")
		})

		By(fmt.Sprintf("Test %s Completed \n\n\n", test.TestName))

	}

}
