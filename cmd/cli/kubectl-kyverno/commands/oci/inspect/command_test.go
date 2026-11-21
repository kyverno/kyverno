package inspect

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/stretchr/testify/assert"
)

func TestCommandRequiresImage(t *testing.T) {
	cmd := Command(authn.DefaultKeychain)
	assert.NotNil(t, cmd)
	err := cmd.Execute()
	assert.Error(t, err)
}

func TestCommandRejectsInvalidOutputFlag(t *testing.T) {
	cmd := Command(authn.DefaultKeychain)
	cmd.SetArgs([]string{"-i", "example.com/repo/test:v1", "-o", "xml"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid output format")
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
