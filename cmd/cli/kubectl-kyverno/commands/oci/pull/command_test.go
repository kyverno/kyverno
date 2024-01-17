package pull

import (
	"bytes"
	"io"
	"strings"
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

func TestCommandWithInvalidArg(t *testing.T) {
	cmd := Command(keychain)
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	err := cmd.Execute()
	assert.Error(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `Error: accepts 1 arg(s), received 0`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
}

func TestCommandWithInvalidFlag(t *testing.T) {
	cmd := Command(keychain)
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.SetArgs([]string{"--xxx"})
	err := cmd.Execute()
	assert.Error(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `Error: unknown flag: --xxx`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
}

func TestCommandHelp(t *testing.T) {
	cmd := Command(keychain)
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	assert.NoError(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(out), cmd.Long))
}
