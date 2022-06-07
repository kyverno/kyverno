package generate

import (
	"time"

	"github.com/kyverno/kyverno/test/e2e"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/gomega"
)

var (
	// Cluster Policy GVR
	clPolGVR = e2e.GetGVR("kyverno.io", "v1", "clusterpolicies")

	// Namespace GVR
	nsGVR = e2e.GetGVR("", "v1", "namespaces")

	// ClusterRole GVR
	crGVR = e2e.GetGVR("rbac.authorization.k8s.io", "v1", "clusterroles")

	// ClusterRoleBinding GVR
	crbGVR = e2e.GetGVR("rbac.authorization.k8s.io", "v1", "clusterrolebindings")

	// Role GVR
	rGVR = e2e.GetGVR("rbac.authorization.k8s.io", "v1", "roles")

	// RoleBinding GVR
	rbGVR = e2e.GetGVR("rbac.authorization.k8s.io", "v1", "rolebindings")

	// ConfigMap GVR
	cmGVR = e2e.GetGVR("", "v1", "configmaps")

	// NetworkPolicy GVR
	npGVR = e2e.GetGVR("networking.k8s.io", "v1", "networkpolicies")

	// ClusterPolicy Namespace
	clPolNS = ""

	// NetworkPolicy Namespace
	npPolNS = ""
)

type testCase struct {
	// TestName - Name of the Test
	TestName string
	// ClusterPolicy - ClusterPolicy yaml file
	ClusterPolicy resource
	// SourceResources - Source resources yaml files
	SourceResources []resource
	// TriggerResource - Trigger resource yaml files
	TriggerResource resource
	// ExpectedResources - Expected resources to pass the test
	ExpectedResources []expectedResource
	// Steps - Test case steps
	Steps []testCaseStep
}

// RoleTests is E2E Test Config for Role and RoleBinding
// TODO:- Clone for Role and RoleBinding
var RoleTests = []testCase{
	{
		TestName:        "test-role-rolebinding-without-clone",
		ClusterPolicy:   clusteredResource(clPolGVR, roleRoleBindingYamlWithSync),
		TriggerResource: clusteredResource(nsGVR, namespaceYaml),
		ExpectedResources: []expectedResource{
			expected(rGVR, "test", "ns-role"),
			expected(rbGVR, "test", "ns-role-binding"),
		},
	},
	{
		TestName:        "test-role-rolebinding-withsync-without-clone",
		ClusterPolicy:   clusteredResource(clPolGVR, roleRoleBindingYamlWithSync),
		TriggerResource: clusteredResource(nsGVR, namespaceYaml),
		ExpectedResources: []expectedResource{
			expected(rGVR, "test", "ns-role"),
			expected(rbGVR, "test", "ns-role-binding"),
		},
	},
	{
		TestName:      "test-role-rolebinding-with-clone",
		ClusterPolicy: clusteredResource(clPolGVR, roleRoleBindingYamlWithClone),
		SourceResources: []resource{
			namespacedResource(rGVR, "default", sourceRoleYaml),
			namespacedResource(rbGVR, "default", sourceRoleBindingYaml),
		},
		TriggerResource: clusteredResource(nsGVR, namespaceYaml),
		ExpectedResources: []expectedResource{
			expected(rGVR, "test", "ns-role"),
			expected(rbGVR, "test", "ns-role-binding"),
		},
	},
}

// ClusterRoleTests - E2E Test Config for ClusterRole and ClusterRoleBinding
var ClusterRoleTests = []testCase{
	{
		TestName:        "test-clusterrole-clusterrolebinding-without-clone",
		ClusterPolicy:   clusteredResource(clPolGVR, genClusterRoleYamlWithSync),
		TriggerResource: clusteredResource(nsGVR, namespaceYaml),
		ExpectedResources: []expectedResource{
			expected(crGVR, "", "ns-cluster-role"),
			expected(crbGVR, "", "ns-cluster-role-binding"),
		},
	},
	{
		TestName:        "test-clusterrole-clusterrolebinding-with-sync-without-clone",
		ClusterPolicy:   clusteredResource(clPolGVR, genClusterRoleYamlWithSync),
		TriggerResource: clusteredResource(nsGVR, namespaceYaml),
		ExpectedResources: []expectedResource{
			expected(crGVR, "", "ns-cluster-role"),
			expected(crbGVR, "", "ns-cluster-role-binding"),
		},
	},
	{
		TestName:      "test-clusterrole-clusterrolebinding-with-sync-with-clone",
		ClusterPolicy: clusteredResource(clPolGVR, clusterRoleRoleBindingYamlWithClone),
		SourceResources: []resource{
			clusteredResource(crGVR, baseClusterRoleData),
			clusteredResource(crbGVR, baseClusterRoleBindingData),
		},
		TriggerResource: clusteredResource(nsGVR, namespaceYaml),
		ExpectedResources: []expectedResource{
			expected(crGVR, "", "cloned-cluster-role"),
			expected(crbGVR, "", "cloned-cluster-role-binding"),
		},
	},
}

// NetworkPolicyGenerateTests - E2E Test Config for NetworkPolicyGenerateTests
var NetworkPolicyGenerateTests = []testCase{
	{
		TestName:        "test-generate-policy-for-namespace-with-label",
		ClusterPolicy:   clusteredResource(clPolGVR, genNetworkPolicyYaml),
		TriggerResource: clusteredResource(nsGVR, namespaceWithLabelYaml),
		ExpectedResources: []expectedResource{
			expected(npGVR, "test", "allow-dns"),
		},
	},
}

// NetworkPolicyGenerateTests - E2E Test Config for NetworkPolicyGenerateTests
var GenerateNetworkPolicyOnNamespaceWithoutLabelTests = []testCase{
	{
		TestName:        "test-generate-policy-for-namespace-label-actions",
		ClusterPolicy:   clusteredResource(clPolGVR, genNetworkPolicyYaml),
		TriggerResource: clusteredResource(nsGVR, namespaceYaml),
		ExpectedResources: []expectedResource{
			expected(npGVR, "test", "allow-dns"),
		},
		Steps: []testCaseStep{
			stepResourceNotFound(npGVR, "test", "allow-dns"),
			stepUpateResource(nsGVR, "", "test", func(resource *unstructured.Unstructured) error {
				element, _, err := unstructured.NestedMap(resource.UnstructuredContent(), "metadata", "labels")
				if err != nil {
					return err
				}
				element["security"] = "standard"
				return unstructured.SetNestedMap(resource.UnstructuredContent(), element, "metadata", "labels")
			}),
			stepExpectResource(npGVR, "test", "allow-dns"),
			stepUpateResource(clPolGVR, "", "add-networkpolicy", func(resource *unstructured.Unstructured) error {
				return yaml.Unmarshal(updatGenNetworkPolicyYaml, resource)
			}),
			stepExpectResource(npGVR, "test", "allow-dns", func(resource *unstructured.Unstructured) {
				// TODO: this sucks
				time.Sleep(time.Second * 50)
			}),
			stepExpectResource(npGVR, "test", "allow-dns", func(resource *unstructured.Unstructured) {
				element, found, err := unstructured.NestedMap(resource.UnstructuredContent(), "spec")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(loopElement(false, element)).To(BeTrue())
			}),
		},
	},
}

// NetworkPolicyGenerateTests - E2E Test Config for NetworkPolicyGenerateTests
var GenerateSynchronizeFlagTests = []testCase{
	{
		TestName:        "test-generate-policy-for-namespace-with-label",
		ClusterPolicy:   clusteredResource(clPolGVR, genNetworkPolicyYaml),
		TriggerResource: clusteredResource(nsGVR, namespaceWithLabelYaml),
		ExpectedResources: []expectedResource{
			expected(npGVR, "test", "allow-dns"),
		},
		Steps: []testCaseStep{
			// Test: when synchronize flag is set to true in the policy and someone deletes the generated resource, kyverno generates back the resource
			stepDeleteResource(npGVR, "test", "allow-dns"),
			stepExpectResource(npGVR, "test", "allow-dns"),
			// Test: change synchronize to false in the policy, the label in generated resource should be updated to policy.kyverno.io/synchronize: disable
			stepUpateResource(clPolGVR, "", "add-networkpolicy", func(resource *unstructured.Unstructured) error {
				return yaml.Unmarshal(updateSynchronizeInGeneratePolicyYaml, resource)
			}),
			stepExpectResource(npGVR, "test", "allow-dns", func(resource *unstructured.Unstructured) {
				// TODO: this sucks
				time.Sleep(time.Second * 30)
			}),
			stepExpectResource(npGVR, "test", "allow-dns", func(resource *unstructured.Unstructured) {
				labels := resource.GetLabels()
				Expect(labels["policy.kyverno.io/synchronize"]).To(Equal("disable"))
			}),
			// Test: with synchronize is false, one should be able to delete the generated resource
			stepDeleteResource(npGVR, "test", "allow-dns"),
			stepResourceNotFound(npGVR, "test", "allow-dns"),
		},
	},
}

// ClusterRoleTests - E2E Test Config for ClusterRole and ClusterRoleBinding
var SourceResourceUpdateReplicationTests = []testCase{
	{
		TestName:      "test-clone-source-resource-update-replication",
		ClusterPolicy: clusteredResource(clPolGVR, genCloneConfigMapPolicyYaml),
		SourceResources: []resource{
			namespacedResource(cmGVR, "default", cloneSourceResource),
		},
		TriggerResource: clusteredResource(nsGVR, namespaceYaml),
		ExpectedResources: []expectedResource{
			expected(cmGVR, "test", "game-demo"),
		},
		Steps: []testCaseStep{
			// Test: when a source clone resource is updated, the same changes should be replicated in the generated resource
			stepUpateResource(cmGVR, "default", "game-demo", func(resource *unstructured.Unstructured) error {
				element, _, err := unstructured.NestedMap(resource.UnstructuredContent(), "data")
				if err != nil {
					return err
				}
				element["initial_lives"] = "5"
				return unstructured.SetNestedMap(resource.UnstructuredContent(), element, "data")
			}),
			stepExpectResource(cmGVR, "test", "game-demo", func(resource *unstructured.Unstructured) {
				// TODO: this sucks
				time.Sleep(time.Second * 15)
			}),
			stepExpectResource(cmGVR, "test", "game-demo", func(resource *unstructured.Unstructured) {
				element, _, err := unstructured.NestedMap(resource.UnstructuredContent(), "data")
				Expect(err).NotTo(HaveOccurred())
				Expect(element["initial_lives"]).To(Equal("5"))
			}),
			// Test: when a generated resource is edited with some conflicting changes (with respect to the clone source resource or generate data), kyverno will regenerate the resource
			stepUpateResource(cmGVR, "test", "game-demo", func(resource *unstructured.Unstructured) error {
				element, _, err := unstructured.NestedMap(resource.UnstructuredContent(), "data")
				if err != nil {
					return err
				}
				element["initial_lives"] = "15"
				return unstructured.SetNestedMap(resource.UnstructuredContent(), element, "data")
			}),
			stepExpectResource(cmGVR, "test", "game-demo", func(resource *unstructured.Unstructured) {
				// TODO: this sucks
				time.Sleep(time.Second * 50)
			}),
			stepExpectResource(cmGVR, "test", "game-demo", func(resource *unstructured.Unstructured) {
				element, _, err := unstructured.NestedMap(resource.UnstructuredContent(), "data")
				Expect(err).NotTo(HaveOccurred())
				Expect(element["initial_lives"]).To(Equal("5"))
			}),
		},
	},
}

var GeneratePolicyDeletionforCloneTests = []testCase{
	{
		TestName:      "test-clone-source-resource-update-replication",
		ClusterPolicy: clusteredResource(clPolGVR, genCloneConfigMapPolicyYaml),
		SourceResources: []resource{
			namespacedResource(cmGVR, "default", cloneSourceResource),
		},
		TriggerResource: clusteredResource(nsGVR, namespaceYaml),
		ExpectedResources: []expectedResource{
			expected(cmGVR, "test", "game-demo"),
		},
		Steps: []testCaseStep{
			// delete policy -> generated resource still exists
			stepDeleteResource(clPolGVR, "", "generate-policy"),
			stepExpectResource(cmGVR, "test", "game-demo"),
			// update source -> generated resource not updated
			stepUpateResource(cmGVR, "default", "game-demo", func(resource *unstructured.Unstructured) error {
				element, _, err := unstructured.NestedMap(resource.UnstructuredContent(), "data")
				if err != nil {
					return err
				}
				element["initial_lives"] = "5"
				return unstructured.SetNestedMap(resource.UnstructuredContent(), element, "data")
			}),
			stepExpectResource(cmGVR, "test", "game-demo", func(resource *unstructured.Unstructured) {
				element, _, err := unstructured.NestedMap(resource.UnstructuredContent(), "data")
				Expect(err).NotTo(HaveOccurred())
				Expect(element["initial_lives"]).To(Equal("2"))
			}),
			// deleted source -> generated resource not deleted
			stepDeleteResource(cmGVR, "default", "game-demo"),
			stepExpectResource(cmGVR, "test", "game-demo"),
		},
	},
}
