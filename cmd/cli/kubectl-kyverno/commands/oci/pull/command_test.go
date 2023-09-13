package pull

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/stretchr/testify/assert"
)

var keychain = authn.NewMultiKeychain(
	authn.DefaultKeychain,
	github.Keychain,
	registryclient.AWSKeychain,
	registryclient.GCPKeychain,
	registryclient.AzureKeychain,
)

func TestCommandNoImageRef(t *testing.T) {
	cmd := Command(keychain)
	assert.NotNil(t, cmd)
	err := cmd.Execute()
	assert.Error(t, err)
}

func TestCommandWithArgs(t *testing.T) {
	cmd := Command(keychain)
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"foo"})
	err := cmd.Execute()
	assert.Error(t, err)
}
