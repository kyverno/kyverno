package generate

// RoleTests is E2E Test Config for Role and RoleBinding
// TODO:- Clone for Role and RoleBinding
var RoleTests = []struct {
	//TestName - Name of the Test
	TestName string
	// RoleName - Name of the Role to be Created
	RoleName string
	// RoleBindingName - Name of the RoleBindingName
	RoleBindingName string
	// ResourceNamespace - Namespace for which Role and ReleBinding are Created
	ResourceNamespace string
	// Clone - Set Clone Value
	Clone bool
	// CloneSourceRoleData - Source Role Name from which Role is Cloned
	CloneSourceRoleData []byte
	// CloneSourceRoleBindingData - Source RoleBinding Name from which RoleBinding is Cloned
	CloneSourceRoleBindingData []byte
	// CloneNamespace - Namespace where Roles are Cloned
	CloneNamespace string
	// Sync - Set Synchronize
	Sync bool
	// Data - The Yaml file of the ClusterPolicy of the ROle and RoleBinding - ([]byte{})
	Data []byte
	// PolicyName - Name of the Policy
	PolicyName string
}{
	{
		TestName:          "test-role-rolebinding-without-clone",
		RoleName:          "ns-role",
		RoleBindingName:   "ns-role-binding",
		ResourceNamespace: "test",
		Clone:             false,
		Sync:              false,
		Data:              roleRoleBindingYamlWithSync,
		PolicyName:        "gen-role-policy",
	},
	{
		TestName:          "test-role-rolebinding-withsync-without-clone",
		RoleName:          "ns-role",
		RoleBindingName:   "ns-role-binding",
		ResourceNamespace: "test",
		Clone:             false,
		Sync:              true,
		Data:              roleRoleBindingYamlWithSync,
		PolicyName:        "gen-role-policy",
	},
	{
		TestName:                   "test-role-rolebinding-with-clone",
		RoleName:                   "ns-role",
		RoleBindingName:            "ns-role-binding",
		ResourceNamespace:          "test",
		Clone:                      true,
		CloneSourceRoleData:        sourceRoleYaml,
		CloneSourceRoleBindingData: sourceRoleBindingYaml,
		CloneNamespace:             "default",
		Sync:                       false,
		Data:                       roleRoleBindingYamlWithClone,
		PolicyName:                 "gen-role-policy",
	},
}

// ClusterRoleTests - E2E Test Config for ClusterRole and ClusterRoleBinding
var ClusterRoleTests = []struct {
	//TestName - Name of the Test
	TestName string
	// ClusterRoleName - Name of the ClusterRole to be Created
	ClusterRoleName string
	// ClusterRoleBindingName - Name of the ClusterRoleBinding
	ClusterRoleBindingName string
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
	// PolicyName - Name of the Policy
	PolicyName string
}{
	{
		TestName:               "test-clusterrole-clusterrolebinding-without-clone",
		ClusterRoleName:        "ns-cluster-role",
		ClusterRoleBindingName: "ns-cluster-role-binding",
		ResourceNamespace:      "test",
		Clone:                  false,
		Sync:                   false,
		Data:                   genClusterRoleYamlWithSync,
		PolicyName:             "gen-cluster-policy",
	},
	{
		TestName:               "test-clusterrole-clusterrolebinding-with-sync-without-clone",
		ClusterRoleName:        "ns-cluster-role",
		ClusterRoleBindingName: "ns-cluster-role-binding",
		ResourceNamespace:      "test",
		Clone:                  false,
		Sync:                   true,
		Data:                   genClusterRoleYamlWithSync,
		PolicyName:             "gen-cluster-policy",
	},
	{
		TestName:                          "test-clusterrole-clusterrolebinding-with-sync-with-clone",
		ClusterRoleName:                   "ns-cluster-role",
		ClusterRoleBindingName:            "ns-cluster-role-binding",
		ResourceNamespace:                 "test",
		Clone:                             true,
		ClonerClusterRoleName:             "base-cluster-role",
		ClonerClusterRoleBindingName:      "base-cluster-role-binding",
		CloneSourceClusterRoleData:        baseClusterRoleData,
		CloneSourceClusterRoleBindingData: baseClusterRoleBindingData,
		Sync:                              false,
		Data:                              genClusterRoleYamlWithSync,
		PolicyName:                        "gen-cluster-policy",
	},
}

// NetworkPolicyGenerateTests - E2E Test Config for NetworkPolicyGenerateTests
var NetworkPolicyGenerateTests = []struct {
	//TestName - Name of the Test
	TestName string
	// NetworkPolicyName - Name of the NetworkPolicy to be Created
	NetworkPolicyName string
	// ResourceNamespace - Namespace for which Resources are Created
	ResourceNamespace string
	// PolicyName - Name of the Policy
	PolicyName string
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
}{
	{
		TestName:          "test-generate-policy-for-namespace-with-label",
		NetworkPolicyName: "allow-dns",
		ResourceNamespace: "test",
		PolicyName:        "add-networkpolicy",
		Clone:             false,
		Sync:              true,
		Data:              genNetworkPolicyYaml,
	},
}

// NetworkPolicyGenerateTests - E2E Test Config for NetworkPolicyGenerateTests
var GenerateNetworkPolicyOnNamespaceWithoutLabelTests = []struct {
	//TestName - Name of the Test
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
	//TestName - Name of the Test
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
	//TestName - Name of the Test
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
	//TestName - Name of the Test
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
