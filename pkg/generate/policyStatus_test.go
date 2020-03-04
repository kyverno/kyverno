package generate

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
)

func Test_Stats(t *testing.T) {
	testCase := struct {
		generatedSyncStats []generateSyncStats
		expectedOutput     []byte
		existingStatus     map[string]v1.PolicyStatus
	}{
		expectedOutput: []byte(`{"policy1":{"averageExecutionTime":"","resourcesGeneratedCount":2,"ruleStatus":[{"ruleName":"rule1","averageExecutionTime":"23ns","resourcesGeneratedCount":1},{"ruleName":"rule2","averageExecutionTime":"44ns","resourcesGeneratedCount":1},{"ruleName":"rule3"}]}}`),
		generatedSyncStats: []generateSyncStats{
			{
				policyName: "policy1",
				ruleNameToProcessingTime: map[string]time.Duration{
					"rule1": time.Nanosecond * 23,
					"rule2": time.Nanosecond * 44,
				},
			},
		},
		existingStatus: map[string]v1.PolicyStatus{
			"policy1": {
				Rules: []v1.RuleStats{
					{
						Name: "rule1",
					},
					{
						Name: "rule2",
					},
					{
						Name: "rule3",
					},
				},
			},
		},
	}

	for _, generateSyncStat := range testCase.generatedSyncStats {
		testCase.existingStatus[generateSyncStat.PolicyName()] = generateSyncStat.UpdateStatus(testCase.existingStatus[generateSyncStat.PolicyName()])
	}

	output, _ := json.Marshal(testCase.existingStatus)
	if !reflect.DeepEqual(output, testCase.expectedOutput) {
		t.Errorf("\n\nTestcase has failed\nExpected:\n%v\nGot:\n%v\n\n", string(testCase.expectedOutput), string(output))
	}
}
