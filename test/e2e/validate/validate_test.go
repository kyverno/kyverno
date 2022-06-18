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
	// Cluster Policy GVR
	policyGVR = e2e.GetGVR("kyverno.io", "v1", "clusterpolicies")
	// Namespace GVR
	namespaceGVR = e2e.GetGVR("", "v1", "namespaces")
	// CRD GVR
	crdGVR = e2e.GetGVR("apiextensions.k8s.io", "v1", "customresourcedefinitions")
	// ClusterPolicy Namespace
	policyNamespace = "flux-multi-tenancy"
	// CRD Name
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

	for _, test := range FluxValidateTests {
		By(fmt.Sprintf("Test to validate objects: \"%s\"", test.TestName))
		// Clean up Resources
		By("Deleting Cluster Policies...")
		_ = e2eClient.CleanClusterPolicies(policyGVR)

		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace: %s...", test.ResourceNamespace))
		_ = e2eClient.DeleteClusteredResource(namespaceGVR, test.ResourceNamespace)

		// Wait Till Deletion of Namespace
		By(fmt.Sprintf("Wait Till Deletion of Namespace: %s...", test.ResourceNamespace))
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetClusteredResource(namespaceGVR, test.ResourceNamespace)
			if err != nil {
				return nil
			}
			return fmt.Errorf("Failed to delete namespace '%s': %v", test.ResourceNamespace, err)
		})
		Expect(err).NotTo(HaveOccurred())

		// Clean up CRDs
		_ = e2eClient.DeleteClusteredResource(crdGVR, crdName)

		// Wait Till Deletion of CRD
		err = e2e.GetWithRetry(time.Duration(1*time.Second), 15, func() error {
			_, err := e2eClient.GetClusteredResource(namespaceGVR, test.ResourceNamespace)
			if err != nil {
				return nil
			}
			return fmt.Errorf("Failed to delete CRD: %v", err)
		})
		Expect(err).NotTo(HaveOccurred())

		// Create Namespace
		By(fmt.Sprintf("Creating Namespace: %s...", test.ResourceNamespace))
		_, err = e2eClient.CreateClusteredResourceYaml(namespaceGVR, newNamespaceYaml(test.ResourceNamespace))
		Expect(err).NotTo(HaveOccurred())

		// Wait Till Creation of Namespace
		By(fmt.Sprintf("Wait Till Creation of Namespace: %s...", test.ResourceNamespace))
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetClusteredResource(namespaceGVR, test.ResourceNamespace)
			if err != nil {
				return err
			}
			return fmt.Errorf("Failed to create namespace '%s': %v", test.ResourceNamespace, err)
		})
		Expect(err).NotTo(HaveOccurred())

		// Create policy
		By(fmt.Sprintf("Creating Policy in \"%s\"...", policyNamespace))
		_, err = e2eClient.CreateNamespacedResourceYaml(policyGVR, policyNamespace, test.PolicyName, test.PolicyRaw)
		Expect(err).NotTo(HaveOccurred())

		err = commonE2E.PolicyCreated(test.PolicyName)
		Expect(err).NotTo(HaveOccurred())

		// Create Flux CRD
		By(fmt.Sprintf("Creating Flux CRD in \"%s\"...", test.ResourceNamespace))
		_, err = e2eClient.CreateClusteredResourceYaml(crdGVR, kyverno_2043_FluxCRD)
		Expect(err).NotTo(HaveOccurred())

		// Wait till CRD is created
		err = e2e.GetWithRetry(time.Duration(1*time.Second), 15, func() error {
			_, err := e2eClient.GetClusteredResource(crdGVR, crdName)
			if err == nil {
				return nil
			}
			return fmt.Errorf("Failed to create CRD: %v", err)
		})
		Expect(err).NotTo(HaveOccurred())

		// Created CRD is not a guarantee that we already can create new resources
		time.Sleep(3 * time.Second)

		// Create Kustomize resource
		kustomizeGVR := e2e.GetGVR("kustomize.toolkit.fluxcd.io", "v1beta1", "kustomizations")
		By(fmt.Sprintf("Creating Kustomize resource in \"%s\"", test.ResourceNamespace))
		_, err = e2eClient.CreateNamespacedResourceYaml(kustomizeGVR, test.ResourceNamespace, "", test.ResourceRaw)

		if test.MustSucceed {
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
		}

		//CleanUp Resources
		_ = e2eClient.CleanClusterPolicies(policyGVR)

		//CleanUp CRDs
		_ = e2eClient.DeleteClusteredResource(crdGVR, crdName)

		// Clear Namespace
		_ = e2eClient.DeleteClusteredResource(namespaceGVR, test.ResourceNamespace)

		// Wait Till Deletion of Namespace
		err = e2e.GetWithRetry(time.Duration(1*time.Second), 15, func() error {
			_, err := e2eClient.GetClusteredResource(namespaceGVR, test.ResourceNamespace)
			if err != nil {
				return nil
			}
			return fmt.Errorf("Failed to delete namespace '%s': %v", test.ResourceNamespace, err)
		})
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Test %s Completed \n\n\n", test.TestName))
	}
}

func TestValidate(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}

	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())

	for _, test := range ValidateTests {
		By(fmt.Sprintf("Mutation Test: %s", test.TestDescription))

		By("Deleting Cluster Policies...")
		_ = e2eClient.CleanClusterPolicies(policyGVR)

		By("Deleting Resource...")
		_ = e2eClient.DeleteNamespacedResource(test.ResourceGVR, test.ResourceNamespace, test.ResourceName)

		By("Deleting Namespace...")
		By(fmt.Sprintf("Deleting Namespace: %s...", test.ResourceNamespace))
		_ = e2eClient.DeleteClusteredResource(namespaceGVR, test.ResourceNamespace)

		By("Wait Till Deletion of Namespace...")
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetClusteredResource(namespaceGVR, test.ResourceNamespace)
			if err != nil {
				return nil
			}
			return fmt.Errorf("Failed to delete namespace '%s': %v", test.ResourceNamespace, err)
		})
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Creating Namespace: %s...", policyNamespace))
		_, err = e2eClient.CreateClusteredResourceYaml(namespaceGVR, newNamespaceYaml(test.ResourceNamespace))
		Expect(err).NotTo(HaveOccurred())

		By("Wait Till Creation of Namespace...")
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetClusteredResource(namespaceGVR, test.ResourceNamespace)
			if err != nil {
				return err
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		By("Creating Policy...")
		_, err = e2eClient.CreateNamespacedResourceYaml(policyGVR, policyNamespace, test.PolicyName, test.PolicyRaw)
		Expect(err).NotTo(HaveOccurred())

		err = commonE2E.PolicyCreated(test.PolicyName)
		Expect(err).NotTo(HaveOccurred())

		By("Creating Resource...")
		_, err = e2eClient.CreateNamespacedResourceYaml(test.ResourceGVR, test.ResourceNamespace, test.PolicyName, test.ResourceRaw)

		statusErr, ok := err.(*k8sErrors.StatusError)
		validationError := (ok && statusErr.ErrStatus.Code == 400) // Validation error is always Bad Request

		if test.MustSucceed || !validationError {
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
		}

		By("Deleting Cluster Policies...")
		err = e2eClient.CleanClusterPolicies(policyGVR)
		Expect(err).NotTo(HaveOccurred())

		By("Deleting Resource...") // if it is present, so ignore an error
		e2eClient.DeleteNamespacedResource(test.ResourceGVR, test.ResourceNamespace, test.ResourceName)

		By("Deleting Namespace...")
		err = e2eClient.DeleteClusteredResource(namespaceGVR, test.ResourceNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Wait Till Deletion of Namespace...")
		e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetClusteredResource(namespaceGVR, test.ResourceNamespace)
			if err != nil {
				return nil
			}
			return fmt.Errorf("Failed to delete namespace '%s': %v", test.ResourceNamespace, err)
		})

		// Do not fail if waiting fails. Sometimes namespace needs time to be deleted.
		By("Done")
	}
}
