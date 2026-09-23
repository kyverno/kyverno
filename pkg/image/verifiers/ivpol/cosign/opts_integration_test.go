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
	err := initializeTuf(ctx, nil)
	require.NoError(t, err)
}

func TestInitializeTuf_WithCustomMirror(t *testing.T) {
	ctx := testContext(t)
	tufCfg := &v1beta1.TUF{
		Mirror: "https://custom-tuf.example.com",
	}
	err := initializeTuf(ctx, tufCfg)
	assert.Error(t, err)
}

func TestGetRekor_WithURL(t *testing.T) {
	ctx := testContext(t)
	err := initializeTuf(ctx, nil)
	require.NoError(t, err)
	tr, err := getTrustedRootFromTUF(ctx, nil)
	require.NoError(t, err)
	ctlog := &v1beta1.CTLog{
		URL: "https://rekor.sigstore.dev",
	}
	rekorClient, rekorPubKeys, ctlogPubKeys, err := getRekor(ctx, ctlog, tr)
	require.NoError(t, err)
	assert.NotNil(t, rekorClient)
	assert.NotNil(t, rekorPubKeys)
	assert.NotNil(t, ctlogPubKeys)
}

func TestGetRekor_NilCTLog(t *testing.T) {
	ctx := testContext(t)
	err := initializeTuf(ctx, nil)
	require.NoError(t, err)
	tr, err := getTrustedRootFromTUF(ctx, nil)
	require.NoError(t, err)
	rekorClient, rekorPubKeys, ctlogPubKeys, err := getRekor(ctx, nil, tr)
	require.NoError(t, err)
	assert.Nil(t, rekorClient)
	assert.NotNil(t, rekorPubKeys)
	assert.NotNil(t, ctlogPubKeys)
}

func TestGetFulcio(t *testing.T) {
	ctx := testContext(t)
	err := initializeTuf(ctx, nil)
	require.NoError(t, err)
	tr, err := getTrustedRootFromTUF(ctx, nil)
	require.NoError(t, err)
	roots, intermediates, err := getFulcio(ctx, tr)
	require.NoError(t, err)
	assert.NotNil(t, roots)
	assert.NotNil(t, intermediates)
}

func TestGetTrustedRootFromTUF(t *testing.T) {
	ctx := testContext(t)
	err := initializeTuf(ctx, nil)
	require.NoError(t, err)
	trustedRoot, err := getTrustedRootFromTUF(ctx, nil)
	require.NoError(t, err)
	assert.NotNil(t, trustedRoot)
}
