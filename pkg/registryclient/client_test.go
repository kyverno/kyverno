package registryclient

import (
	"context"
	"crypto/tls"
	"net/http"
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

	tagDesc, err := c.FetchImageDescriptor(context.Background(), "ghcr.io/kyverno/test-verify-image:signed-keyless")
	assert.NilError(t, err)
	assert.Equal(t, tagDesc.Digest.String(), "sha256:445a99db22e9add9bfb15ddb1980861a329e5dff5c88d7eec9cbf08b6b2f4eb1")

	digestDesc, err := c.FetchImageDescriptor(context.Background(), "ghcr.io/kyverno/test-verify-image@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105")
	assert.NilError(t, err)
	assert.Equal(t, digestDesc.Digest.String(), "sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105")
}
