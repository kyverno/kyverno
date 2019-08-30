package testrunner

import "testing"

// func TestCLI(t *testing.T) {
// 	//https://github.com/nirmata/kyverno/issues/301
// 	runner(t, "/test/scenarios/cli")
// }

func Test_Mutate_EndPoint(t *testing.T) {
	testScenario(t, "/test/scenarios/test/scenario_mutate_endPpoint.yaml")
}

func Test_Mutate_imagePullPolicy(t *testing.T) {
	testScenario(t, "/test/scenarios/test/scenario_mutate_imagePullPolicy.yaml")
}

func Test_Mutate_Validate_qos(t *testing.T) {
	testScenario(t, "/test/scenarios/test/scenario_mutate_validate_qos.yaml")
}

func Test_validate_containerSecurityContext(t *testing.T) {
	testScenario(t, "/test/scenarios/test/scenario_validate_containerSecurityContext.yaml")
}

func Test_validate_healthChecks(t *testing.T) {
	testScenario(t, "/test/scenarios/test/scenario_validate_healthChecks.yaml")
}

func Test_validate_imageRegistries(t *testing.T) {
	testScenario(t, "/test/scenarios/test/scenario_validate_imageRegistries.yaml")
}

func Test_validate_nonRootUsers(t *testing.T) {
	testScenario(t, "/test/scenarios/test/scenario_validate_nonRootUser.yaml")
}

//TODO add tests for Generation
