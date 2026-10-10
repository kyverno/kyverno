package test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONMutationFixtures(t *testing.T) {
	for _, fixture := range []string{"json-document-mutation", "json-root-transitions", "json-skips-errors"} {
		t.Run(fixture, func(t *testing.T) {
			var out bytes.Buffer
			command := Command()
			command.SetOut(&out)
			command.SetErr(&out)
			command.SetArgs([]string{"../../../../../test/cli/test-mutating-policy/" + fixture, "--require-tests", "--remove-color"})
			err := command.Execute()
			require.NoError(t, err, out.String())
			require.Contains(t, out.String(), "0 tests failed")
		})
	}
}
