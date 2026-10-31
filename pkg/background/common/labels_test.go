package common

import (
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
)

func TestGenerateLabelsSet_ExtractsPolicyNameFromNamespacedKey(t *testing.T) {
	result := GenerateLabelsSet("default/generate-policy")

	assert.Equal(t, "generate-policy", result[kyvernov2.URGeneratePolicyLabel])
	assert.Len(t, result, 1)
}

func TestGenerateLabelsSet_HandlesComplexNamespace(t *testing.T) {
	result := GenerateLabelsSet("kube-system/system-policy")

	assert.Equal(t, "system-policy", result[kyvernov2.URGeneratePolicyLabel])
}
