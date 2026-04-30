package variables

import (
	"testing"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestCompileAttestorIdentities_NoExpressions(t *testing.T) {
	env, err := compiler.NewIdentityExprEnv()
	require.NoError(t, err)

	att := &v1beta1.Attestor{
		Name: "cosign-keyless",
		Cosign: &v1beta1.Cosign{
			Keyless: &v1beta1.Keyless{
				Identities: []v1beta1.Identity{
					{
						Issuer:  "https://token.actions.githubusercontent.com",
						Subject: "https://github.com/org/repo/.github/workflows/release.yml@refs/heads/main",
					},
				},
			},
		},
	}

	compiled, errs := CompileAttestorIdentities(field.NewPath("spec", "attestors").Index(0), att, env)
	assert.Nil(t, errs)
	assert.Nil(t, compiled) // no expressions, returns nil
}

func TestCompileAttestorIdentities_SubjectExpression(t *testing.T) {
	env, err := compiler.NewIdentityExprEnv()
	require.NoError(t, err)

	att := &v1beta1.Attestor{
		Name: "cosign-keyless",
		Cosign: &v1beta1.Cosign{
			Keyless: &v1beta1.Keyless{
				Identities: []v1beta1.Identity{
					{
						Issuer:            "https://token.actions.githubusercontent.com",
						SubjectExpression: `"https://github.com/" + image.split("/")[1] + "/.github/workflows/release.yml@refs/heads/main"`,
					},
				},
			},
		},
	}

	compiled, errs := CompileAttestorIdentities(field.NewPath("spec", "attestors").Index(0), att, env)
	assert.Nil(t, errs)
	require.NotNil(t, compiled)
	assert.Len(t, compiled.identityProgs, 1)
	assert.NotNil(t, compiled.identityProgs[0].subjectExprProg)
}

func TestCompileAttestorIdentities_InvalidExpression(t *testing.T) {
	env, err := compiler.NewIdentityExprEnv()
	require.NoError(t, err)

	att := &v1beta1.Attestor{
		Name: "cosign-keyless",
		Cosign: &v1beta1.Cosign{
			Keyless: &v1beta1.Keyless{
				Identities: []v1beta1.Identity{
					{
						Issuer:            "https://token.actions.githubusercontent.com",
						SubjectExpression: `invalid.cel.expression(((`,
					},
				},
			},
		},
	}

	compiled, errs := CompileAttestorIdentities(field.NewPath("spec", "attestors").Index(0), att, env)
	assert.NotNil(t, errs)
	assert.Nil(t, compiled)
}

func TestEvaluateWithImage_StaticSubject(t *testing.T) {
	att := &v1beta1.Attestor{
		Name: "cosign-keyless",
		Cosign: &v1beta1.Cosign{
			Keyless: &v1beta1.Keyless{
				Identities: []v1beta1.Identity{
					{
						Issuer:  "https://token.actions.githubusercontent.com",
						Subject: "https://github.com/org/repo/.github/workflows/release.yml@refs/heads/main",
					},
				},
			},
		},
	}

	// No identity expressions - EvaluateWithImage returns the attestor unchanged.
	compiled := &CompiledAttestor{
		Key: att.Name,
		val: *att,
	}

	result, err := compiled.EvaluateWithImage(nil, "ghcr.io/org/repo:v1.0.0")
	require.NoError(t, err)
	require.NotNil(t, result.Cosign.Keyless)
	assert.Equal(t,
		"https://github.com/org/repo/.github/workflows/release.yml@refs/heads/main",
		result.Cosign.Keyless.Identities[0].Subject,
	)
}

func TestEvaluateWithImage_SubjectExpression(t *testing.T) {
	env, err := compiler.NewIdentityExprEnv()
	require.NoError(t, err)

	att := &v1beta1.Attestor{
		Name: "cosign-keyless",
		Cosign: &v1beta1.Cosign{
			Keyless: &v1beta1.Keyless{
				Identities: []v1beta1.Identity{
					{
						Issuer: "https://token.actions.githubusercontent.com",
						// image = "ghcr.io/myorg/myrepo:v1.0.0"
						// image.split("/") = ["ghcr.io", "myorg", "myrepo:v1.0.0"]
						// [1] = "myorg"
						SubjectExpression: `"https://github.com/" + image.split("/")[1] + "/.github/workflows/release.yml@refs/heads/main"`,
					},
				},
			},
		},
	}

	compiled, errs := CompileAttestorIdentities(field.NewPath("spec", "attestors").Index(0), att, env)
	require.Nil(t, errs)
	require.NotNil(t, compiled)

	result, err := compiled.EvaluateWithImage(nil, "ghcr.io/myorg/myrepo:v1.0.0")
	require.NoError(t, err)
	require.NotNil(t, result.Cosign.Keyless)
	// SubjectExpression result is stored in SubjectRegExp
	assert.Equal(t,
		"https://github.com/myorg/.github/workflows/release.yml@refs/heads/main",
		result.Cosign.Keyless.Identities[0].SubjectRegExp,
	)
}

func TestEvaluateWithImage_MultipleIdentities(t *testing.T) {
	env, err := compiler.NewIdentityExprEnv()
	require.NoError(t, err)

	att := &v1beta1.Attestor{
		Name: "cosign-keyless",
		Cosign: &v1beta1.Cosign{
			Keyless: &v1beta1.Keyless{
				Identities: []v1beta1.Identity{
					{
						// Static identity - no expression, unchanged by EvaluateWithImage.
						Issuer:  "https://token.actions.githubusercontent.com",
						Subject: "https://github.com/static/repo/.github/workflows/release.yml@refs/heads/main",
					},
					{
						// Dynamic identity - expression evaluated with image.
						Issuer:            "https://token.actions.githubusercontent.com",
						SubjectExpression: `"https://github.com/" + image.split("/")[1] + "/.github/workflows/release.yml@refs/heads/main"`,
					},
				},
			},
		},
	}

	compiled, errs := CompileAttestorIdentities(field.NewPath("spec", "attestors").Index(0), att, env)
	require.Nil(t, errs)
	require.NotNil(t, compiled)

	result, err := compiled.EvaluateWithImage(nil, "ghcr.io/dynamicorg/repo:v1.0.0")
	require.NoError(t, err)
	require.NotNil(t, result.Cosign.Keyless)

	// Static identity unchanged.
	assert.Equal(t,
		"https://github.com/static/repo/.github/workflows/release.yml@refs/heads/main",
		result.Cosign.Keyless.Identities[0].Subject,
	)
	// Dynamic identity: SubjectExpression result stored in SubjectRegExp.
	assert.Equal(t,
		"https://github.com/dynamicorg/.github/workflows/release.yml@refs/heads/main",
		result.Cosign.Keyless.Identities[1].SubjectRegExp,
	)
}

func TestEvaluateWithImage_NilAttestor(t *testing.T) {
	compiled, errs := CompileAttestorIdentities(field.NewPath("spec", "attestors").Index(0), nil, nil)
	assert.Nil(t, errs)
	assert.Nil(t, compiled)
}

func TestEvaluateWithImage_NonCosignAttestor(t *testing.T) {
	env, err := compiler.NewIdentityExprEnv()
	require.NoError(t, err)

	att := &v1beta1.Attestor{
		Name: "notary",
		Notary: &v1beta1.Notary{
			Certs: &v1beta1.StringOrExpression{Value: "cert-data"},
		},
	}

	compiled, errs := CompileAttestorIdentities(field.NewPath("spec", "attestors").Index(0), att, env)
	assert.Nil(t, errs)
	assert.Nil(t, compiled)
}
