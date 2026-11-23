package variables

import (
	"context"
	"testing"

	"github.com/google/cel-go/cel"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// subjectExpressionEnv exposes the interpolation source a policy author would
// use at admission time, e.g. a ConfigMap value resolved through globalContext.
func subjectExpressionEnv(t *testing.T) *cel.Env {
	t.Helper()
	env, err := cel.NewEnv(cel.Variable("data", cel.MapType(cel.StringType, cel.StringType)))
	require.NoError(t, err)
	return env
}

// TestSubjectExpressionBecomesSubjectRegExp pins the documented behaviour of
// Identity.SubjectExpression: the expression is evaluated at admission time and
// its result is used as the subject regexp matched against the certificate SAN
// URI. It used to be copied onto the policy but never compiled nor evaluated,
// so an identity carrying only SubjectExpression reached cosign with an empty
// subject and no subject constraint at all.
func TestSubjectExpressionBecomesSubjectRegExp(t *testing.T) {
	env := subjectExpressionEnv(t)
	attestors := []policiesv1beta1.Attestor{{
		Name: "keyless",
		Cosign: &policiesv1beta1.Cosign{
			Keyless: &policiesv1beta1.Keyless{
				Identities: []policiesv1beta1.Identity{{
					Issuer:            "https://token.actions.githubusercontent.com",
					SubjectExpression: "data.subject",
				}},
			},
		},
	}}

	compiled, errs := CompileAttestors(field.NewPath("spec", "attestors"), attestors, env)
	require.Empty(t, errs)
	require.Len(t, compiled, 1)

	got, err := compiled[0].Evaluate(context.Background(), map[string]any{
		"data": map[string]string{"subject": `^https://github\.com/allowed/repo/.*$`},
	})
	require.NoError(t, err)

	identity := got.Cosign.Keyless.Identities[0]
	assert.Equal(t, `^https://github\.com/allowed/repo/.*$`, identity.SubjectRegExp)
	assert.Empty(t, identity.Subject)
	assert.Equal(t, "data.subject", identity.SubjectExpression)
}

// TestSubjectExpressionPerIdentity checks that every identity keeps its own
// resolved subject, so a later identity cannot inherit an earlier one's value.
func TestSubjectExpressionPerIdentity(t *testing.T) {
	env := subjectExpressionEnv(t)
	attestors := []policiesv1beta1.Attestor{{
		Name: "keyless",
		Cosign: &policiesv1beta1.Cosign{
			Keyless: &policiesv1beta1.Keyless{
				Identities: []policiesv1beta1.Identity{
					{Issuer: "https://token.actions.githubusercontent.com", SubjectExpression: "data.first"},
					{Issuer: "https://accounts.google.com", Subject: "static@example.com"},
					{Issuer: "https://token.actions.githubusercontent.com", SubjectRegExp: "^static-regexp$"},
				},
			},
		},
	}}

	compiled, errs := CompileAttestors(field.NewPath("spec", "attestors"), attestors, env)
	require.Empty(t, errs)

	got, err := compiled[0].Evaluate(context.Background(), map[string]any{
		"data": map[string]string{"first": "^from-expression$"},
	})
	require.NoError(t, err)

	identities := got.Cosign.Keyless.Identities
	assert.Equal(t, "^from-expression$", identities[0].SubjectRegExp)
	assert.Empty(t, identities[1].SubjectRegExp)
	assert.Equal(t, "static@example.com", identities[1].Subject)
	assert.Equal(t, "^static-regexp$", identities[2].SubjectRegExp)
}

// TestSubjectExpressionEmptyResultFailsClosed guards the one way the fix could
// silently widen a policy: cosign reads an empty SubjectRegExp as "no subject
// constraint", so a resolved empty string must be an error, not an allow.
func TestSubjectExpressionEmptyResultFailsClosed(t *testing.T) {
	env := subjectExpressionEnv(t)
	attestors := []policiesv1beta1.Attestor{{
		Name: "keyless",
		Cosign: &policiesv1beta1.Cosign{
			Keyless: &policiesv1beta1.Keyless{
				Identities: []policiesv1beta1.Identity{{
					Issuer:            "https://token.actions.githubusercontent.com",
					SubjectExpression: "data.subject",
				}},
			},
		},
	}}

	compiled, errs := CompileAttestors(field.NewPath("spec", "attestors"), attestors, env)
	require.Empty(t, errs)

	_, err := compiled[0].Evaluate(context.Background(), map[string]any{
		"data": map[string]string{"subject": ""},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty string")
}

// TestSubjectExpressionCompileError checks that a malformed expression is
// reported when the policy is compiled, not deferred to verification time.
func TestSubjectExpressionCompileError(t *testing.T) {
	env := subjectExpressionEnv(t)
	attestors := []policiesv1beta1.Attestor{{
		Name: "keyless",
		Cosign: &policiesv1beta1.Cosign{
			Keyless: &policiesv1beta1.Keyless{
				Identities: []policiesv1beta1.Identity{{
					Issuer:            "https://token.actions.githubusercontent.com",
					SubjectExpression: "data.subject +",
				}},
			},
		},
	}}

	_, errs := CompileAttestors(field.NewPath("spec", "attestors"), attestors, env)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs.ToAggregate().Error(), "subject")
}

// TestSubjectExpressionRejectsSubjectRegExp covers the API contract that the
// two fields are mutually exclusive: both set would have to drop one silently.
func TestSubjectExpressionRejectsSubjectRegExp(t *testing.T) {
	env := subjectExpressionEnv(t)
	attestors := []policiesv1beta1.Attestor{{
		Name: "keyless",
		Cosign: &policiesv1beta1.Cosign{
			Keyless: &policiesv1beta1.Keyless{
				Identities: []policiesv1beta1.Identity{{
					Issuer:            "https://token.actions.githubusercontent.com",
					SubjectExpression: "data.subject",
					SubjectRegExp:     "^already-set$",
				}},
			},
		},
	}}

	_, errs := CompileAttestors(field.NewPath("spec", "attestors"), attestors, env)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs.ToAggregate().Error(), "mutually exclusive")
}
