package cosign

import (
	"context"
	"os"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/test/integration/framework/imagetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	offlineTestIssuer  = imagetest.DefaultIssuer
	offlineTestSubject = imagetest.DefaultSubject
)

// newPrivateSigstore returns a virtual Sigstore instance and its trusted root
// as JSON, shaped like GitHub's private instance: a Fulcio CA and a TSA, with
// no transparency or CT log.
func newPrivateSigstore(t *testing.T) (*imagetest.VirtualSigstore, string) {
	t.Helper()
	vs := imagetest.NewVirtualSigstore(t)
	return vs, vs.TrustedRootJSON(t)
}

// composeTrustedRoot returns a trusted root as JSON with the Fulcio CAs of
// casFrom and the TSAs of tsasFrom (nil for none), and no transparency or CT
// log.
func composeTrustedRoot(t *testing.T, casFrom, tsasFrom *imagetest.VirtualSigstore) string {
	t.Helper()
	return imagetest.ComposeTrustedRootJSON(t, casFrom, tsasFrom)
}

// signKeylessBundle builds a v0.3 bundle the way a private Sigstore instance
// does: a Fulcio leaf certificate and an RFC 3161 timestamp, no tlog entry.
func signKeylessBundle(t *testing.T, vs *imagetest.VirtualSigstore, subjectDigestHex string) []byte {
	t.Helper()
	return vs.SignKeylessBundle(t, subjectDigestHex, offlineTestSubject, offlineTestIssuer, imagetest.DefaultPredicateType)
}

func offlineKeylessAttestor(trustedRoot *v1beta1.StringOrExpression) *v1beta1.Attestor {
	return &v1beta1.Attestor{Name: "private-sigstore", Cosign: &v1beta1.Cosign{
		Keyless: &v1beta1.Keyless{Identities: []v1beta1.Identity{{
			Issuer:  offlineTestIssuer,
			Subject: offlineTestSubject,
		}}},
		CTLog:       &v1beta1.CTLog{InsecureIgnoreTlog: true, InsecureIgnoreSCT: true},
		TrustedRoot: trustedRoot,
	}}
}

// Test_KeylessInlineTrustedRoot_VerifiesWithoutTUF covers #17883 end to end: a
// keyless bundle from a private Sigstore instance verifies against the inline
// trusted root while every TUF access fails, as on a cluster without egress.
func Test_KeylessInlineTrustedRoot_VerifiesWithoutTUF(t *testing.T) {
	vs, trustedRoot := newPrivateSigstore(t)
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	pushReferrer(t, repo, subject, signKeylessBundle(t, vs, subject.Digest.Hex), testBundleMediaType, testBundleMediaType)
	img := fetchTestImage(t, repo.Tag("subject").String())
	attestation := &v1beta1.Attestation{Name: "slsa", InToto: &v1beta1.InToto{Type: imagetest.DefaultPredicateType}}

	t.Run("signature and attestation verify against the inline root", func(t *testing.T) {
		tufCalls := stubTufUnreachable(t)
		v := NewVerifier(nil, logr.Discard())
		attestor := offlineKeylessAttestor(&v1beta1.StringOrExpression{Value: trustedRoot})
		require.NoError(t, v.VerifyImageSignature(context.TODO(), img, attestor))
		require.NoError(t, v.VerifyAttestationSignature(context.TODO(), img, attestation, attestor))
		require.Zero(t, tufCalls.Load())
	})

	other, otherRoot := newPrivateSigstore(t)
	refusals := []struct {
		name        string
		root        string
		errContains string
	}{
		{
			// The signer's TSA is trusted, so the refusal comes from the chain.
			name:        "another instance's Fulcio CA",
			root:        composeTrustedRoot(t, other, vs),
			errContains: "leaf certificate verification failed",
		},
		{
			name:        "another instance's root",
			root:        otherRoot,
			errContains: "threshold not met for verified signed timestamps",
		},
		{
			name:        "root with CAs but no TSAs, tlog ignored",
			root:        composeTrustedRoot(t, vs, nil),
			errContains: "threshold not met for verified signed timestamps",
		},
		{
			name:        "mediaType-only root",
			root:        `{"mediaType":"application/vnd.dev.sigstore.trustedroot+json;version=0.1"}`,
			errContains: "threshold not met for verified signed timestamps",
		},
	}
	for _, tt := range refusals {
		t.Run("refused: "+tt.name, func(t *testing.T) {
			tufCalls := stubTufUnreachable(t)
			v := NewVerifier(nil, logr.Discard())
			attestor := offlineKeylessAttestor(&v1beta1.StringOrExpression{Value: tt.root})

			err := v.VerifyImageSignature(context.TODO(), img, attestor)
			require.ErrorContains(t, err, tt.errContains)
			assert.NotContains(t, err.Error(), "TUF")

			err = v.VerifyAttestationSignature(context.TODO(), img, attestation, attestor)
			require.ErrorContains(t, err, tt.errContains)
			assert.NotContains(t, err.Error(), "TUF")
			assert.Zero(t, tufCalls.Load())
		})
	}

	t.Run("without an inline root TUF is still required", func(t *testing.T) {
		tufCalls := stubTufUnreachable(t)
		v := NewVerifier(nil, logr.Discard())
		err := v.VerifyImageSignature(context.TODO(), img, offlineKeylessAttestor(nil))
		require.ErrorContains(t, err, "failed to initialize TUF client")
		require.NotZero(t, tufCalls.Load())
	})
}

// signLegacyKeyless attaches a cosign v2 (non-bundle) keyless signature, a
// Fulcio leaf certificate from vs with no tlog entry, to the image at tag.
func signLegacyKeyless(t *testing.T, vs *imagetest.VirtualSigstore, repo name.Repository, tag string) {
	t.Helper()
	vs.SignLegacyKeyless(t, repo, tag, offlineTestSubject, offlineTestIssuer)
}

// Test_LegacyKeylessSignature_InlineTrustedRoot covers the non-bundle path: a
// keyless .sig signature is checked against the inline root's Fulcio CAs, and
// a root without CAs refuses it instead of falling back to TUF's Fulcio roots.
func Test_LegacyKeylessSignature_InlineTrustedRoot(t *testing.T) {
	vs, trustedRoot := newPrivateSigstore(t)
	repo, _ := newBundleRegistry(t)
	pushSubject(t, repo, "subject")
	signLegacyKeyless(t, vs, repo, "subject")
	img := fetchTestImage(t, repo.Tag("subject").String())

	t.Run("verifies against a root with the issuing CA", func(t *testing.T) {
		tufCalls := stubTufUnreachable(t)
		v := NewVerifier(nil, logr.Discard())
		require.NoError(t, v.VerifyImageSignature(context.TODO(), img, offlineKeylessAttestor(&v1beta1.StringOrExpression{Value: trustedRoot})))
		require.Zero(t, tufCalls.Load())
	})

	t.Run("refused by a root without CAs", func(t *testing.T) {
		tufCalls := stubTufUnreachable(t)
		v := NewVerifier(nil, logr.Discard())
		attestor := offlineKeylessAttestor(&v1beta1.StringOrExpression{Value: composeTrustedRoot(t, nil, vs)})
		err := v.VerifyImageSignature(context.TODO(), img, attestor)
		require.ErrorContains(t, err, "leaf certificate verification failed")
		assert.NotContains(t, err.Error(), "TUF")
		require.Zero(t, tufCalls.Load())
	})
}

// Test_KeyAttestor_TlogRequired_InlineTrustedRootWithoutTlogs covers a key
// attestor that requires a tlog entry with an inline root that has no tlogs:
// it is not a configuration error, but the signature is still refused at
// verification and TUF is never contacted.
func Test_KeyAttestor_TlogRequired_InlineTrustedRootWithoutTlogs(t *testing.T) {
	githubJSON, err := os.ReadFile("testdata/github-trusted-root.json")
	require.NoError(t, err)
	repo, _ := newBundleRegistry(t)
	pushSubject(t, repo, "subject")
	pubPEM := signV2(t, repo, "subject")
	img := fetchTestImage(t, repo.Tag("subject").String())

	tufCalls := stubTufUnreachable(t)
	v := NewVerifier(nil, logr.Discard())
	attestor := &v1beta1.Attestor{Name: "key", Cosign: &v1beta1.Cosign{
		Key:         &v1beta1.Key{Data: pubPEM},
		TrustedRoot: &v1beta1.StringOrExpression{Value: string(githubJSON)},
	}}
	err = v.VerifyImageSignature(context.TODO(), img, attestor)
	require.ErrorContains(t, err, "rekor client not provided")
	assert.NotContains(t, err.Error(), "TUF")
	assert.Zero(t, tufCalls.Load())
}
