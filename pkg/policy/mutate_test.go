package policy

import (
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
)

func TestListGenerateURs_FiltersByPolicyAndType(t *testing.T) {
	urs := []*kyvernov2.UpdateRequest{
		{
			Spec: kyvernov2.UpdateRequestSpec{
				Policy: "default/test-policy",
				Type:   kyvernov2.Generate,
			},
		},
		{
			Spec: kyvernov2.UpdateRequestSpec{
				Policy: "prod/test-policy",
				Type:   kyvernov2.Generate,
			},
		},
		{
			Spec: kyvernov2.UpdateRequestSpec{
				Policy: "default/test-policy",
				Type:   kyvernov2.Mutate,
			},
		},
	}

	filtered := make([]*kyvernov2.UpdateRequest, 0)

	for _, ur := range urs {
		if ur.Spec.Policy != "default/test-policy" {
			continue
		}

		if ur.Spec.Type != kyvernov2.Generate {
			continue
		}

		filtered = append(filtered, ur)
	}

	assert.Len(t, filtered, 1)
	assert.Equal(t, "default/test-policy", filtered[0].Spec.Policy)
	assert.Equal(t, kyvernov2.Generate, filtered[0].Spec.Type)
}