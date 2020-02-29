package policyviolation

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/nirmata/kyverno/pkg/policyStatus"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
)

type dummyStore struct {
}

func (d *dummyStore) Get(policyName string) (*v1.ClusterPolicy, error) {
	return &v1.ClusterPolicy{
		Status: v1.PolicyStatus{
			Rules: []v1.RuleStats{
				{
					Name: "rule4",
				},
			},
		},
	}, nil
}

func Test_Stats(t *testing.T) {
	testCase := struct {
		violationCountStats []struct {
			policyName    string
			violatedRules []v1.ViolatedRule
		}
		expectedOutput []byte
	}{
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

	s := policyStatus.NewSync(nil, &dummyStore{})

	for _, violationCountStat := range testCase.violationCountStats {
		receiver := &violationCount{
			policyName:    violationCountStat.policyName,
			violatedRules: violationCountStat.violatedRules,
		}
		receiver.UpdateStatus(s)
	}

	output, _ := json.Marshal(s.Cache.Data)
	if !reflect.DeepEqual(output, testCase.expectedOutput) {
		t.Errorf("\n\nTestcase has failed\nExpected:\n%v\nGot:\n%v\n\n", string(testCase.expectedOutput), string(output))
	}
}
