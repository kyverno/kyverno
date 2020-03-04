package policyviolation

import (
	"encoding/json"
	"reflect"
	"testing"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
)

func Test_Stats(t *testing.T) {
	testCase := struct {
		violationCountStats []struct {
			policyName    string
			violatedRules []v1.ViolatedRule
		}
		expectedOutput []byte
		existingCache  map[string]v1.PolicyStatus
	}{
		existingCache: map[string]v1.PolicyStatus{
			"policy1": {
				Rules: []v1.RuleStats{
					{
						Name: "rule4",
					},
				},
			},
			"policy2": {
				Rules: []v1.RuleStats{
					{
						Name: "rule4",
					},
				},
			},
		},
		expectedOutput: []byte(`{"policy1":{"averageExecutionTime":"","violationCount":1,"ruleStatus":[{"ruleName":"rule4","violationCount":1}]},"policy2":{"averageExecutionTime":"","violationCount":1,"ruleStatus":[{"ruleName":"rule4","violationCount":1}]}}`),
		violationCountStats: []struct {
			policyName    string
			violatedRules []v1.ViolatedRule
		}{
			{
				policyName: "policy1",
				violatedRules: []v1.ViolatedRule{
					{
						Name: "rule4",
					},
				},
			},
			{
				policyName: "policy2",
				violatedRules: []v1.ViolatedRule{
					{
						Name: "rule4",
					},
				},
			},
		},
	}

	policyNameToStatus := testCase.existingCache

	for _, violationCountStat := range testCase.violationCountStats {
		receiver := &violationCount{
			policyName:    violationCountStat.policyName,
			violatedRules: violationCountStat.violatedRules,
		}
		policyNameToStatus[receiver.PolicyName()] = receiver.UpdateStatus(policyNameToStatus[receiver.PolicyName()])
	}

	output, _ := json.Marshal(policyNameToStatus)
	if !reflect.DeepEqual(output, testCase.expectedOutput) {
		t.Errorf("\n\nTestcase has failed\nExpected:\n%v\nGot:\n%v\n\n", string(testCase.expectedOutput), string(output))
	}
}
