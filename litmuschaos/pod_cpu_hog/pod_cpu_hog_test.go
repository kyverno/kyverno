package e2e

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kyverno/kyverno/test/e2e"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	// Namespace GVR
	nsGVR = e2e.GetGVR("", "v1", "namespaces")
	// Chaos service account GVR
	saGVR = e2e.GetGVR("", "v1", "serviceaccounts")
	// Role GVR
	rGVR = e2e.GetGVR("rbac.authorization.k8s.io", "v1", "roles")
	// RoleBinding GVR
	rbGVR = e2e.GetGVR("rbac.authorization.k8s.io", "v1", "rolebindings")
	// PodCPUHogExperiment GVR
	cpuGVR = e2e.GetGVR("litmuschaos.io", "v1alpha1", "chaosexperiments")
	// ChaosEngine GVR
	ceGVR = e2e.GetGVR("litmuschaos.io", "v1alpha1", "chaosengines")
	// Chaos Result GVR
	crGVR = e2e.GetGVR("litmuschaos.io", "v1alpha1", "chaosresults")
	// Cluster Policy GVR
	clPolGVR = e2e.GetGVR("kyverno.io", "v1", "clusterpolicies")
	// Kyverno disallow_cri_sock_mount Policy GVR
	dcsmPolGVR = e2e.GetGVR("", "v1", "pods")

	// ClusterPolicy Namespace
	clPolNS = ""
	// Namespace Name
	// Hardcoded in YAML Definition
	nspace = "test-litmus"
)

func Test_Pod_CPU_Hog(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}

	// Generate E2E Client
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())

	for _, resource := range PodCPUHogTest.TestData {

		// CleanUp Resources
		By(fmt.Sprintf("Cleaning Cluster Policies in %s", nspace))
		e2eClient.CleanClusterPolicies(clPolGVR) //Clean Cluster Policy
		By(fmt.Sprintf("Deleting Namespace : %s", nspace))
		e2eClient.DeleteClusteredResource(nsGVR, nspace) // Clear Namespace
		e2eClient.DeleteNamespacedResource(dcsmPolGVR, nspace, resource.testResourceName)
		e2e.GetWithRetry(1*time.Second, 15, func() error { // Wait Till Deletion of Namespace
			_, err := e2eClient.GetClusteredResource(nsGVR, nspace)
			if err != nil {
				return nil
			}
			return errors.New("Deleting Namespace")
		})

		// Create Namespace
		By(fmt.Sprintf("Creating Namespace %s", saGVR))
		_, err = e2eClient.CreateClusteredResourceYaml(nsGVR, LitmusChaosnamespaceYaml)
		Expect(err).NotTo(HaveOccurred())
		e2e.GetWithRetry(1*time.Second, 15, func() error { // Wait Till Creation of Namespace
			_, err := e2eClient.GetClusteredResource(nsGVR, resource.namespace)
			if err != nil {
				return err
			}
			return nil
		})

		// ================== Litmus Chaos Experiment ==================
		// Prepare chaosServiceAccount
		By(fmt.Sprintf("\nPrepareing Chaos Service Account in %s", nspace))
		_, err = e2eClient.CreateNamespacedResourceYaml(saGVR, nspace, ChaosServiceAccountYaml)
		Expect(err).NotTo(HaveOccurred())
		_, err = e2eClient.CreateNamespacedResourceYaml(rGVR, nspace, ChaosRoleYaml)
		Expect(err).NotTo(HaveOccurred())
		_, err = e2eClient.CreateNamespacedResourceYaml(rbGVR, nspace, ChaosRoleBindingYaml)
		Expect(err).NotTo(HaveOccurred())

		// Deploy Pod CPU Hog Experiment
		By(fmt.Sprintf("\nInstalling Litmus Chaos Experiment in %s", nspace))
		_, err = e2eClient.CreateNamespacedResourceYaml(cpuGVR, nspace, PodCPUHogExperimentYaml)
		Expect(err).NotTo(HaveOccurred())

		// Prepare Chaos Engine
		By(fmt.Sprintf("\nCreating ChaosEngine Resource in %s", nspace))
		_, err = e2eClient.CreateNamespacedResourceYaml(ceGVR, nspace, ChaosEngineYaml)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("\nMonitoring status from ChaosResult in %s", nspace))

		e2e.GetWithRetry(1*time.Second, 120, func() error { // Wait Till preparing Chaos engine
			chaosresult, err := e2eClient.GetNamespacedResource(crGVR, nspace, "kind-chaos-pod-cpu-hog")
			if err != nil {
				return fmt.Errorf("Unable to fatch ChaosResult: %v", err)
			}
			chaosVerdict, _, err := unstructured.NestedString(chaosresult.UnstructuredContent(), "status", "experimentStatus", "verdict")
			if err != nil {
				By(fmt.Sprintf("\nUnable to fatch the status.verdict from ChaosResult: %v", err))
			}

			By(fmt.Sprintf("\nChaos verdict %s", chaosVerdict))

			if chaosVerdict == "Pass" {
				return nil
			}
			return errors.New("Chaos result is not passed")
		})

		// Create disallow_cri_sock_mount policy
		By(fmt.Sprintf("\nCreating Enforce Policy in %s", clPolNS))
		_, err = e2eClient.CreateNamespacedResourceYaml(clPolGVR, clPolNS, DisallowAddingCapabilitiesYaml)
		Expect(err).NotTo(HaveOccurred())

		// Deploy disallow_cri_sock_mount policy
		By(fmt.Sprintf("\nDeploying Enforce Policy in %s", nspace))
		_, err = e2eClient.CreateNamespacedResourceYaml(dcsmPolGVR, nspace, resource.manifest)
		Expect(err).To(HaveOccurred())

		//CleanUp Resources
		e2eClient.CleanClusterPolicies(clPolGVR) //Clean Cluster Policy
		e2eClient.CleanClusterPolicies(saGVR)
		e2eClient.DeleteClusteredResource(nsGVR, nspace)   // Clear Namespace
		e2e.GetWithRetry(1*time.Second, 15, func() error { // Wait Till Deletion of Namespace
			_, err := e2eClient.GetClusteredResource(nsGVR, nspace)
			if err != nil {
				return nil
			}
			return errors.New("Deleting Namespace")
		})

		By(fmt.Sprintf("Test %s Completed. \n\n\n", PodCPUHogTest.TestName))
	}

}
