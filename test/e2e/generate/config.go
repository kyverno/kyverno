package generate

import (
	"github.com/kyverno/kyverno/test/e2e"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

type expectedResource struct {
	gvr  schema.GroupVersionResource
	ns   string
	name string
}

// RoleTests is E2E Test Config for Role and RoleBinding
// TODO:- Clone for Role and RoleBinding
var RoleTests = []struct {
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
}{
	{
		TestName:        "test-role-rolebinding-without-clone",
		ClusterPolicy:   clusteredResource(clPolGVR, roleRoleBindingYamlWithSync),
		TriggerResource: clusteredResource(nsGVR, namespaceYaml),
		ExpectedResources: []expectedResource{
			{rGVR, "test", "ns-role"},
			{rbGVR, "test", "ns-role-binding"},
		},
	},
	{
		TestName:        "test-role-rolebinding-withsync-without-clone",
		ClusterPolicy:   clusteredResource(clPolGVR, roleRoleBindingYamlWithSync),
		TriggerResource: clusteredResource(nsGVR, namespaceYaml),
		ExpectedResources: []expectedResource{
			{rGVR, "test", "ns-role"},
			{rbGVR, "test", "ns-role-binding"},
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
			{rGVR, "test", "ns-role"},
			{rbGVR, "test", "ns-role-binding"},
		},
	},
}

// ClusterRoleTests - E2E Test Config for ClusterRole and ClusterRoleBinding
var ClusterRoleTests = []struct {
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
}{
	{
		TestName:        "test-clusterrole-clusterrolebinding-without-clone",
		ClusterPolicy:   clusteredResource(clPolGVR, genClusterRoleYamlWithSync),
		TriggerResource: clusteredResource(nsGVR, namespaceYaml),
		ExpectedResources: []expectedResource{
			{crGVR, "", "ns-cluster-role"},
			{crbGVR, "", "ns-cluster-role-binding"},
		},
	},
	{
		TestName:        "test-clusterrole-clusterrolebinding-with-sync-without-clone",
		ClusterPolicy:   clusteredResource(clPolGVR, genClusterRoleYamlWithSync),
		TriggerResource: clusteredResource(nsGVR, namespaceYaml),
		ExpectedResources: []expectedResource{
			{crGVR, "", "ns-cluster-role"},
			{crbGVR, "", "ns-cluster-role-binding"},
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
			{crGVR, "", "cloned-cluster-role"},
			{crbGVR, "", "cloned-cluster-role-binding"},
		},
	},
}

// NetworkPolicyGenerateTests - E2E Test Config for NetworkPolicyGenerateTests
var NetworkPolicyGenerateTests = []struct {
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
}{
	{
		TestName:        "test-generate-policy-for-namespace-with-label",
		ClusterPolicy:   clusteredResource(clPolGVR, genNetworkPolicyYaml),
		TriggerResource: clusteredResource(nsGVR, namespaceWithLabelYaml),
		ExpectedResources: []expectedResource{
			{npGVR, "test", "allow-dns"},
		},
	},
}

// NetworkPolicyGenerateTests - E2E Test Config for NetworkPolicyGenerateTests
var GenerateNetworkPolicyOnNamespaceWithoutLabelTests = []struct {
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
		TestName:           "test-generate-policy-for-namespace-label-actions",
		ResourceNamespace:  "test",
		NetworkPolicyName:  "allow-dns",
		GeneratePolicyName: "add-networkpolicy",
		Clone:              false,
		Sync:               true,
		Data:               genNetworkPolicyYaml,
		UpdateData:         updatGenNetworkPolicyYaml,
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

var GeneratePolicyDeletionforCloneTests = []struct {
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
