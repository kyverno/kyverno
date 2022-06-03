package generate

import (
	"time"

	"github.com/kyverno/kyverno/test/e2e"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

type resource struct {
	gvr schema.GroupVersionResource
	ns  string
	raw []byte
}

func clusteredResource(gvr schema.GroupVersionResource, raw []byte) resource {
	return resource{gvr, "", raw}
}

func namespacedResource(gvr schema.GroupVersionResource, ns string, raw []byte) resource {
	return resource{gvr, ns, raw}
}

type existingResource struct {
	gvr  schema.GroupVersionResource
	ns   string
	name string
}

func existing(gvr schema.GroupVersionResource, ns string, name string) existingResource {
	return existingResource{gvr, ns, name}
}

type expectedResource struct {
	existingResource
	validate []func(*unstructured.Unstructured)
}

func expected(gvr schema.GroupVersionResource, ns string, name string, validate ...func(*unstructured.Unstructured)) expectedResource {
	return expectedResource{existing(gvr, ns, name), validate}
}

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
var GenerateSynchronizeFlagTests = []struct {
	// TestName - Name of the Test
	TestName string
	// NetworkPolicyName - Name of the NetworkPolicy to be Created
	NetworkPolicyName string
	// GeneratePolicyName - Name of the Policy to be Created/Updated
	GeneratePolicyName string
	// ResourceNamespace - Namespace for which Resources are Created
	ResourceNamespace string
	// Clone - Set Clone Value
	Clone bool
	// CloneClusterRoleName
	ClonerClusterRoleName string
	// CloneClusterRoleBindingName
	ClonerClusterRoleBindingName string
	// CloneSourceRoleData - Source ClusterRole Name from which ClusterRole is Cloned
	CloneSourceClusterRoleData []byte
	// CloneSourceRoleBindingData - Source ClusterRoleBinding Name from which ClusterRoleBinding is Cloned
	CloneSourceClusterRoleBindingData []byte
	// CloneNamespace - Namespace where Roles are Cloned
	CloneNamespace string
	// Sync - Set Synchronize
	Sync bool
	// Data - The Yaml file of the ClusterPolicy of the ClusterRole and ClusterRoleBinding - ([]byte{})
	Data []byte
	// Data - The Yaml file of the ClusterPolicy of the ClusterRole and ClusterRoleBinding - ([]byte{})
	UpdateData []byte
}{
	{
		TestName:           "test-generate-policy-for-namespace-with-label",
		NetworkPolicyName:  "allow-dns",
		GeneratePolicyName: "add-networkpolicy",
		ResourceNamespace:  "test",
		Clone:              false,
		Sync:               true,
		Data:               genNetworkPolicyYaml,
		UpdateData:         updateSynchronizeInGeneratePolicyYaml,
	},
}

// ClusterRoleTests - E2E Test Config for ClusterRole and ClusterRoleBinding
var SourceResourceUpdateReplicationTests = []struct {
	// TestName - Name of the Test
	TestName string
	// ClusterRoleName - Name of the ClusterRole to be Created
	ResourceNamespace string
	// Clone - Set Clone Value
	Clone bool
	// CloneNamespace - Namespace where Roles are Cloned
	CloneNamespace string
	// Sync - Set Synchronize
	Sync bool
	// Data - The Yaml file of the ClusterPolicy - ([]byte{})
	Data []byte
	// ConfigMapName - name of configMap
	ConfigMapName string
	// CloneSourceConfigMapData - Source ConfigMap Yaml
	CloneSourceConfigMapData []byte
	// PolicyName - Name of the Policy
	PolicyName string
}{
	{
		TestName:                 "test-clone-source-resource-update-replication",
		ResourceNamespace:        "test",
		Clone:                    true,
		Sync:                     true,
		Data:                     genCloneConfigMapPolicyYaml,
		ConfigMapName:            "game-demo",
		CloneNamespace:           "default",
		CloneSourceConfigMapData: cloneSourceResource,
		PolicyName:               "generate-policy",
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
