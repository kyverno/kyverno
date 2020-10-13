package policyviolation

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/engine/response"
	"gotest.tools/assert"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
						Name:    "test-path-not-exist",
						Type:    "Mutation",
						Message: "referenced paths are not present: request.object.metadata.name1",
						Success: false,
					},
					{
						Name:    "test-path-exist",
						Type:    "Mutation",
						Success: true,
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
						Name:    "test-path-not-exist-across-policy",
						Type:    "Mutation",
						Message: "referenced paths are not present: request.object.metadata.name1",
						Success: true,
					},
				},
			},
		},
	}

	pvInfos := GeneratePVsFromEngineResponse(ers, log.Log)
	assert.Assert(t, len(pvInfos) == 1)
}
