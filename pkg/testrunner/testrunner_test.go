package testrunner

import "testing"

func Test_Mutate_EndPoint(t *testing.T) {
	testScenario(t, "/test/scenarios/other/scenario_mutate_endpoint.yaml")
}

func Test_Mutate_Validate_qos(t *testing.T) {
	testScenario(t, "/test/scenarios/other/scenario_mutate_validate_qos.yaml")
}

func Test_validate_deny_runasrootuser(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_deny_runasrootuser.yaml")
}

func Test_validate_disallow_priviledgedprivelegesecalation(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_disallow_priviledged_privelegesecalation.yaml")
}

func Test_validate_healthChecks(t *testing.T) {
	testScenario(t, "/test/scenarios/other/scenario_validate_healthChecks.yaml")
}

func Test_validate_nonRootUsers(t *testing.T) {
	testScenario(t, "/test/scenarios/samples/best_practices/scenario_validate_nonRootUser.yaml")
}

func Test_generate_networkPolicy(t *testing.T) {
	testScenario(t, "/test/scenarios/samples/best_practices/scenario_generate_networkPolicy.yaml")
}

// namespace is blank, not "default" as testrunner evaulates the policyengine, but the "default" is added by kubeapiserver

func Test_validate_require_image_tag_not_latest_deny(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_valiadate_require_image_tag_not_latest_deny.yaml")
}

func Test_validate_require_image_tag_not_latest_pass(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_valiadate_require_image_tag_not_latest_pass.yaml")
}

func Test_validate_disallow_automoutingapicred_pass(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_disallow_automountingapicred.yaml")
}

func Test_validate_disallow_default_namespace(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_disallow_default_namespace.yaml")
}

func Test_validate_host_network_port(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_disallow_host_network_hostport.yaml")
}

func Test_validate_hostPID_hostIPC(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_disallow_hostpid_hostipc.yaml")
}

func Test_validate_not_readonly_rootfilesystem(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_require_readonly_rootfilesystem.yaml")
}

func Test_validate_require_namespace_quota(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_require_namespace_quota.yaml")
}

func Test_validate_disallow_node_port(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_disallow_node_port.yaml")
}

func Test_validate_disallow_default_serviceaccount(t *testing.T) {
	testScenario(t, "test/scenarios/other/scenario_validate_disallow_default_serviceaccount.yaml")
}

func Test_validate_fsgroup(t *testing.T) {
	testScenario(t, "test/scenarios/samples/more/scenario_validate_fsgroup.yaml")
}

func Test_validate_selinux_context(t *testing.T) {
	testScenario(t, "test/scenarios/other/scenario_validate_selinux_context.yaml")
}

func Test_validate_proc_mount(t *testing.T) {
	testScenario(t, "test/scenarios/other/scenario_validate_default_proc_mount.yaml")
}

func Test_validate_container_capabilities(t *testing.T) {
	testScenario(t, "test/scenarios/samples/more/scenario_validate_container_capabilities.yaml")
}

func Test_validate_disallow_sysctl(t *testing.T) {
	testScenario(t, "test/scenarios/samples/more/scenario_validate_sysctl_configs.yaml")
}

func Test_validate_volume_whitelist(t *testing.T) {
	testScenario(t, "test/scenarios/other/scenario_validate_volume_whiltelist.yaml")
}

func Test_validate_trusted_image_registries(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_trusted_image_registries.yaml")
}

func Test_require_pod_requests_limits(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_require_pod_requests_limits.yaml")
}

func Test_require_probes(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_probes.yaml")
}

func Test_validate_disallow_host_filesystem_fail(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_disallow_host_filesystem.yaml")
}

func Test_validate_disallow_host_filesystem_pass(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_disallow_host_filesystem_pass.yaml")
}
func Test_validate_disallow_new_capabilities(t *testing.T) {
	testScenario(t, "/test/scenarios/samples/best_practices/scenario_validate_disallow_new_capabilities.yaml")
}

func Test_validate_disallow_docker_sock_mount(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_disallow_docker_sock_mount.yaml")
}

func Test_validate_disallow_helm_tiller(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_disallow_helm_tiller.yaml")
}

func Test_add_safe_to_evict_annotation(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_mutate_safe-to-evict.yaml")
}

func Test_add_safe_to_evict_annotation2(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_mutate_safe-to-evict2.yaml")
}
