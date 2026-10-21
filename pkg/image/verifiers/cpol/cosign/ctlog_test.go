package cosign

import (
	"context"
	"crypto/x509"
	"errors"
	"testing"

	"github.com/kyverno/kyverno/pkg/image/verifiers"
	"github.com/kyverno/sdk/extensions/registryclient"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubCTLogPubs(t *testing.T) *int {
	t.Helper()
	calls := 0
	orig := ctLogPubsFn
	ctLogPubsFn = func(context.Context, string) (*cosign.TrustedTransparencyLogPubKeys, error) {
		calls++
		return nil, errors.New("ct log keys requested")
	}
	t.Cleanup(func() { ctLogPubsFn = orig })
	return &calls
}

func TestBuildCosignOptions_KeyedIgnoreTlogSkipsCTLogKeys(t *testing.T) {
	calls := stubCTLogPubs(t)

	opts := verifiers.Options{
		Client:     registryclient.New(),
		Key:        wrongPubKey,
		IgnoreTlog: true,
		IgnoreSCT:  false,
	}
	co, err := buildCosignOptions(context.TODO(), opts)
	require.NoError(t, err)
	assert.Equal(t, 0, *calls)
	assert.NotNil(t, co.SigVerifier)
	assert.Nil(t, co.CTLogPubKeys)
}

func TestBuildCosignOptions_KeylessStillRequiresCTLogKeys(t *testing.T) {
	calls := stubCTLogPubs(t)

	root, _ := generateTestRootCA(t)
	opts := verifiers.Options{
		Client:     registryclient.New(),
		Roots:      string(certsToPEM([]*x509.Certificate{root})),
		IgnoreTlog: true,
		IgnoreSCT:  false,
	}
	_, err := buildCosignOptions(context.TODO(), opts)
	require.ErrorContains(t, err, "failed to load CTLogs public keys")
	assert.Equal(t, 1, *calls)
}

func TestBuildCosignOptions_KeylessIgnoreSCTSkipsCTLogKeys(t *testing.T) {
	calls := stubCTLogPubs(t)

	root, _ := generateTestRootCA(t)
	opts := verifiers.Options{
		Client:     registryclient.New(),
		Roots:      string(certsToPEM([]*x509.Certificate{root})),
		IgnoreTlog: true,
		IgnoreSCT:  true,
	}
	co, err := buildCosignOptions(context.TODO(), opts)
	require.NoError(t, err)
	assert.Equal(t, 0, *calls)
	assert.True(t, co.IgnoreSCT)
}

func TestBuildCosignOptions_PinnedCertSkipsCTLogKeys(t *testing.T) {
	calls := stubCTLogPubs(t)

	root, _ := generateTestRootCA(t)
	opts := verifiers.Options{
		Client:     registryclient.New(),
		Cert:       string(certsToPEM([]*x509.Certificate{root})),
		IgnoreTlog: true,
		IgnoreSCT:  false,
	}
	co, err := buildCosignOptions(context.TODO(), opts)
	require.NoError(t, err)
	assert.Equal(t, 0, *calls)
	assert.NotNil(t, co.SigVerifier)
	assert.Nil(t, co.CTLogPubKeys)
}

func TestBuildCosignOptions_CertChainRootsStillRequiresCTLogKeys(t *testing.T) {
	calls := stubCTLogPubs(t)

	root, _ := generateTestRootCA(t)
	opts := verifiers.Options{
		Client:     registryclient.New(),
		CertChain:  string(certsToPEM([]*x509.Certificate{root})),
		IgnoreTlog: true,
		IgnoreSCT:  false,
	}
	_, err := buildCosignOptions(context.TODO(), opts)
	require.ErrorContains(t, err, "failed to load CTLogs public keys")
	assert.Equal(t, 1, *calls)
}
