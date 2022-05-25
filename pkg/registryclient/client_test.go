package registryclient

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"gotest.tools/assert"
)

// Make sure that client conforms Client interface.
var _ Client = &client{}

func TestInitClientWithEmptyOptions(t *testing.T) {
	expClient := &client{
		transport: remote.DefaultTransport,
	}
	c, err := InitClient()
	assert.NilError(t, err)
	assert.Assert(t, expClient.transport == c.Transport())
	assert.Assert(t, c.Keychain() != nil)
}

func TestInitClientWithInsecureRegistryOption(t *testing.T) {
	expClient := &client{
		transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	c, err := InitClient(WithAllowInsecureRegistry())

	expInsecureSkipVerify := expClient.transport.TLSClientConfig.InsecureSkipVerify
	gotInsecureSkipVerify := c.Transport().TLSClientConfig.InsecureSkipVerify

	assert.NilError(t, err)
	assert.Assert(t, expInsecureSkipVerify == gotInsecureSkipVerify)
	assert.Assert(t, c.Keychain() != nil)
}
