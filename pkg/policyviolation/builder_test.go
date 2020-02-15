package policyviolation

import (
	"testing"

	"github.com/nirmata/kyverno/pkg/engine/response"
	"gotest.tools/assert"
)

func Test_GeneratePVsFromEngineResponse_PathNotExist(t *testing.T) {
	ers := []response.EngineResponse{
		{
			PolicyResponse: response.PolicyResponse{
				Policy: "test-substitute-variable",
				Resource: response.ResourceSpec{
					Kind:      "Pod",
					Name:      "test",
					Namespace: "test",
				},
				Rules: []response.RuleResponse{
					{
						Name:           "test-path-not-exist",
						Type:           "Mutation",
						Message:        "referenced paths are not present: request.object.metadata.name1",
						Success:        true,
						PathNotPresent: true,
					},
					{
						Name:           "test-path-exist",
						Type:           "Mutation",
						Success:        true,
						PathNotPresent: false,
					},
				},
			},
		},
		{
			PolicyResponse: response.PolicyResponse{
				Policy: "test-substitute-variable2",
				Resource: response.ResourceSpec{
					Kind:      "Pod",
					Name:      "test",
					Namespace: "test",
				},
				Rules: []response.RuleResponse{
					{
						Name:           "test-path-not-exist-across-policy",
						Type:           "Mutation",
						Message:        "referenced paths are not present: request.object.metadata.name1",
						Success:        true,
						PathNotPresent: true,
					},
				},
			},
		},
	}

	pvInfos := GeneratePVsFromEngineResponse(ers)
	assert.Assert(t, len(pvInfos) == 2)
}
