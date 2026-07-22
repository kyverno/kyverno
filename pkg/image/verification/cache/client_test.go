package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCacheSetGetStoresVerifiedDigest(t *testing.T) {
	t.Parallel()

	client, err := New(
		WithCacheEnableFlag(true),
		WithMaxSize(1000),
		WithTTLDuration(time.Hour),
	)
	require.NoError(t, err)

	policy := &metav1.ObjectMeta{
		UID:             "policy-uid",
		ResourceVersion: "1",
	}
	const (
		ruleName = "verify"
		imageRef = "ghcr.io/acme/app:signed"
		digest   = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	)

	set, err := client.Set(context.Background(), policy, ruleName, imageRef, digest, true)
	require.NoError(t, err)
	assert.True(t, set)

	found, cachedDigest, err := client.Get(context.Background(), policy, ruleName, imageRef, true)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, digest, cachedDigest)
}

func TestCacheSetRejectsEmptyDigest(t *testing.T) {
	t.Parallel()

	client, err := New(
		WithCacheEnableFlag(true),
		WithMaxSize(1000),
		WithTTLDuration(time.Hour),
	)
	require.NoError(t, err)

	policy := &metav1.ObjectMeta{UID: "policy-uid", ResourceVersion: "1"}
	set, err := client.Set(context.Background(), policy, "verify", "ghcr.io/acme/app:signed", "", true)
	require.NoError(t, err)
	assert.False(t, set)

	found, digest, err := client.Get(context.Background(), policy, "verify", "ghcr.io/acme/app:signed", true)
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, digest)
}
