package generate

import (
	"encoding/json"
	"reflect"
	"testing"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/policyStatus"
)

type dummyStore struct {
}

func (d *dummyStore) Get(policyName string) (*v1.ClusterPolicy, error) {
	return &v1.ClusterPolicy{}, nil
}

func Test_Stats(t *testing.T) {
	testCase := struct {
		generatedCountStats []v1.GenerateRequest
		expectedOutput      []byte
	}{
		expectedOutput: []byte(`{"policy1":{"averageExecutionTime":"","resourcesGeneratedCount":1},"policy2":{"averageExecutionTime":"","resourcesGeneratedCount":1}}`),
		generatedCountStats: []v1.GenerateRequest{
			{
				Spec: v1.GenerateRequestSpec{
					Policy: "policy1",
				},
				Status: v1.GenerateRequestStatus{
					GeneratedResources: make([]v1.ResourceSpec, 1),
				},
			},
			{
				Spec: v1.GenerateRequestSpec{
					Policy: "policy2",
				},
				Status: v1.GenerateRequestStatus{
					GeneratedResources: make([]v1.ResourceSpec, 1),
				},
			},
		},
	}

	s := policyStatus.NewSync(nil, &dummyStore{})

	for _, generateCountStat := range testCase.generatedCountStats {
		receiver := &generateSyncStats{
			generateRequest: generateCountStat,
		}
		receiver.UpdateStatus(s)
	}

	output, _ := json.Marshal(s.Cache.Data)
	if !reflect.DeepEqual(output, testCase.expectedOutput) {
		t.Errorf("\n\nTestcase has failed\nExpected:\n%v\nGot:\n%v\n\n", string(testCase.expectedOutput), string(output))
	}
}
