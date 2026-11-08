package internal

import (
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestImageReferencesHasVariables(t *testing.T) {
	ruleNoVar := kyvernov1.Rule{
		VerifyImages: []kyvernov1.ImageVerification{
			{
				ImageReferences: []string{"ghcr.io/kyverno/test:latest", "index.docker.io/library/nginx:*"},
			},
		},
	}
	assert.False(t, ImageReferencesHasVariables(ruleNoVar))

	ruleWithVar := kyvernov1.Rule{
		VerifyImages: []kyvernov1.ImageVerification{
			{
				ImageReferences: []string{"{{ request.object.metadata.annotations.allowedImage }}"},
			},
		},
	}
	assert.True(t, ImageReferencesHasVariables(ruleWithVar))

	ruleWithRef := kyvernov1.Rule{
		VerifyImages: []kyvernov1.ImageVerification{
			{
				ImageReferences: []string{"$(allowedImage)"},
			},
		},
	}
	assert.True(t, ImageReferencesHasVariables(ruleWithRef))

	ruleEmpty := kyvernov1.Rule{}
	assert.False(t, ImageReferencesHasVariables(ruleEmpty))
}

func TestSubstituteImageVerifyVariables(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	ctx := enginecontext.NewContext(jp)
	err := ctx.AddVariable("allowed", "ghcr.io/verified/*")
	require.NoError(t, err)

	rule := kyvernov1.Rule{
		VerifyImages: []kyvernov1.ImageVerification{
			{
				ImageReferences: []string{"{{ allowed }}"},
				Validation: kyvernov1.ValidateImageVerification{
					Message: "{{ image.reference }} is not signed",
					Deny: &kyvernov1.Deny{
						RawAnyAllConditions: &kyvernov1.ConditionsWrapper{
							Conditions: []kyvernov1.Condition{
								{
									RawKey: &apiextv1.JSON{Raw: []byte(`"{{ image.reference }}"`)},
								},
							},
						},
					},
				},
				Attestations: []kyvernov1.Attestation{
					{
						Conditions: []kyvernov1.AnyAllConditions{
							{
								AllConditions: []kyvernov1.Condition{
									{
										RawKey: &apiextv1.JSON{Raw: []byte(`"{{ image.path }}"`)},
									},
								},
							},
						},
					},
				},
			},
			{
				// Entry with no Validation and Deny == nil to verify null-safety
				ImageReferences: []string{"static-image:*"},
			},
		},
	}

	res, err := SubstituteImageVerifyVariables(rule, ctx, logr.Discard())
	require.NoError(t, err)
	require.NotNil(t, res)

	// ImageReferences should be substituted
	assert.Equal(t, "ghcr.io/verified/*", res.VerifyImages[0].ImageReferences[0])

	// Validation.Message must NOT be substituted early
	assert.Equal(t, "{{ image.reference }} is not signed", res.VerifyImages[0].Validation.Message)

	// Deny conditions must be preserved
	assert.NotNil(t, res.VerifyImages[0].Validation.Deny)
	assert.NotNil(t, res.VerifyImages[0].Validation.Deny.RawAnyAllConditions)
	assert.Equal(t, rule.VerifyImages[0].Validation.Deny.RawAnyAllConditions, res.VerifyImages[0].Validation.Deny.RawAnyAllConditions)

	// Attestations conditions must be preserved
	require.Len(t, res.VerifyImages[0].Attestations, 1)
	assert.NotNil(t, res.VerifyImages[0].Attestations[0].Conditions)

	// Null-safe entry should not panic and remain untouched
	assert.Nil(t, res.VerifyImages[1].Validation.Deny)
	assert.Equal(t, "static-image:*", res.VerifyImages[1].ImageReferences[0])
}

func TestSubstituteImageVerifyVariables_Error(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	ctx := enginecontext.NewContext(jp)

	rule := kyvernov1.Rule{
		VerifyImages: []kyvernov1.ImageVerification{
			{
				ImageReferences: []string{"{{ undefined_var }}"},
			},
		},
	}

	_, err := SubstituteImageVerifyVariables(rule, ctx, logr.Discard())
	require.Error(t, err)
}

func TestSubstitutePropertiesInRule(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	ctx := enginecontext.NewContext(jp)
	require.NoError(t, ctx.AddVariable("severity", "high"))

	rule := &kyvernov1.Rule{
		ReportProperties: map[string]string{
			"level": "{{ severity }}",
		},
	}

	err := SubstitutePropertiesInRule(logr.Discard(), rule, ctx)
	require.NoError(t, err)
	assert.Equal(t, "high", rule.ReportProperties["level"])

	ruleEmpty := &kyvernov1.Rule{}
	err = SubstitutePropertiesInRule(logr.Discard(), ruleEmpty, ctx)
	require.NoError(t, err)

	ruleInvalid := &kyvernov1.Rule{
		ReportProperties: map[string]string{
			"level": "{{ undefined_variable }}",
		},
	}
	err = SubstitutePropertiesInRule(logr.Discard(), ruleInvalid, ctx)
	require.Error(t, err)
}
