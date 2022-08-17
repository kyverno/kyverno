package testrunner

import "testing"

func Test_Mutate_EndPoint(t *testing.T) {
	testScenario(t, "/test/scenarios/other/scenario_mutate_endpoint.yaml")
}

func Test_Mutate_Validate_qos(t *testing.T) {
	testScenario(t, "/test/scenarios/other/scenario_mutate_validate_qos.yaml")
}

func Test_disallow_privileged(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_priviledged.yaml")
}

func Test_validate_healthChecks(t *testing.T) {
	testScenario(t, "/test/scenarios/other/scenario_validate_healthChecks.yaml")
}

func Test_validate_host_network_port(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_host_network_port.yaml")
}

func Test_validate_host_PID_IPC(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_host_pid_ipc.yaml")
}

//TODO: support generate
// func Test_add_ns_quota(t *testing.T) {
// 	testScenario(t, "test/scenarios/samples/best_practices/add_ns_quota.yaml")
// }

func Test_validate_disallow_default_serviceaccount(t *testing.T) {
	testScenario(t, "test/scenarios/other/scenario_validate_disallow_default_serviceaccount.yaml")
}

func Test_validate_selinux_context(t *testing.T) {
	testScenario(t, "test/scenarios/other/scenario_validate_selinux_context.yaml")
}

func Test_validate_proc_mount(t *testing.T) {
	testScenario(t, "test/scenarios/other/scenario_validate_default_proc_mount.yaml")
}

func Test_validate_volume_whitelist(t *testing.T) {
	testScenario(t, "test/scenarios/other/scenario_validate_volume_whiltelist.yaml")
}

func Test_validate_disallow_bind_mounts_fail(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_bind_mounts_fail.yaml")
}

func Test_validate_disallow_bind_mounts_pass(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/disallow_bind_mounts_pass.yaml")
}

func Test_disallow_sysctls(t *testing.T) {
	testScenario(t, "/test/scenarios/samples/best_practices/disallow_sysctls.yaml")
}

func Test_add_safe_to_evict(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/add_safe_to_evict.yaml")
}

func Test_add_safe_to_evict_annotation2(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/add_safe_to_evict2.yaml")
}

func Test_add_safe_to_evict_annotation3(t *testing.T) {
	testScenario(t, "test/scenarios/samples/best_practices/add_safe_to_evict3.yaml")
}

func Test_validate_restrict_automount_sa_token_pass(t *testing.T) {
	testScenario(t, "test/scenarios/samples/more/restrict_automount_sa_token.yaml")
}

func Test_known_ingress(t *testing.T) {
	testScenario(t, "test/scenarios/samples/more/restrict_ingress_classes.yaml")
}

func Test_unknown_ingress(t *testing.T) {
	testScenario(t, "test/scenarios/samples/more/unknown_ingress_class.yaml")
}

func Test_mutate_pod_spec(t *testing.T) {
	testScenario(t, "test/scenarios/other/scenario_mutate_pod_spec.yaml")
}
