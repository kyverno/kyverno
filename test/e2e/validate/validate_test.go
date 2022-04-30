package validate

import (
	"errors"
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
	// Namespace Name
	// Hardcoded in YAML Definition
	nspace = "test-validate"

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

		// Create policy
		By(fmt.Sprintf("Creating policy in \"%s\"", policyNamespace))
		_, err = e2eClient.CreateNamespacedResourceYaml(policyGVR, policyNamespace, "", test.PolicyRaw)
		Expect(err).NotTo(HaveOccurred())

		// Create Flux CRD
		By(fmt.Sprintf("Creating Flux CRD in \"%s\"", nspace))
		_, err = e2eClient.CreateClusteredResourceYaml(crdGVR, kyverno_2043_FluxCRD)
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

		// Create Kustomize resource
		kustomizeGVR := e2e.GetGVR("kustomize.toolkit.fluxcd.io", "v1beta1", "kustomizations")
		By(fmt.Sprintf("Creating Kustomize resource in \"%s\"", nspace))
		_, err = e2eClient.CreateNamespacedResourceYaml(kustomizeGVR, nspace, "", test.ResourceRaw)

		if test.MustSucceed {
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
		}

		//CleanUp Resources
		e2eClient.CleanClusterPolicies(policyGVR)

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
			return fmt.Errorf("failed to delete namespace: %v", err)
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

		By("Wait Till Creation of Namespace...")
		e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetClusteredResource(namespaceGVR, test.ResourceNamespace)
			if err != nil {
				return nil
			}
			return fmt.Errorf("failed to delete namespace: %v", err)
		})

		// Do not fail if waiting fails. Sometimes namespace needs time to be deleted.

		By("Done")
	}
}
