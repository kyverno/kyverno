package e2e

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
	// Cluster Policy GVR
	clPolGVR = e2e.GetGVR("kyverno.io", "v1", "clusterpolicies")
	// disallow_cri_sock_mount Policy GVR
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

		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace : %s", nspace))
		e2eClient.DeleteClusteredResource(nsGVR, nspace)
		e2eClient.DeleteNamespacedResource(dcsmPolGVR, nspace, resource.testResourceName)

		e2e.GetWithRetry(time.Duration(1), 15, func() error { // Wait Till Deletion of Namespace
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
		e2e.GetWithRetry(time.Duration(1), 15, func() error { // Wait Till Creation of Namespace
			_, err := e2eClient.GetClusteredResource(nsGVR, resource.namespace)
			if err != nil {
				return err
			}
			return nil
		})

		// ====== Litmus Chaos Experiment ==================
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

		e2e.GetWithRetry(time.Duration(1), 15, func() error { // Wait Till preparing Chaos engine
			_, err := e2eClient.GetNamespacedResource(ceGVR, nspace, "kind-chaos")
			if err = litmuschaos_experiment_verdict.Status.experimentStatus.verdict("pass"); err != nil {
				return nil
			}

			//return errors.New("Creating Chaos Engine ")
		})

		// Create disallow_cri_sock_mount policy; kind: ClusterPolicy
		By(fmt.Sprintf("\nCreating Enforce Policy in %s", clPolNS))
		_, err = e2eClient.CreateNamespacedResourceYaml(clPolGVR, clPolNS, DisallowAddingCapabilitiesYaml)
		Expect(err).NotTo(HaveOccurred())

		// Deploy disallow_cri_sock_mount policy
		By(fmt.Sprintf("\nDeploying Enforce Policy in %s", nspace))
		_, err = e2eClient.CreateNamespacedResourceYaml(dcsmPolGVR, nspace, resource.manifest)
		Expect(err).NotTo(HaveOccurred())

		// Check pod responce

		//CleanUp Resources
		e2eClient.CleanClusterPolicies(saGVR)
		e2eClient.DeleteClusteredResource(nsGVR, nspace)      // Clear Namespace
		e2e.GetWithRetry(time.Duration(1), 15, func() error { // Wait Till Deletion of Namespace
			_, err := e2eClient.GetClusteredResource(nsGVR, nspace)
			if err != nil {
				return nil
			}
			return errors.New("Deleting Namespace")
		})

		By(fmt.Sprintf("Test %s Completed. \n\n\n", PodCPUHogTest.TestName))
	}

}
