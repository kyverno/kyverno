package mutate

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kyverno/kyverno/pkg/engine/validate"
	"github.com/kyverno/kyverno/test/e2e"
	commonE2E "github.com/kyverno/kyverno/test/e2e/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	// Cluster Policy GVR
	policyGVR = e2e.GetGVR("kyverno.io", "v1", "clusterpolicies")
	// Namespace GVR
	namespaceGVR = e2e.GetGVR("", "v1", "namespaces")
	// ConfigMap GVR
	configMapGVR = e2e.GetGVR("", "v1", "configmaps")

	// ClusterPolicy Namespace
	policyNamespace = ""
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
		e2eClient.CleanClusterPolicies(policyGVR)

		// Clear Namespace
		By(fmt.Sprintf("Deleting Namespace : %s", tests.ResourceNamespace))
		e2eClient.DeleteClusteredResource(namespaceGVR, tests.ResourceNamespace)

		// Wait Till Deletion of Namespace
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetClusteredResource(namespaceGVR, tests.ResourceNamespace)
			if err != nil {
				return nil
			}
			return fmt.Errorf("failed to delete namespace: %v", err)
		})
		Expect(err).NotTo(HaveOccurred())

		// Create Namespace
		By(fmt.Sprintf("Creating Namespace %s", policyNamespace))
		_, err = e2eClient.CreateClusteredResourceYaml(namespaceGVR, newNamespaceYaml("test-mutate"))
		Expect(err).NotTo(HaveOccurred())

		// Wait Till Creation of Namespace
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetClusteredResource(namespaceGVR, tests.ResourceNamespace)
			if err != nil {
				return err
			}

			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// Create source CM
		By(fmt.Sprintf("\nCreating source ConfigMap in %s", tests.ResourceNamespace))
		_, err = e2eClient.CreateNamespacedResourceYaml(configMapGVR, tests.ResourceNamespace, sourceConfigMapYaml)
		Expect(err).NotTo(HaveOccurred())

		// Create CM Policy
		By(fmt.Sprintf("\nCreating Mutate ConfigMap Policy in %s", policyNamespace))
		_, err = e2eClient.CreateNamespacedResourceYaml(policyGVR, policyNamespace, tests.Data)
		Expect(err).NotTo(HaveOccurred())

		err = commonE2E.PolicyCreated(tests.PolicyName)
		Expect(err).NotTo(HaveOccurred())

		// Create target CM
		By(fmt.Sprintf("\nCreating target ConfigMap in %s", tests.ResourceNamespace))
		_, err = e2eClient.CreateNamespacedResourceYaml(configMapGVR, tests.ResourceNamespace, targetConfigMapYaml)
		Expect(err).NotTo(HaveOccurred())

		// Verify created ConfigMap
		By(fmt.Sprintf("Verifying ConfigMap in the Namespace : %s", tests.ResourceNamespace))
		// Wait Till Creation of ConfigMap
		e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetNamespacedResource(configMapGVR, tests.ResourceNamespace, "target")
			if err != nil {
				return err
			}

			return nil
		})

		cmRes, err := e2eClient.GetNamespacedResource(configMapGVR, tests.ResourceNamespace, "target")
		c, _ := json.Marshal(cmRes)
		By(fmt.Sprintf("configMap : %s", string(c)))

		Expect(err).NotTo(HaveOccurred())
		Expect(cmRes.GetLabels()["kyverno.key/copy-me"]).To(Equal("sample-value"))

		//CleanUp Resources
		e2eClient.CleanClusterPolicies(policyGVR)

		// Clear Namespace
		e2eClient.DeleteClusteredResource(namespaceGVR, tests.ResourceNamespace)

		// Wait Till Deletion of Namespace
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetClusteredResource(namespaceGVR, tests.ResourceNamespace)
			if err != nil {
				return nil
			}
			return fmt.Errorf("failed to delete namespace: %v", err)
		})
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Test %s Completed \n\n\n", tests.TestName))
	}
}

func Test_Mutate(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}

	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())

	for _, test := range tests {
		By(fmt.Sprintf("Mutation Test: %s", test.TestDescription))

		By("Deleting Cluster Policies...")
		e2eClient.CleanClusterPolicies(policyGVR)

		By("Deleting Resource...")
		e2eClient.DeleteNamespacedResource(test.ResourceGVR, test.ResourceNamespace, test.ResourceName)

		By("Deleting Namespace...")
		By(fmt.Sprintf("Deleting Namespace: %s...", test.ResourceNamespace))
		e2eClient.DeleteClusteredResource(namespaceGVR, test.ResourceNamespace)

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
		_, err = e2eClient.CreateNamespacedResourceYaml(policyGVR, policyNamespace, test.PolicyRaw)
		Expect(err).NotTo(HaveOccurred())

		err = commonE2E.PolicyCreated(test.PolicyName)
		Expect(err).NotTo(HaveOccurred())

		By("Creating Resource...")
		_, err = e2eClient.CreateNamespacedResourceYaml(test.ResourceGVR, test.ResourceNamespace, test.ResourceRaw)
		Expect(err).NotTo(HaveOccurred())

		By("Checking that resource is created...")
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			_, err := e2eClient.GetNamespacedResource(test.ResourceGVR, test.ResourceNamespace, test.ResourceName)
			if err != nil {
				return err
			}

			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		res, err := e2eClient.GetNamespacedResource(test.ResourceGVR, test.ResourceNamespace, test.ResourceName)
		Expect(err).NotTo(HaveOccurred())

		actualJSON, err := json.Marshal(res)
		Expect(err).NotTo(HaveOccurred())

		var actual interface{}

		err = json.Unmarshal(actualJSON, &actual)
		Expect(err).NotTo(HaveOccurred())

		expected, err := rawYAMLToJSONInterface(test.ExpectedPatternRaw)
		Expect(err).NotTo(HaveOccurred())

		By("Validating created resource with the expected pattern...")
		err = validate.MatchPattern(log.Log, actual, expected)
		Expect(err).NotTo(HaveOccurred())

		By("Deleting Cluster Policies...")
		err = e2eClient.CleanClusterPolicies(policyGVR)
		Expect(err).NotTo(HaveOccurred())

		By("Deleting Resource...")
		err = e2eClient.DeleteNamespacedResource(test.ResourceGVR, test.ResourceNamespace, test.ResourceName)
		Expect(err).NotTo(HaveOccurred())

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

func Test_Mutate_Ingress(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}

	// Generate E2E Client
	e2eClient, err := e2e.NewE2EClient()
	Expect(err).To(BeNil())

	nspace := ingressTests.testNamespace
	By("Cleaning Cluster Policies")
	e2eClient.CleanClusterPolicies(policyGVR)

	By(fmt.Sprintf("Deleting Namespace : %s", nspace))
	e2eClient.DeleteClusteredResource(namespaceGVR, nspace)

	// Wait Till Deletion of Namespace
	err = e2e.GetWithRetry(1*time.Second, 15, func() error {
		_, err := e2eClient.GetClusteredResource(namespaceGVR, nspace)
		if err != nil {
			return nil
		}
		return fmt.Errorf("failed to delete namespace: %v", err)
	})
	Expect(err).To(BeNil())

	By("Creating mutate ClusterPolicy")
	_, err = e2eClient.CreateClusteredResourceYaml(policyGVR, ingressTests.cpol)
	Expect(err).NotTo(HaveOccurred())

	err = commonE2E.PolicyCreated(ingressTests.policyName)
	Expect(err).NotTo(HaveOccurred())

	By(fmt.Sprintf("Creating Namespace %s", nspace))
	_, err = e2eClient.CreateClusteredResourceYaml(namespaceGVR, newNamespaceYaml(nspace))
	Expect(err).NotTo(HaveOccurred())

	for _, test := range ingressTests.tests {
		if test.skip {
			continue
		}
		By(fmt.Sprintf("\n\nStart testing %s", test.testName))
		gvr := e2e.GetGVR(test.group, test.version, test.rsc)
		By(fmt.Sprintf("Creating Ingress %v in %s", gvr, nspace))
		_, err = e2eClient.CreateNamespacedResourceYaml(gvr, nspace, test.resource)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Verifying Ingress %v in the Namespace : %s", gvr, nspace))
		var mutatedResource *unstructured.Unstructured
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			mutatedResource, err = e2eClient.GetNamespacedResource(gvr, nspace, test.resourceName)
			if err != nil {
				return err
			}
			return nil
		})
		Expect(err).To(BeNil())

		By("Comparing patched field")
		rules, ok, err := unstructured.NestedSlice(mutatedResource.UnstructuredContent(), "spec", "rules")
		Expect(err).To(BeNil())
		Expect(ok).To(BeTrue())
		rule := rules[0].(map[string]interface{})
		host := rule["host"].(string)
		Expect(host).To(Equal("kuard.mycompany.com"))
	}
}

func rawYAMLToJSONInterface(y []byte) (interface{}, error) {
	var temp, result interface{}
	var err error

	err = UnmarshalYAML(y, &temp)
	if err != nil {
		return nil, err
	}

	jsonRaw, err := json.Marshal(temp)
	if err != nil {
		return nil, err
	}

	json.Unmarshal(jsonRaw, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// UnmarshalYAML unmarshals YAML to map[string]interface{} instead of map[interface{}]interface{}.
func UnmarshalYAML(in []byte, out interface{}) error {
	var res interface{}

	if err := yaml.Unmarshal(in, &res); err != nil {
		return err
	}
	*out.(*interface{}) = cleanupMapValue(res)

	return nil
}

func cleanupInterfaceArray(in []interface{}) []interface{} {
	res := make([]interface{}, len(in))
	for i, v := range in {
		res[i] = cleanupMapValue(v)
	}
	return res
}

func cleanupInterfaceMap(in map[interface{}]interface{}) map[string]interface{} {
	res := make(map[string]interface{})
	for k, v := range in {
		res[fmt.Sprintf("%v", k)] = cleanupMapValue(v)
	}
	return res
}

func cleanupMapValue(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		return cleanupInterfaceArray(v)
	case map[interface{}]interface{}:
		return cleanupInterfaceMap(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
