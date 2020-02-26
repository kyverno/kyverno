package policyStatus

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"

	"github.com/nirmata/kyverno/pkg/engine/response"
)

func Test_Stats(t *testing.T) {
	testCase := struct {
		mutateStats         []response.EngineResponse
		validateStats       []response.EngineResponse
		generateStats       []response.EngineResponse
		violationCountStats []struct {
			policyName    string
			violatedRules []v1.ViolatedRule
		}
		generatedCountStats []v1.GenerateRequest
		expectedOutput      []byte
	}{
		expectedOutput: []byte(`{"policy1":{"averageExecutionTime":"1.482µs","violationCount":1,"rulesFailedCount":3,"rulesAppliedCount":3,"resourcesBlockedCount":1,"resourcesMutatedCount":1,"resourcesGeneratedCount":1,"ruleStatus":[{"ruleName":"rule1","averageExecutionTime":"243ns","appliedCount":1,"resourcesMutatedCount":1},{"ruleName":"rule2","averageExecutionTime":"251ns","failedCount":1},{"ruleName":"rule3","averageExecutionTime":"243ns","appliedCount":1},{"ruleName":"rule4","averageExecutionTime":"251ns","violationCount":1,"failedCount":1,"resourcesBlockedCount":1},{"ruleName":"rule5","averageExecutionTime":"243ns","appliedCount":1},{"ruleName":"rule6","averageExecutionTime":"251ns","failedCount":1}]},"policy2":{"averageExecutionTime":"1.299µs","violationCount":1,"rulesFailedCount":3,"rulesAppliedCount":3,"resourcesMutatedCount":1,"resourcesGeneratedCount":1,"ruleStatus":[{"ruleName":"rule1","averageExecutionTime":"222ns","appliedCount":1,"resourcesMutatedCount":1},{"ruleName":"rule2","averageExecutionTime":"211ns","failedCount":1},{"ruleName":"rule3","averageExecutionTime":"222ns","appliedCount":1},{"ruleName":"rule4","averageExecutionTime":"211ns","violationCount":1,"failedCount":1},{"ruleName":"rule5","averageExecutionTime":"222ns","appliedCount":1},{"ruleName":"rule6","averageExecutionTime":"211ns","failedCount":1}]}}`),
		generatedCountStats: []v1.GenerateRequest{
			{
				Spec: v1.GenerateRequestSpec{
					Policy: "policy1",
				},
				Status: v1.GenerateRequestStatus{
					GeneratedResources: make([]v1.ResourceSpec, 1, 1),
				},
			},
			{
				Spec: v1.GenerateRequestSpec{
					Policy: "policy2",
				},
				Status: v1.GenerateRequestStatus{
					GeneratedResources: make([]v1.ResourceSpec, 1, 1),
				},
			},
		},
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
		mutateStats: []response.EngineResponse{
			{
				PolicyResponse: response.PolicyResponse{
					Policy: "policy1",
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
					Policy: "policy2",
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
		validateStats: []response.EngineResponse{
			{
				PolicyResponse: response.PolicyResponse{
					Policy:                  "policy1",
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
					Policy: "policy2",
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
		generateStats: []response.EngineResponse{
			{
				PolicyResponse: response.PolicyResponse{
					Policy: "policy1",
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
					Policy: "policy2",
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

	s := NewSync(nil, nil, nil)
	for _, mutateStat := range testCase.mutateStats {
		receiver := &mutateStats{
			s:    s,
			resp: mutateStat,
		}
		receiver.updateStatus()
	}

	for _, validateStat := range testCase.validateStats {
		receiver := &validateStats{
			s:    s,
			resp: validateStat,
		}
		receiver.updateStatus()
	}

	for _, generateStat := range testCase.generateStats {
		receiver := &generateStats{
			s:    s,
			resp: generateStat,
		}
		receiver.updateStatus()
	}

	for _, generateCountStat := range testCase.generatedCountStats {
		receiver := &generatedResourceCount{
			sync:            s,
			generateRequest: generateCountStat,
		}
		receiver.updateStatus()
	}

	for _, violationCountStat := range testCase.violationCountStats {
		receiver := &violationCount{
			sync:          s,
			policyName:    violationCountStat.policyName,
			violatedRules: violationCountStat.violatedRules,
		}
		receiver.updateStatus()
	}

	output, _ := json.Marshal(s.cache.data)
	if !reflect.DeepEqual(output, testCase.expectedOutput) {
		t.Errorf("\n\nTestcase has failed\nExpected:\n%v\nGot:\n%v\n\n", string(testCase.expectedOutput), string(output))
	}
}
