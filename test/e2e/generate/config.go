package generate

import (
	"time"

	"github.com/kyverno/kyverno/test/e2e"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
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

	// Secret GVR
	secretGVR = e2e.GetGVR("", "v1", "secrets")

	// NetworkPolicy GVR
	npGVR = e2e.GetGVR("networking.k8s.io", "v1", "networkpolicies")
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

// roleTests is E2E Test Config for Role and RoleBinding
// TODO:- Clone for Role and RoleBinding
var roleTests = []testCase{
	{
		TestName:        "test-role-rolebinding-without-clone",
		ClusterPolicy:   clusterPolicy(roleRoleBindingYamlWithSync),
		TriggerResource: namespace(namespaceYaml),
		ExpectedResources: expectations(
			expectation(idRole("test", "ns-role")),
			expectation(idRoleBinding("test", "ns-role-binding")),
		),
	},
	{
		TestName:        "test-role-rolebinding-withsync-without-clone",
		ClusterPolicy:   clusterPolicy(roleRoleBindingYamlWithSync),
		TriggerResource: namespace(namespaceYaml),
		ExpectedResources: expectations(
			expectation(idRole("test", "ns-role")),
			expectation(idRoleBinding("test", "ns-role-binding")),
		),
	},
	{
		TestName:      "test-role-rolebinding-with-clone",
		ClusterPolicy: clusterPolicy(roleRoleBindingYamlWithClone),
		SourceResources: resources(
			role("default", sourceRoleYaml),
			roleBinding("default", sourceRoleBindingYaml),
		),
		TriggerResource: namespace(namespaceYaml),
		ExpectedResources: expectations(
			expectation(idRole("test", "ns-role")),
			expectation(idRoleBinding("test", "ns-role-binding")),
		),
	},
}

// clusterRoleTests - E2E Test Config for ClusterRole and ClusterRoleBinding
var clusterRoleTests = []testCase{
	{
		TestName:        "test-clusterrole-clusterrolebinding-without-clone",
		ClusterPolicy:   clusterPolicy(genClusterRoleYamlWithSync),
		TriggerResource: namespace(namespaceYaml),
		ExpectedResources: expectations(
			expectation(idClusterRole("ns-cluster-role")),
			expectation(idClusterRoleBinding("ns-cluster-role-binding")),
		),
	},
	{
		TestName:        "test-clusterrole-clusterrolebinding-with-sync-without-clone",
		ClusterPolicy:   clusterPolicy(genClusterRoleYamlWithSync),
		TriggerResource: namespace(namespaceYaml),
		ExpectedResources: expectations(
			expectation(idClusterRole("ns-cluster-role")),
			expectation(idClusterRoleBinding("ns-cluster-role-binding")),
		),
	},
	{
		TestName:      "test-clusterrole-clusterrolebinding-with-sync-with-clone",
		ClusterPolicy: clusterPolicy(clusterRoleRoleBindingYamlWithClone),
		SourceResources: resources(
			clusterRole(baseClusterRoleData),
			clusterRoleBinding(baseClusterRoleBindingData),
		),
		TriggerResource: namespace(namespaceYaml),
		ExpectedResources: expectations(
			expectation(idClusterRole("cloned-cluster-role")),
			expectation(idClusterRoleBinding("cloned-cluster-role-binding")),
		),
	},
}

// networkPolicyGenerateTests - E2E Test Config for networkPolicyGenerateTests
var networkPolicyGenerateTests = []testCase{
	{
		TestName:        "test-generate-policy-for-namespace-with-label",
		ClusterPolicy:   clusterPolicy(genNetworkPolicyYaml),
		TriggerResource: namespace(namespaceWithLabelYaml),
		ExpectedResources: expectations(
			expectation(idNetworkPolicy("test", "allow-dns")),
		),
	},
}

var generateNetworkPolicyOnNamespaceWithoutLabelTests = []testCase{
	{
		TestName:        "test-generate-policy-for-namespace-label-actions",
		ClusterPolicy:   clusterPolicy(genNetworkPolicyYaml),
		TriggerResource: namespace(namespaceYaml),
		ExpectedResources: expectations(
			expectation(idNetworkPolicy("test", "allow-dns")),
		),
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
			stepWaitResource(npGVR, "test", "allow-dns", time.Second, 30, func(resource *unstructured.Unstructured) bool {
				element, found, err := unstructured.NestedMap(resource.UnstructuredContent(), "spec")
				if err != nil || !found {
					return false
				}
				return loopElement(false, element)
			}),
		},
	},
}

// NetworkPolicyGenerateTests - E2E Test Config for NetworkPolicyGenerateTests
var generateSynchronizeFlagTests = []testCase{
	{
		TestName:        "test-generate-policy-for-namespace-with-label",
		ClusterPolicy:   clusterPolicy(genNetworkPolicyYaml),
		TriggerResource: namespace(namespaceWithLabelYaml),
		// expectation is resource should no longer exists once deleted
		// if sync is set to false

		Steps: []testCaseStep{
			stepBy("When synchronize flag is set to true in the policy and someone deletes the generated resource, kyverno generates back the resource"),
			stepDeleteResource(npGVR, "test", "allow-dns"),
			stepExpectResource(npGVR, "test", "allow-dns"),
			stepBy("Change synchronize to false in the policy, the label in generated resource should be updated to policy.kyverno.io/synchronize: disable"),
			stepUpateResource(clPolGVR, "", "add-networkpolicy", func(resource *unstructured.Unstructured) error {
				return yaml.Unmarshal(updateSynchronizeInGeneratePolicyYaml, resource)
			}),
			stepWaitResource(npGVR, "test", "allow-dns", time.Second, 30, func(resource *unstructured.Unstructured) bool {
				labels := resource.GetLabels()
				return labels["policy.kyverno.io/synchronize"] == "disable"
			}),
			stepBy("With synchronize is false, one should be able to delete the generated resource"),
			stepDeleteResource(npGVR, "test", "allow-dns"),
			stepResourceNotFound(npGVR, "test", "allow-dns"),
		},
	},
}

// ClusterRoleTests - E2E Test Config for ClusterRole and ClusterRoleBinding
var sourceResourceUpdateReplicationTests = []testCase{
	{
		TestName:      "test-clone-source-resource-update-replication",
		ClusterPolicy: clusterPolicy(genCloneConfigMapPolicyYaml),
		SourceResources: resources(
			configMap("default", cloneSourceResource),
		),
		TriggerResource: namespace(namespaceYaml),
		ExpectedResources: expectations(
			expectation(idConfigMap("test", "game-demo")),
		),
		Steps: []testCaseStep{
			stepBy("When a source clone resource is updated, the same changes should be replicated in the generated resource"),
			stepUpateResource(cmGVR, "default", "game-demo", func(resource *unstructured.Unstructured) error {
				element, _, err := unstructured.NestedMap(resource.UnstructuredContent(), "data")
				if err != nil {
					return err
				}
				element["initial_lives"] = "5"
				return unstructured.SetNestedMap(resource.UnstructuredContent(), element, "data")
			}),
			stepWaitResource(cmGVR, "test", "game-demo", time.Second, 15, func(resource *unstructured.Unstructured) bool {
				element, found, err := unstructured.NestedMap(resource.UnstructuredContent(), "data")
				if err != nil || !found {
					return false
				}
				return element["initial_lives"] == "5"
			}),
			stepBy("When a generated resource is edited with some conflicting changes (with respect to the clone source resource or generate data), kyverno will regenerate the resource"),
			stepUpateResource(cmGVR, "test", "game-demo", func(resource *unstructured.Unstructured) error {
				element, _, err := unstructured.NestedMap(resource.UnstructuredContent(), "data")
				if err != nil {
					return err
				}
				element["initial_lives"] = "15"
				return unstructured.SetNestedMap(resource.UnstructuredContent(), element, "data")
			}),
			stepWaitResource(cmGVR, "test", "game-demo", time.Second, 30, func(resource *unstructured.Unstructured) bool {
				element, found, err := unstructured.NestedMap(resource.UnstructuredContent(), "data")
				if err != nil || !found {
					return false
				}
				return element["initial_lives"] == "5"
			}),
		},
	},
}

var generatePolicyDeletionforCloneTests = []testCase{
	{
		TestName:      "test-clone-source-resource-update-replication",
		ClusterPolicy: clusterPolicy(genCloneConfigMapPolicyYaml),
		SourceResources: resources(
			configMap("default", cloneSourceResource),
		),
		TriggerResource: namespace(namespaceYaml),
		ExpectedResources: expectations(
			expectation(idConfigMap("test", "game-demo")),
		),
		Steps: []testCaseStep{
			stepBy("delete policy -> generated resource still exists"),
			stepDeleteResource(clPolGVR, "", "generate-policy"),
			stepExpectResource(cmGVR, "test", "game-demo"),
			stepBy("update source -> generated resource not updated"),
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
			stepBy("deleted source -> generated resource not deleted"),
			stepDeleteResource(cmGVR, "default", "game-demo"),
			stepExpectResource(cmGVR, "test", "game-demo"),
		},
	},
}

var generatePolicyMultipleCloneTests = []testCase{
	{
		TestName:      "test-multiple-clone-resources",
		ClusterPolicy: clusterPolicy(genMultipleClonePolicyYaml),
		SourceResources: resources(
			configMap("default", cloneSourceResource),
			secret("default", cloneSecretSourceResource),
		),
		TriggerResource: namespace(namespaceYaml),
		ExpectedResources: expectations(
			expectation(idConfigMap("test", "game-demo")),
			expectation(idSecret("test", "secret-basic-auth")),
		),
		Steps: []testCaseStep{
			stepExpectResource(cmGVR, "test", "game-demo"),
			stepBy("verify generated resource data in configMap"),
			stepExpectResource(cmGVR, "test", "game-demo", func(resource *unstructured.Unstructured) {
				element, _, err := unstructured.NestedMap(resource.UnstructuredContent(), "data")
				Expect(err).NotTo(HaveOccurred())
				Expect(element["initial_lives"]).To(Equal("2"))
			}),

			stepBy("verify generated resource data in secret"),
			stepExpectResource(secretGVR, "test", "secret-basic-auth"),

			stepBy("deleted source -> generated resource not deleted"),
			stepDeleteResource(cmGVR, "default", "game-demo"),
			stepDeleteResource(secretGVR, "default", "secret-basic-auth"),
			stepExpectResource(cmGVR, "test", "game-demo"),
			stepExpectResource(secretGVR, "test", "secret-basic-auth"),
		},
	},
}
