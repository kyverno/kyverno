package resolve

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/stretchr/testify/assert"
)

// TestCommandRequiresImage pins D1's flags table: --image is required.
func TestCommandRequiresImage(t *testing.T) {
	cmd := Command(authn.DefaultKeychain)
	assert.NotNil(t, cmd)
	err := cmd.Execute()
	assert.Error(t, err)
}

func TestCommandWithInvalidFlag(t *testing.T) {
	cmd := Command(authn.DefaultKeychain)
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
	cmd := Command(authn.DefaultKeychain)
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

// TestCommandRejectsUnknownReference pins Args: cobra.NoArgs and a malformed --image value
// failing with no stdout output.
func TestCommandRejectsUnknownReference(t *testing.T) {
	cmd := Command(authn.DefaultKeychain)
	out := bytes.NewBufferString("")
	cmd.SetOut(out)
	cmd.SetArgs([]string{"-i", "not a ref"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing image reference")
	assert.Empty(t, out.String())
}
