package docs

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestCommandWithNilRoot(t *testing.T) {
	cmd := Command(nil)
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"-o", "foo"})
	err := cmd.Execute()
	assert.Error(t, err)
}

func TestCommandWithoutArgs(t *testing.T) {
	cmd := Command(&cobra.Command{})
	assert.NotNil(t, cmd)
	err := cmd.Execute()
	assert.Error(t, err)
}
