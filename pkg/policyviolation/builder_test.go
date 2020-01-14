package policyviolation

import (
	"testing"

	"github.com/nirmata/kyverno/pkg/engine/response"
	"gotest.tools/assert"
)

func Test_GeneratePVsFromEngineResponse_PathNotExist(t *testing.T) {
	ers := []response.EngineResponse{
		response.EngineResponse{
			PolicyResponse: response.PolicyResponse{
				Policy: "test-substitue-variable",
				Resource: response.ResourceSpec{
					Kind:      "Pod",
					Name:      "test",
					Namespace: "test",
				},
				Rules: []response.RuleResponse{
					response.RuleResponse{
						Name:           "test-path-not-exist",
						Type:           "Mutation",
						Message:        "referenced paths are not present: request.object.metadata.name1",
						Success:        true,
						PathNotPresent: true,
					},
					response.RuleResponse{
						Name:           "test-path-exist",
						Type:           "Mutation",
						Success:        true,
						PathNotPresent: false,
					},
				},
			},
		},
		response.EngineResponse{
			PolicyResponse: response.PolicyResponse{
				Policy: "test-substitue-variable2",
				Resource: response.ResourceSpec{
					Kind:      "Pod",
					Name:      "test",
					Namespace: "test",
				},
				Rules: []response.RuleResponse{
					response.RuleResponse{
						Name:           "test-path-not-exist-accross-policy",
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
