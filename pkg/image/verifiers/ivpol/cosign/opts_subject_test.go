package cosign

import (
	"context"
	"testing"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckOptions_KeylessSubjectRegExpFromResolvedAttestor pins the hand-off
// between the attestor evaluator and the verifier: once SubjectExpression has
// been evaluated into SubjectRegExp, checkOptions must carry it into the cosign
// identity. Without that, cosign sees no subject constraint at all and accepts
// any subject on the issuer.
func TestCheckOptions_KeylessSubjectRegExpFromResolvedAttestor(t *testing.T) {
	stubTufWithFixture(t)
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	resolvedSubject := `^https://github\.com/allowed/repo/\.github/workflows/build\.yml@refs/heads/main$`
	cosignCfg := &v1beta1.Cosign{
		Keyless: &v1beta1.Keyless{
			Identities: []v1beta1.Identity{
				{
					Issuer:            testIssuer,
					SubjectExpression: "data.subject",
					// set by the attestor evaluator before verification
					SubjectRegExp: resolvedSubject,
				},
			},
		},
		CTLog: &v1beta1.CTLog{
			URL:               "https://rekor.sigstore.dev",
			InsecureIgnoreSCT: true,
		},
	}

	opts, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
	require.NoError(t, err)
	require.Len(t, opts.Identities, 1)
	assert.Equal(t, testIssuer, opts.Identities[0].Issuer)
	assert.Equal(t, resolvedSubject, opts.Identities[0].SubjectRegExp)
	assert.Empty(t, opts.Identities[0].Subject)
}
