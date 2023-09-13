package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommand(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"request.object.metadata.name | truncate(@, `9`)"})
	err := cmd.Execute()
	assert.NoError(t, err)
}
