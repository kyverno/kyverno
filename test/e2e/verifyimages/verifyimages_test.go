package verifyimages

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kyverno/kyverno/test/e2e"
	"github.com/kyverno/kyverno/test/e2e/framework"
	"github.com/kyverno/kyverno/test/e2e/framework/id"
	"github.com/kyverno/kyverno/test/e2e/framework/step"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	// Cluster Polict GVR
	policyGVR = e2e.GetGVR("kyverno.io", "v1", "clusterpolicies")
	// Namespace GVR
	namespaceGVR = e2e.GetGVR("", "v1", "namespaces")

	crdGVR = e2e.GetGVR("apiextensions.k8s.io", "v1", "customresourcedefinitions")

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

	By(fmt.Sprintf("Deleting CRD: %s...", crdName))
	e2eClient.DeleteClusteredResource(crdGVR, crdName)

	By("Wait Till Deletion of CRD...")
	err = e2e.GetWithRetry(1*time.Second, 15, func() error {
		_, err := e2eClient.GetClusteredResource(crdGVR, crdName)
		if err != nil {
			return nil
		}

		return fmt.Errorf("failed to crd: %v", err)
	})
	Expect(err).NotTo(HaveOccurred())

	// Create Tekton CRD
	By("Creating Tekton CRD")
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
	time.Sleep(15 * time.Second)

	for _, tcase := range VerifyImagesTests {
		test := tcase
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

		By(fmt.Sprintf("Creating Namespace: %s...", test.ResourceNamespace))
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

		// Create policy
		By(fmt.Sprintf("Creating policy \"%s\"", test.PolicyName))
		err = e2e.GetWithRetry(1*time.Second, 30, func() error {
			_, err := e2eClient.CreateClusteredResourceYaml(policyGVR, test.PolicyRaw)
			if err != nil {
				return err
			}

			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(e2eClient.ClusterPolicyReady(test.PolicyName)).To(BeTrue())

		By("Creating Resource...")
		_, err := e2eClient.CreateNamespacedResourceYaml(test.ResourceGVR, test.ResourceNamespace, test.ResourceName, test.ResourceRaw)
		if test.MustSucceed {
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
		}

		// Clean up policies
		By("Deleting Cluster Policies...")
		err = e2eClient.CleanClusterPolicies(policyGVR)
		Expect(err).NotTo(HaveOccurred())

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
	//CleanUp CRDs
	e2eClient.DeleteClusteredResource(crdGVR, crdName)

}

func Test_BoolFields(t *testing.T) {
	framework.Setup(t)
	for _, field := range []string{"mutateDigest", "verifyDigest", "required"} {
		framework.RunSubTest(t, field,
			step.CreateClusterPolicy(cpolVerifyImages),
			step.By(fmt.Sprintf("Checking spec.rules[0].verifyImages[0].%s is false ...", field)),
			step.ExpectResource(id.ClusterPolicy("verify-images"), func(resource *unstructured.Unstructured) {
				rules, found, err := unstructured.NestedSlice(resource.UnstructuredContent(), "spec", "rules")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				verifyImages, found, err := unstructured.NestedSlice(rules[0].(map[string]interface{}), "verifyImages")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				mutateDigest, found, err := unstructured.NestedBool(verifyImages[0].(map[string]interface{}), field)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(mutateDigest).To(BeFalse())
			}),
		)
	}
}
