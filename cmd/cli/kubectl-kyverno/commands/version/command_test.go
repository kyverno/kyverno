package version

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/pkg/version"
	"github.com/stretchr/testify/assert"
)

func TestCommand(t *testing.T) {
	version.BuildVersion = "test"
	cmd := Command()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	err := cmd.Execute()
	assert.NoError(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `
Version: test
Time: ---
Git commit ID: ---`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
}

func TestCommandWithArgs(t *testing.T) {
	cmd := Command()
	cmd.SetArgs([]string{"test"})
	err := cmd.Execute()
	assert.Error(t, err)
}
