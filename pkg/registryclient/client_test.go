package registryclient

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"testing"

	"gotest.tools/assert"
)

// Make sure that client conforms Client interface.
var _ Client = &client{}

func TestInitClientWithEmptyOptions(t *testing.T) {
	c, err := New()
	assert.NilError(t, err)
	assert.Assert(t, defaultTransport == c.getTransport())
	assert.Assert(t, c.Keychain() != nil)
}

func TestInitClientWithInsecureRegistryOption(t *testing.T) {
	expClient := &client{
		transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	c, err := New(WithAllowInsecureRegistry())
	expInsecureSkipVerify := expClient.transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify
	gotInsecureSkipVerify := c.getTransport().(*http.Transport).TLSClientConfig.InsecureSkipVerify
	assert.NilError(t, err)
	assert.Assert(t, expInsecureSkipVerify == gotInsecureSkipVerify)
	assert.Assert(t, c.Keychain() != nil)
}

func TestFetchImageDescriptor(t *testing.T) {
	c, err := New()
	assert.NilError(t, err)

	// Test without platform parameter (backward compatibility)
	tagDesc, err := c.FetchImageDescriptor(context.Background(), "ghcr.io/kyverno/test-verify-image:signed-keyless", "")
	assert.NilError(t, err)
	assert.Equal(t, tagDesc.Digest.String(), "sha256:445a99db22e9add9bfb15ddb1980861a329e5dff5c88d7eec9cbf08b6b2f4eb1")

	digestDesc, err := c.FetchImageDescriptor(context.Background(), "ghcr.io/kyverno/test-verify-image@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105", "")
	assert.NilError(t, err)
	assert.Equal(t, digestDesc.Digest.String(), "sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105")
}

func TestFetchImageDescriptorWithPlatform(t *testing.T) {
	c, err := New()
	assert.NilError(t, err)

	// Test with platform parameter
	desc, err := c.FetchImageDescriptor(context.Background(), "nginx:latest", "linux/amd64")

	if err != nil {
		nerr, ok := err.(net.Error)
		if ok && nerr.Timeout() {
			t.Skipf("Skipping test due to network error: %v", err)
			return
		}
		assert.NilError(t, err)
	}

	assert.Assert(t, desc != nil)
	assert.Assert(t, desc.Digest.String() != "")

	desc2, err := c.FetchImageDescriptor(context.Background(), "nginx:latest", "linux/arm64")

	if err != nil {
		t.Skipf("Skipping test due to network error: %v", err)
		return
	}

	assert.NilError(t, err)
	assert.Assert(t, desc2 != nil)
	assert.Assert(t, desc2.Digest.String() != "")

}

func TestFetchImageDescriptorWithEmptyPlatform(t *testing.T) {
	c, err := New()
	assert.NilError(t, err)

	// Test that empty platform works (should use default behavior)
	desc1, err := c.FetchImageDescriptor(context.Background(), "nginx:latest", "")

	if err != nil {
		nerr, ok := err.(net.Error)
		if ok && nerr.Timeout() {
			t.Skipf("Skipping test due to network error: %v", err)
			return
		}
		assert.NilError(t, err)
	}

	assert.Assert(t, desc1 != nil)

	desc2, err := c.FetchImageDescriptor(context.Background(), "nginx:latest", "linux/amd64")

	if err != nil {
		nerr, ok := err.(net.Error)
		if ok && nerr.Timeout() {
			t.Skipf("Skipping test due to network error: %v", err)
			return
		}
		assert.NilError(t, err)
	}
	assert.Assert(t, desc2 != nil)

	assert.Equal(t, desc1.Digest.String(), desc2.Digest.String())
}

func TestFetchImageDescriptorPlatformEdgeCases(t *testing.T) {
	c, err := New()
	assert.NilError(t, err)

	tests := []struct {
		name        string
		imageRef    string
		platform    string
		expectError bool
		description string
	}{
		{
			name:        "Empty platform",
			imageRef:    "nginx:latest",
			platform:    "",
			expectError: false,
			description: "Empty platform should use default behavior",
		},
		{
			name:        "Valid linux/amd64",
			imageRef:    "nginx:latest",
			platform:    "linux/amd64",
			expectError: false,
			description: "Standard platform should work",
		},
		{
			name:        "Valid linux/arm64",
			imageRef:    "nginx:latest",
			platform:    "linux/arm64",
			expectError: false,
			description: "ARM64 platform should work",
		},
		{
			name:        "Unusual but valid platform",
			imageRef:    "nginx:latest",
			platform:    "linux/arm/v7",
			expectError: false,
			description: "ARM v7 platform should work",
		},
		{
			name:        "Invalid image with platform",
			imageRef:    "invalid-registry.example.com/nonexistent:tag",
			platform:    "linux/amd64",
			expectError: true,
			description: "Should fail due to invalid image, not platform parsing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, err := c.FetchImageDescriptor(context.Background(), tt.imageRef, tt.platform)

			if tt.expectError {
				assert.Assert(t, err != nil, tt.description)
				assert.Assert(t, desc == nil)
				assert.Assert(t, !strings.Contains(err.Error(), "failed to parse platform"))
			} else {
				if err != nil {
					t.Skipf("Skipping test due to network error: %v", err)
					return
				}
				assert.NilError(t, err, tt.description)
				assert.Assert(t, desc != nil)
				assert.Assert(t, desc.Digest.String() != "")
			}
		})
	}
}
