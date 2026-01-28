package metrics

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
)

func TestParseRuleType_validate_rule(t *testing.T) {
	t.Parallel()

	rule := kyvernov1.Rule{
		Validation: &kyvernov1.Validation{
			Message: "Container must have resource limits",
		},
	}

	result := ParseRuleType(rule)
	assert.Equal(t, Validate, result)
}

func TestParseRuleType_mutate_rule(t *testing.T) {
	t.Parallel()

	rule := kyvernov1.Rule{
		Mutation: &kyvernov1.Mutation{
			Targets: []kyvernov1.TargetResourceSpec{
				{
					TargetSelector: kyvernov1.TargetSelector{
						ResourceSpec: kyvernov1.ResourceSpec{Kind: "ConfigMap"},
					},
				},
			},
		},
	}

	result := ParseRuleType(rule)
	assert.Equal(t, Mutate, result)
}

func TestParseRuleType_generate_rule(t *testing.T) {
	t.Parallel()

	rule := kyvernov1.Rule{
		Generation: &kyvernov1.Generation{
			Synchronize: true,
			GeneratePattern: kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{
					Kind:      "ConfigMap",
					Namespace: "default",
					Name:      "generated-config",
				},
			},
		},
	}

	result := ParseRuleType(rule)
	assert.Equal(t, Generate, result)
}

func TestParseRuleType_image_verify_rule(t *testing.T) {
	t.Parallel()

	rule := kyvernov1.Rule{
		VerifyImages: []kyvernov1.ImageVerification{
			{
				ImageReferences: []string{"ghcr.io/kyverno/*"},
			},
		},
	}

	result := ParseRuleType(rule)
	assert.Equal(t, ImageVerify, result)
}

func TestParseRuleType_empty_rule(t *testing.T) {
	t.Parallel()

	rule := kyvernov1.Rule{}

	result := ParseRuleType(rule)
	assert.Equal(t, EmptyRuleType, result)
}

func TestParseRuleType_empty_validation_block(t *testing.T) {
	t.Parallel()

	rule := kyvernov1.Rule{
		Validation: &kyvernov1.Validation{},
	}

	result := ParseRuleType(rule)
	assert.Equal(t, EmptyRuleType, result)
}

func TestParseRuleType_empty_mutation_block(t *testing.T) {
	t.Parallel()

	rule := kyvernov1.Rule{
		Mutation: &kyvernov1.Mutation{},
	}

	result := ParseRuleType(rule)
	assert.Equal(t, EmptyRuleType, result)
}

func TestParseRuleType_empty_generation_block(t *testing.T) {
	t.Parallel()

	rule := kyvernov1.Rule{
		Generation: &kyvernov1.Generation{},
	}

	result := ParseRuleType(rule)
	assert.Equal(t, EmptyRuleType, result)
}

func TestParseRuleType_empty_verify_images(t *testing.T) {
	t.Parallel()

	rule := kyvernov1.Rule{
		VerifyImages: []kyvernov1.ImageVerification{},
	}

	result := ParseRuleType(rule)
	assert.Equal(t, EmptyRuleType, result)
}

func TestParseResourceRequestOperation_create(t *testing.T) {
	t.Parallel()

	result, err := ParseResourceRequestOperation("CREATE")

	assert.NoError(t, err)
	assert.Equal(t, ResourceCreated, result)
}

func TestParseResourceRequestOperation_update(t *testing.T) {
	t.Parallel()

	result, err := ParseResourceRequestOperation("UPDATE")

	assert.NoError(t, err)
	assert.Equal(t, ResourceUpdated, result)
}

func TestParseResourceRequestOperation_delete(t *testing.T) {
	t.Parallel()

	result, err := ParseResourceRequestOperation("DELETE")

	assert.NoError(t, err)
	assert.Equal(t, ResourceDeleted, result)
}

func TestParseResourceRequestOperation_connect(t *testing.T) {
	t.Parallel()

	result, err := ParseResourceRequestOperation("CONNECT")

	assert.NoError(t, err)
	assert.Equal(t, ResourceConnected, result)
}

func TestParseResourceRequestOperation_invalid_operation(t *testing.T) {
	t.Parallel()

	result, err := ParseResourceRequestOperation("INVALID")

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "unknown request operation")
}

func TestParseResourceRequestOperation_lowercase_rejected(t *testing.T) {
	t.Parallel()

	result, err := ParseResourceRequestOperation("create")

	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestParseResourceRequestOperation_empty_string(t *testing.T) {
	t.Parallel()

	result, err := ParseResourceRequestOperation("")

	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestParseRuleTypeFromEngineRuleResponse_validation(t *testing.T) {
	t.Parallel()

	response := engineapi.NewRuleResponse(
		"require-labels",
		engineapi.Validation,
		"",
		engineapi.RuleStatusPass,
		nil,
	)

	result := ParseRuleTypeFromEngineRuleResponse(*response)
	assert.Equal(t, Validate, result)
}

func TestParseRuleTypeFromEngineRuleResponse_mutation(t *testing.T) {
	t.Parallel()

	response := engineapi.NewRuleResponse(
		"add-default-labels",
		engineapi.Mutation,
		"",
		engineapi.RuleStatusPass,
		nil,
	)

	result := ParseRuleTypeFromEngineRuleResponse(*response)
	assert.Equal(t, Mutate, result)
}

func TestParseRuleTypeFromEngineRuleResponse_generation(t *testing.T) {
	t.Parallel()

	response := engineapi.NewRuleResponse(
		"generate-network-policy",
		engineapi.Generation,
		"",
		engineapi.RuleStatusPass,
		nil,
	)

	result := ParseRuleTypeFromEngineRuleResponse(*response)
	assert.Equal(t, Generate, result)
}

func TestParseRuleTypeFromEngineRuleResponse_image_verify(t *testing.T) {
	t.Parallel()

	response := engineapi.NewRuleResponse(
		"verify-image-signature",
		engineapi.ImageVerify,
		"",
		engineapi.RuleStatusPass,
		nil,
	)

	result := ParseRuleTypeFromEngineRuleResponse(*response)
	assert.Equal(t, ImageVerify, result)
}

func TestParseRuleTypeFromEngineRuleResponse_unknown_type(t *testing.T) {
	t.Parallel()

	response := engineapi.NewRuleResponse(
		"unknown-rule",
		engineapi.RuleType("Unknown"),
		"",
		engineapi.RuleStatusPass,
		nil,
	)

	result := ParseRuleTypeFromEngineRuleResponse(*response)
	assert.Equal(t, EmptyRuleType, result)
}

func TestParseRuleType_validation_takes_priority(t *testing.T) {
	t.Parallel()

	rule := kyvernov1.Rule{
		Validation: &kyvernov1.Validation{
			Message: "validation message",
		},
		Mutation: &kyvernov1.Mutation{
			Targets: []kyvernov1.TargetResourceSpec{
				{
					TargetSelector: kyvernov1.TargetSelector{
						ResourceSpec: kyvernov1.ResourceSpec{Kind: "Pod"},
					},
				},
			},
		},
	}

	result := ParseRuleType(rule)
	assert.Equal(t, Validate, result)
}

func TestParseRuleType_mutation_over_generation(t *testing.T) {
	t.Parallel()

	rule := kyvernov1.Rule{
		Mutation: &kyvernov1.Mutation{
			Targets: []kyvernov1.TargetResourceSpec{
				{
					TargetSelector: kyvernov1.TargetSelector{
						ResourceSpec: kyvernov1.ResourceSpec{Kind: "Pod"},
					},
				},
			},
		},
		Generation: &kyvernov1.Generation{
			Synchronize: true,
			GeneratePattern: kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{Kind: "ConfigMap"},
			},
		},
	}

	result := ParseRuleType(rule)
	assert.Equal(t, Mutate, result)
}

func TestParseRuleType_generation_over_image_verify(t *testing.T) {
	t.Parallel()

	rule := kyvernov1.Rule{
		Generation: &kyvernov1.Generation{
			Synchronize: true,
			GeneratePattern: kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{Kind: "ConfigMap"},
			},
		},
		VerifyImages: []kyvernov1.ImageVerification{
			{ImageReferences: []string{"*"}},
		},
	}

	result := ParseRuleType(rule)
	assert.Equal(t, Generate, result)
}
