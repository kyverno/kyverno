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
}{
	{
		TestName:          "test-role-rolebinding-without-clone",
		RoleName:          "ns-role",
		RoleBindingName:   "ns-role-binding",
		ResourceNamespace: "test",
		Clone:             false,
		Sync:              false,
		Data:              roleRoleBindingYamlWithSync,
	},
	{
		TestName:          "test-role-rolebinding-withsync-without-clone",
		RoleName:          "ns-role",
		RoleBindingName:   "ns-role-binding",
		ResourceNamespace: "test",
		Clone:             false,
		Sync:              true,
		Data:              roleRoleBindingYamlWithSync,
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
}{
	{
		TestName:               "test-clusterrole-clusterrolebinding-without-clone",
		ClusterRoleName:        "ns-cluster-role",
		ClusterRoleBindingName: "ns-cluster-role-binding",
		ResourceNamespace:      "test",
		Clone:                  false,
		Sync:                   false,
		Data:                   genClusterRoleYamlWithSync,
	},
	{
		TestName:               "test-clusterrole-clusterrolebinding-with-sync-without-clone",
		ClusterRoleName:        "ns-cluster-role",
		ClusterRoleBindingName: "ns-cluster-role-binding",
		ResourceNamespace:      "test",
		Clone:                  false,
		Sync:                   true,
		Data:                   genClusterRoleYamlWithSync,
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
	},
}
