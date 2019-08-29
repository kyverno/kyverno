package testrunner

import "testing"

// func TestCLI(t *testing.T) {
// 	//https://github.com/nirmata/kyverno/issues/301
// 	runner(t, "/test/scenarios/cli")
// }

func Test_Devlop(t *testing.T) {
	//load scenario
	scenario, err := loadScenario(t, "/test/scenarios/test/s1.yaml")
	if err != nil {
		t.Error(err)
	}
	runScenario(t, scenario)

}
