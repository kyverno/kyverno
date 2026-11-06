package validate

import (
	"bytes"
	"io"
	"strings"
	"testing"

	ecr "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/kyverno/sdk/extensions/regcreds"
	"github.com/stretchr/testify/assert"
)

var keychain = authn.NewMultiKeychain(
	authn.DefaultKeychain,
	github.Keychain,
	authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(io.Discard))),
	google.Keychain,
	regcreds.AzureKeychain,
)

// TestCommandNoArgsNoImageFails pins D1's mutual-exclusion rule: neither a positional path nor
// --image is an error, not a silent no-op.
func TestCommandNoArgsNoImageFails(t *testing.T) {
	cmd := Command(keychain)
	assert.NotNil(t, cmd)
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "a bundle directory, file, or --image reference is required")
}

// TestCommandBothArgAndImageFails is TestCommandNoArgsNoImageFails's sibling: a positional path
// together with --image is also rejected, not silently preferring one.
func TestCommandBothArgAndImageFails(t *testing.T) {
	cmd := Command(keychain)
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"some-dir", "-i", "example.com/repo/test:v1"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "specify either a bundle directory or file, or --image, not both")
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

// TestCommandTooManyArgsFails pins Args: cobra.MaximumNArgs(1): two positional args is a cobra-
// level error, before options.validate ever runs.
func TestCommandTooManyArgsFails(t *testing.T) {
	cmd := Command(keychain)
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"dir-one", "dir-two"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts at most 1 arg")
}
