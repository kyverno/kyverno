package testrunner

import "testing"

func Test_Mutate_EndPoint(t *testing.T) {
	testScenario(t, "/test/scenarios/other/scenario_mutate_endpoint.yaml")
}

func Test_Mutate_Validate_qos(t *testing.T) {
	testScenario(t, "/test/scenarios/other/scenario_mutate_validate_qos.yaml")
}

func Test_disallow_root_user(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_root_user.yaml")
}

func Test_disallow_priviledged(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_priviledged.yaml")
}

func Test_validate_healthChecks(t *testing.T) {
	testScenario(t, "/test/scenarios/other/scenario_validate_healthChecks.yaml")
}

func Test_add_networkPolicy(t *testing.T) {
	testScenario(t, "/test/scenarios/samples/best_practices/add_networkPolicy.yaml")
}

// namespace is blank, not "default" as testrunner evaulates the policyengine, but the "default" is added by kubeapiserver

func Test_validate_disallow_latest_tag(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_latest_tag.yaml")
}

func Test_validate_require_image_tag_not_latest_pass(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_latest_tag_pass.yaml")
}

func Test_validate_disallow_automoutingapicred_pass(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_disallow_automountingapicred.yaml")
}

func Test_validate_disallow_default_namespace(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_default_namespace.yaml")
}

func Test_validate_host_network_port(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_host_network_port.yaml")
}

func Test_validate_host_PID_IPC(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_host_pid_ipc.yaml")
}

func Test_validate_ro_rootfs(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/require_ro_rootfs.yaml")
}

func Test_add_ns_quota(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/add_ns_quota.yaml")
}

func Test_validate_disallow_node_port(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_node_port.yaml")
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

func Test_validate_restrict_image_registries(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/restrict_image_registries.yaml")
}

func Test_require_pod_requests_limits(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/require_pod_requests_limits.yaml")
}

func Test_require_probes(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/require_probes.yaml")
}

func Test_validate_disallow_bind_mounts_fail(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_bind_mounts_fail.yaml")
}

func Test_validate_disallow_bind_mounts_pass(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_bind_mounts_pass.yaml")
}

func Test_validate_disallow_new_capabilities(t *testing.T) {
	testScenario(t, "/test/scenarios/samples/best_practices/disallow_new_capabilities.yaml")
}

func Test_validate_disallow_docker_sock_mount(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_docker_sock_mount.yaml")
}

func Test_validate_disallow_helm_tiller(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_disallow_helm_tiller.yaml")
}

func Test_add_safe_to_evict(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/add_safe_to_evict.yaml")
}

func Test_add_safe_to_evict_annotation2(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/add_safe_to_evict2.yaml")
}

func Test_known_ingress(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_known_ingress_class.yaml")
}

func Test_unknown_ingress(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/scenario_validate_unknown_ingress_class.yaml")
}
