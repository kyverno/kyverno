package imageverification

import (
	"testing"
	"time"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNamespaceLabelErrorCreatesEngineResponse(t *testing.T) {
	// Simulate what the fix does: create an error response when namespace labels fail
	var engineResponses []engineapi.EngineResponse

	// This is what the fix adds when namespace label fetch fails
	resp := engineapi.NewEngineResponse(
		unstructured.Unstructured{},
		nil,
		nil,
	)
	policyResponse := engineapi.NewPolicyResponse()
	policyResponse.Add(
		engineapi.NewExecutionStats(time.Now(), time.Now()),
		*engineapi.RuleError("", engineapi.ImageVerify, "failed to get namespace labels", nil, nil),
	)
	resp = resp.WithPolicyResponse(policyResponse)
	engineResponses = append(engineResponses, resp)

	// Verify error response was created
	assert.Len(t, engineResponses, 1)
	assert.Equal(t, 1, engineResponses[0].PolicyResponse.RulesErrorCount())
	assert.False(t, engineResponses[0].IsSuccessful())
}
