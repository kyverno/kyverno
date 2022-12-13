package validate

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kyverno/kyverno/test/e2e"
	commonE2E "github.com/kyverno/kyverno/test/e2e/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	// Cluster Polict GVR
	policyGVR = e2e.GetGVR("kyverno.io", "v1", "clusterpolicies")
	// Namespace GVR
	namespaceGVR = e2e.GetGVR("", "v1", "namespaces")

	crdGVR = e2e.GetGVR("apiextensions.k8s.io", "v1", "customresourcedefinitions")

	// ClusterPolicy Namespace
	policyNamespace = ""

	crdName = "kustomizations.kustomize.toolkit.fluxcd.io"
)

func Test_Validate_Flux_Sets(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}

	// Generate E2E Client
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())

	// Create Flux CRD
	err = createKustomizationCRD(e2eClient)
	Expect(err).NotTo(HaveOccurred())

	// Created CRD is not a guarantee that we already can create new resources
	time.Sleep(10 * time.Second)

	for _, test := range FluxValidateTests {
		By(fmt.Sprintf("Validate Test: %s", test.TestDescription))

		err = deleteClusterPolicy(e2eClient)
		Expect(err).NotTo(HaveOccurred())

		err = deleteResource(e2eClient, test)
		Expect(err).NotTo(HaveOccurred())

		err = deleteNamespace(e2eClient, test)
		Expect(err).NotTo(HaveOccurred())

		err = createNamespace(e2eClient, test)
		Expect(err).NotTo(HaveOccurred())

		err = createPolicy(e2eClient, test)
		Expect(err).NotTo(HaveOccurred())

		err = createResource(e2eClient, test)

		if test.MustSucceed {
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
		}

		err = deleteClusterPolicy(e2eClient)
		Expect(err).NotTo(HaveOccurred())

		err = deleteResource(e2eClient, test)
		Expect(err).NotTo(HaveOccurred())

		err = deleteNamespace(e2eClient, test)
		Expect(err).NotTo(HaveOccurred())

		By("Test passed successfully:" + test.TestDescription)
	}

	err = deleteKustomizationCRD(e2eClient)
	Expect(err).NotTo(HaveOccurred())
}

func TestValidate(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}

	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())

	for _, test := range ValidateTests {
		By(fmt.Sprintf("Validate Test: %s", test.TestDescription))

		err = deleteClusterPolicy(e2eClient)
		Expect(err).NotTo(HaveOccurred())

		err = deleteResource(e2eClient, test)
		Expect(err).NotTo(HaveOccurred())

		err = deleteNamespace(e2eClient, test)
		Expect(err).NotTo(HaveOccurred())

		err = createNamespace(e2eClient, test)
		Expect(err).NotTo(HaveOccurred())

		err = createPolicy(e2eClient, test)
		Expect(err).NotTo(HaveOccurred())

		err = createResource(e2eClient, test)

		statusErr, ok := err.(*k8sErrors.StatusError)
		validationError := ok && statusErr.ErrStatus.Code == 400 // Validation error is always Bad Request

		if test.MustSucceed || !validationError {
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
		}

		err = deleteClusterPolicy(e2eClient)
		Expect(err).NotTo(HaveOccurred())

		err = deleteResource(e2eClient, test)
		Expect(err).NotTo(HaveOccurred())

		err = deleteNamespace(e2eClient, test)
		Expect(err).NotTo(HaveOccurred())

		By("Done")
	}
}

func createNamespace(e2eClient *e2e.E2EClient, test ValidationTest) error {
	By(fmt.Sprintf("Creating Namespace: %s...", test.ResourceNamespace))
	_, err := e2eClient.CreateClusteredResourceYaml(namespaceGVR, newNamespaceYaml(test.ResourceNamespace))
	Expect(err).NotTo(HaveOccurred())

	By("Wait Till Creation of Namespace...")
	err = e2e.GetWithRetry(1*time.Second, 240, func() error {
		_, err := e2eClient.GetClusteredResource(namespaceGVR, test.ResourceNamespace)
		if err != nil {
			return err
		}

		return nil
	})
	return err
}

func createPolicy(e2eClient *e2e.E2EClient, test ValidationTest) error {
	By("Creating Policy...")
	_, err := e2eClient.CreateNamespacedResourceYaml(policyGVR, policyNamespace, test.PolicyName, test.PolicyRaw)
	Expect(err).NotTo(HaveOccurred())

	err = commonE2E.PolicyCreated(test.PolicyName)
	return err
}

func createResource(e2eClient *e2e.E2EClient, test ValidationTest) error {
	By("Creating Resource...")
	_, err := e2eClient.CreateNamespacedResourceYaml(test.ResourceGVR, test.ResourceNamespace, test.ResourceName, test.ResourceRaw)
	return err
}

func createKustomizationCRD(e2eClient *e2e.E2EClient) error {
	By("Creating Flux CRD")
	_, err := e2eClient.CreateClusteredResourceYaml(crdGVR, kyverno2043Fluxcrd)
	Expect(err).NotTo(HaveOccurred())

	// Wait till CRD is created
	By("Wait Till Creation of CRD...")
	err = e2e.GetWithRetry(1*time.Second, 240, func() error {
		_, err := e2eClient.GetClusteredResource(crdGVR, crdName)
		if err == nil {
			return nil
		}
		return fmt.Errorf("failed to create CRD: %v", err)
	})
	return err
}

func deleteClusterPolicy(e2eClient *e2e.E2EClient) error {
	By("Deleting Cluster Policies...")
	err := e2eClient.CleanClusterPolicies(policyGVR)
	return err
}

func deleteResource(e2eClient *e2e.E2EClient, test ValidationTest) error {
	By("Deleting Resource...")
	err := e2eClient.DeleteNamespacedResource(test.ResourceGVR, test.ResourceNamespace, test.ResourceName)
	if k8sErrors.IsNotFound(err) {
		return nil
	}
	return err
}

func deleteNamespace(e2eClient *e2e.E2EClient, test ValidationTest) error {
	By("Deleting Namespace...")
	By(fmt.Sprintf("Deleting Namespace: %s...", test.ResourceNamespace))
	_ = e2eClient.DeleteClusteredResource(namespaceGVR, test.ResourceNamespace)

	By("Wait Till Deletion of Namespace...")
	err := e2e.GetWithRetry(1*time.Second, 240, func() error {
		_, err := e2eClient.GetClusteredResource(namespaceGVR, test.ResourceNamespace)
		if err != nil {
			return nil
		}
		return fmt.Errorf("failed to delete namespace: %v", err)
	})
	return err
}

func deleteKustomizationCRD(e2eClient *e2e.E2EClient) error {
	By("Deleting Flux CRD")
	_ = e2eClient.DeleteClusteredResource(crdGVR, crdName)

	// Wait till CRD is deleted
	By("Wait Till Deletion of CRD...")
	err := e2e.GetWithRetry(1*time.Second, 240, func() error {
		_, err := e2eClient.GetClusteredResource(crdGVR, crdName)
		if err != nil {
			return nil
		}
		return fmt.Errorf("failed to delete CRD: %v", err)
	})
	return err
}
