//go:build integration

package cosign

import (
	"context"
	"testing"
	"time"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestInitializeTuf_Default(t *testing.T) {
	ctx := testContext(t)
	trust, err := initTUFAndFetch(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, trust)
}

func TestInitializeTuf_WithCustomMirror(t *testing.T) {
	ctx := testContext(t)
	tufCfg := &v1beta1.TUF{
		Mirror: "https://custom-tuf.example.com",
	}
	_, err := initTUFAndFetch(ctx, tufCfg)
	assert.Error(t, err)
}

func TestIntegrationGetRekor_WithURL(t *testing.T) {
	ctx := testContext(t)
	trust, err := initTUFAndFetch(ctx, nil)
	require.NoError(t, err)
	ctlog := &v1beta1.CTLog{
		URL: "https://rekor.sigstore.dev",
	}
	rekorClient, rekorPubKeys, ctlogPubKeys, err := getRekor(ctx, ctlog, trust.rekorPubKeys, trust.ctlogPubKeys)
	require.NoError(t, err)
	assert.NotNil(t, rekorClient)
	assert.NotNil(t, rekorPubKeys)
	assert.NotNil(t, ctlogPubKeys)
}

func TestIntegrationGetRekor_NilCTLog(t *testing.T) {
	ctx := testContext(t)
	trust, err := initTUFAndFetch(ctx, nil)
	require.NoError(t, err)
	rekorClient, rekorPubKeys, ctlogPubKeys, err := getRekor(ctx, nil, trust.rekorPubKeys, trust.ctlogPubKeys)
	require.NoError(t, err)
	assert.Nil(t, rekorClient)
	assert.NotNil(t, rekorPubKeys)
	assert.NotNil(t, ctlogPubKeys)
}

func TestGetFulcio(t *testing.T) {
	ctx := testContext(t)
	trust, err := initTUFAndFetch(ctx, nil)
	require.NoError(t, err)
	assert.NotNil(t, trust.fulcioRoots)
	assert.NotNil(t, trust.fulcioIntermediates)
}

func TestGetVerificationMaterialFromTUF(t *testing.T) {
	ctx := testContext(t)
	material, err := getVerificationMaterialFromTUF(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, material)
	assert.NotNil(t, material.TrustedRoot)
}
