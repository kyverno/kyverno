package registryclient

import (
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
	assert.Assert(t, c.getKeychain() != nil)
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
	assert.Assert(t, c.getKeychain() != nil)
}
