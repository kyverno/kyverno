package validate

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

	crdGVR = e2e.GetGVR("apiextensions.k8s.io", "v1", "customresourcedefinitions")

	// ClusterPolicy Namespace
	clPolNS = ""
	// Namespace Name
	// Hardcoded in YAML Definition
	nspace = "test-validate"

	crdName = "kustomizations.kustomize.toolkit.fluxcd.io"
)

func Test_Validate_Sets(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}

	// Generate E2E Client
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())

	for _, test := range ValidateTests {
		By(fmt.Sprintf("Test to validate objects: \"%s\"", test.TestName))

		// Clean up Resources
		By(fmt.Sprintf("Cleaning Cluster Policies"))
		e2eClient.CleanClusterPolicies(clPolGVR)
		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace: \"%s\"", nspace))
		e2eClient.DeleteClusteredResource(nsGVR, nspace)
		//CleanUp CRDs
		e2eClient.DeleteClusteredResource(crdGVR, crdName)

		// Wait Till Deletion of Namespace
		e2e.GetWithRetry(time.Duration(1*time.Second), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, nspace)
			if err != nil {
				return nil
			}
			return errors.New("Deleting Namespace")
		})

		// Create Namespace
		By(fmt.Sprintf("Creating namespace \"%s\"", nspace))
		_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, namespaceYaml)
		Expect(err).NotTo(HaveOccurred())

		// Create policy
		By(fmt.Sprintf("Creating policy in \"%s\"", clPolNS))
		_, err = e2eClient.CreateNamespacedResourceYaml(clPolGVR, clPolNS, test.PolicyRaw)
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
		_, err = e2eClient.CreateNamespacedResourceYaml(kustomizeGVR, nspace, test.ResourceRaw)

		if test.MustSucceed {
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
		}

		//CleanUp Resources
		e2eClient.CleanClusterPolicies(clPolGVR)

		//CleanUp CRDs
		e2eClient.DeleteClusteredResource(crdGVR, crdName)

		// Clear Namespace
		e2eClient.DeleteClusteredResource(nsGVR, nspace)
		// Wait Till Deletion of Namespace
		e2e.GetWithRetry(time.Duration(1*time.Second), 15, func() error {
			_, err := e2eClient.GetClusteredResource(nsGVR, nspace)
			if err != nil {
				return nil
			}
			return errors.New("Deleting Namespace")
		})

		By(fmt.Sprintf("Test %s Completed \n\n\n", test.TestName))
	}
}
