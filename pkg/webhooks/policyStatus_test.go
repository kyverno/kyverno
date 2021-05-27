package webhooks

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
)

func Test_GenerateStats(t *testing.T) {
	testCase := struct {
		generateStats  []*response.EngineResponse
		expectedOutput []byte
	}{
		expectedOutput: []byte(`{"policy1":{"averageExecutionTime":"494ns","rulesFailedCount":1,"rulesAppliedCount":1,"ruleStatus":[{"ruleName":"rule5","averageExecutionTime":"243ns","appliedCount":1},{"ruleName":"rule6","averageExecutionTime":"251ns","failedCount":1}]},"policy2":{"averageExecutionTime":"433ns","rulesFailedCount":1,"rulesAppliedCount":1,"ruleStatus":[{"ruleName":"rule5","averageExecutionTime":"222ns","appliedCount":1},{"ruleName":"rule6","averageExecutionTime":"211ns","failedCount":1}]}}`),
		generateStats: []*response.EngineResponse{
			{
				PolicyResponse: response.PolicyResponse{
					Policy: response.PolicySpec{Name: "policy1"},
					Rules: []response.RuleResponse{
						{
							Name:    "rule5",
							Success: true,
							RuleStats: response.RuleStats{
								ProcessingTime: time.Nanosecond * 243,
							},
						},
						{
							Name:    "rule6",
							Success: false,
							RuleStats: response.RuleStats{
								ProcessingTime: time.Nanosecond * 251,
							},
						},
					},
				},
			},
			{
				PolicyResponse: response.PolicyResponse{
					Policy: response.PolicySpec{Name: "policy2"},
					Rules: []response.RuleResponse{
						{
							Name:    "rule5",
							Success: true,
							RuleStats: response.RuleStats{
								ProcessingTime: time.Nanosecond * 222,
							},
						},
						{
							Name:    "rule6",
							Success: false,
							RuleStats: response.RuleStats{
								ProcessingTime: time.Nanosecond * 211,
							},
						},
					},
				},
			},
		},
	}

	policyNameToStatus := map[string]v1.PolicyStatus{}

	for _, generateStat := range testCase.generateStats {
		receiver := generateStats{
			resp: generateStat,
		}
		policyNameToStatus[receiver.PolicyName()] = receiver.UpdateStatus(policyNameToStatus[receiver.PolicyName()])
	}

	output, _ := json.Marshal(policyNameToStatus)
	if !reflect.DeepEqual(output, testCase.expectedOutput) {
		t.Errorf("\n\nTestcase has failed\nExpected:\n%v\nGot:\n%v\n\n", string(testCase.expectedOutput), string(output))
	}
}

func Test_MutateStats(t *testing.T) {
	testCase := struct {
		mutateStats    []*response.EngineResponse
		expectedOutput []byte
	}{
		expectedOutput: []byte(`{"policy1":{"averageExecutionTime":"494ns","rulesFailedCount":1,"rulesAppliedCount":1,"resourcesMutatedCount":1,"ruleStatus":[{"ruleName":"rule1","averageExecutionTime":"243ns","appliedCount":1,"resourcesMutatedCount":1},{"ruleName":"rule2","averageExecutionTime":"251ns","failedCount":1}]},"policy2":{"averageExecutionTime":"433ns","rulesFailedCount":1,"rulesAppliedCount":1,"resourcesMutatedCount":1,"ruleStatus":[{"ruleName":"rule1","averageExecutionTime":"222ns","appliedCount":1,"resourcesMutatedCount":1},{"ruleName":"rule2","averageExecutionTime":"211ns","failedCount":1}]}}`),
		mutateStats: []*response.EngineResponse{
			{
				PolicyResponse: response.PolicyResponse{
					Policy: response.PolicySpec{Name: "policy1"},
					Rules: []response.RuleResponse{
						{
							Name:    "rule1",
							Success: true,
							RuleStats: response.RuleStats{
								ProcessingTime: time.Nanosecond * 243,
							},
						},
						{
							Name:    "rule2",
							Success: false,
							RuleStats: response.RuleStats{
								ProcessingTime: time.Nanosecond * 251,
							},
						},
					},
				},
			},
			{
				PolicyResponse: response.PolicyResponse{
					Policy: response.PolicySpec{Name: "policy2"},
					Rules: []response.RuleResponse{
						{
							Name:    "rule1",
							Success: true,
							RuleStats: response.RuleStats{
								ProcessingTime: time.Nanosecond * 222,
							},
						},
						{
							Name:    "rule2",
							Success: false,
							RuleStats: response.RuleStats{
								ProcessingTime: time.Nanosecond * 211,
							},
						},
					},
				},
			},
		},
	}

	policyNameToStatus := map[string]v1.PolicyStatus{}
	for _, mutateStat := range testCase.mutateStats {
		receiver := mutateStats{
			resp: mutateStat,
		}
		policyNameToStatus[receiver.PolicyName()] = receiver.UpdateStatus(policyNameToStatus[receiver.PolicyName()])
	}

	output, _ := json.Marshal(policyNameToStatus)
	if !reflect.DeepEqual(output, testCase.expectedOutput) {
		t.Errorf("\n\nTestcase has failed\nExpected:\n%v\nGot:\n%v\n\n", string(testCase.expectedOutput), string(output))
	}
}

func Test_ValidateStats(t *testing.T) {
	testCase := struct {
		validateStats  []*response.EngineResponse
		expectedOutput []byte
	}{
		expectedOutput: []byte(`{"policy1":{"averageExecutionTime":"494ns","rulesFailedCount":1,"rulesAppliedCount":1,"resourcesBlockedCount":1,"ruleStatus":[{"ruleName":"rule3","averageExecutionTime":"243ns","appliedCount":1},{"ruleName":"rule4","averageExecutionTime":"251ns","failedCount":1,"resourcesBlockedCount":1}]},"policy2":{"averageExecutionTime":"433ns","rulesFailedCount":1,"rulesAppliedCount":1,"ruleStatus":[{"ruleName":"rule3","averageExecutionTime":"222ns","appliedCount":1},{"ruleName":"rule4","averageExecutionTime":"211ns","failedCount":1}]}}`),
		validateStats: []*response.EngineResponse{
			{
				PolicyResponse: response.PolicyResponse{
					Policy:                  response.PolicySpec{Name: "policy1"},
					ValidationFailureAction: "enforce",
					Rules: []response.RuleResponse{
						{
							Name:    "rule3",
							Success: true,
							RuleStats: response.RuleStats{
								ProcessingTime: time.Nanosecond * 243,
							},
						},
						{
							Name:    "rule4",
							Success: false,
							RuleStats: response.RuleStats{
								ProcessingTime: time.Nanosecond * 251,
							},
						},
					},
				},
			},
			{
				PolicyResponse: response.PolicyResponse{
					Policy: response.PolicySpec{Name: "policy2"},
					Rules: []response.RuleResponse{
						{
							Name:    "rule3",
							Success: true,
							RuleStats: response.RuleStats{
								ProcessingTime: time.Nanosecond * 222,
							},
						},
						{
							Name:    "rule4",
							Success: false,
							RuleStats: response.RuleStats{
								ProcessingTime: time.Nanosecond * 211,
							},
						},
					},
				},
			},
		},
	}

	policyNameToStatus := map[string]v1.PolicyStatus{}
	for _, validateStat := range testCase.validateStats {
		receiver := validateStats{
			resp: validateStat,
		}
		policyNameToStatus[receiver.PolicyName()] = receiver.UpdateStatus(policyNameToStatus[receiver.PolicyName()])
	}

	output, _ := json.Marshal(policyNameToStatus)
	if !reflect.DeepEqual(output, testCase.expectedOutput) {
		t.Errorf("\n\nTestcase has failed\nExpected:\n%v\nGot:\n%v\n\n", string(testCase.expectedOutput), string(output))
	}
}
