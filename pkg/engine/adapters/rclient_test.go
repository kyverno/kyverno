package adapters

import (
	"context"
	"fmt"
	"testing"

	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRclientAdapter_ForRef_Integration(t *testing.T) {
	client, err := registryclient.New()
	require.NoError(t, err)

	adapter := RegistryClient(client)
	require.NotNil(t, adapter)

	ctx := context.Background()
	result, err := adapter.ForRef(ctx, "docker.io/library/hello-world:latest")

	if err != nil {
		t.Skipf("Skipping test due to network error: %v", err)
		return
	}

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "docker.io/library/hello-world:latest", result.Image)
	assert.Contains(t, result.ResolvedImage, "docker.io/library/hello-world@sha256:")
	assert.Equal(t, "index.docker.io", result.Registry) // Docker Hub uses index.docker.io
	assert.Equal(t, "library/hello-world", result.Repository)
	assert.Equal(t, "latest", result.Identifier)

	assert.NotNil(t, result.Manifest)
	assert.NotNil(t, result.Config)
}

func TestRclientAdapter_ForRef_InvalidImage(t *testing.T) {
	client, err := registryclient.New()
	require.NoError(t, err)

	adapter := RegistryClient(client)
	require.NotNil(t, adapter)

	ctx := context.Background()
	result, err := adapter.ForRef(ctx, "invalid-image-reference")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to fetch image descriptor")
}

func TestRclientAdapter_ForRef_NonExistentImage(t *testing.T) {
	client, err := registryclient.New()
	require.NoError(t, err)

	adapter := RegistryClient(client)
	require.NotNil(t, adapter)

	ctx := context.Background()
	result, err := adapter.ForRef(ctx, "docker.io/library/non-existent-image:latest")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to fetch image descriptor")
}

func TestRclientAdapter_ForRef_WithDigest(t *testing.T) {
	client, err := registryclient.New()
	require.NoError(t, err)

	adapter := RegistryClient(client)
	require.NotNil(t, adapter)

	ctx := context.Background()
	result, err := adapter.ForRef(ctx, "docker.io/library/hello-world@sha256:74cc54b2b37c2c4b41bb10dc6422d6072d469509f2f22f1a3ce74f4a59661d34")

	if err != nil {
		t.Skipf("Skipping test due to network error: %v", err)
		return
	}

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "docker.io/library/hello-world@sha256:74cc54b2b37c2c4b41bb10dc6422d6072d469509f2f22f1a3ce74f4a59661d34", result.Image)
	assert.Equal(t, "docker.io/library/hello-world@sha256:74cc54b2b37c2c4b41bb10dc6422d6072d469509f2f22f1a3ce74f4a59661d34", result.ResolvedImage)
	assert.Equal(t, "index.docker.io", result.Registry)
	assert.Equal(t, "library/hello-world", result.Repository)
	assert.Equal(t, "sha256:74cc54b2b37c2c4b41bb10dc6422d6072d469509f2f22f1a3ce74f4a59661d34", result.Identifier)
}

func TestRclientAdapter_ForRef_WithTagAndDigest(t *testing.T) {
	client, err := registryclient.New()
	require.NoError(t, err)

	adapter := RegistryClient(client)
	require.NotNil(t, adapter)

	ctx := context.Background()

	result, err := adapter.ForRef(ctx, "docker.io/library/alpine:latest@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1")

	if err != nil {
		t.Skipf("Skipping test due to network error: %v", err)
		return
	}

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "docker.io/library/alpine:latest@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1", result.Image)
	assert.Equal(t, "index.docker.io", result.Registry)
	assert.Equal(t, "library/alpine", result.Repository)
	assert.Equal(t, "latest@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1", result.Identifier)
}

func TestRclientAdapter_ForRef_IdentifierParsingEdgeCases(t *testing.T) {
	client, err := registryclient.New()
	require.NoError(t, err)

	adapter := RegistryClient(client)
	require.NotNil(t, adapter)

	ctx := context.Background()

	tests := []struct {
		name           string
		imageRef       string
		expectedTag    string
		expectedDigest string
		description    string
	}{
		{
			name:           "No tag before digest",
			imageRef:       "docker.io/library/alpine@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1",
			expectedTag:    "",
			expectedDigest: "sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1",
			description:    "When no colon before @, should not extract tag",
		},
		{
			name:           "Tag with underscore",
			imageRef:       "docker.io/library/alpine:my_tag@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1",
			expectedTag:    "my_tag",
			expectedDigest: "sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1",
			description:    "Should handle tags with underscores",
		},
		{
			name:           "Tag with dash",
			imageRef:       "docker.io/library/alpine:my-tag@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1",
			expectedTag:    "my-tag",
			expectedDigest: "sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1",
			description:    "Should handle tags with dashes",
		},
		{
			name:           "Tag with dots",
			imageRef:       "docker.io/library/alpine:v1.2.3@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1",
			expectedTag:    "v1.2.3",
			expectedDigest: "sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1",
			description:    "Should handle tags with dots",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := adapter.ForRef(ctx, tt.imageRef)
			if err != nil {
				t.Skipf("Skipping test due to network error: %v", err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.expectedTag != "" {
				expectedIdentifier := fmt.Sprintf("%s@%s", tt.expectedTag, tt.expectedDigest)
				assert.Equal(t, expectedIdentifier, result.Identifier, tt.description)
			} else {
				assert.Equal(t, tt.expectedDigest, result.Identifier, tt.description)
			}
		})
	}
}
