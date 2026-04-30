package test

import (
	"bytes"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
	"github.com/stretchr/testify/assert"
)

func Test_printFailures(t *testing.T) {
	rows := []table.Row{
		{
			RowCompact: table.RowCompact{
				ID:        1,
				IsFailure: true,
			},
			Message: "failure message 1",
		},
		{
			RowCompact: table.RowCompact{
				ID:        2,
				IsFailure: false,
			},
			Message: "success message",
		},
		{
			RowCompact: table.RowCompact{
				ID:        3,
				IsFailure: true,
			},
			Message: "failure message 2",
		},
		{
			RowCompact: table.RowCompact{
				ID:        4,
				IsFailure: true,
			},
			Message: "",
		},
	}

	t.Run("detailed results", func(t *testing.T) {
		out := &bytes.Buffer{}
		printFailures(out, rows, true)
		assert.Empty(t, out.String())
	})

	t.Run("not detailed results", func(t *testing.T) {
		out := &bytes.Buffer{}
		printFailures(out, rows, false)
		expected := "\nFailure 1: failure message 1\n\nFailure 3: failure message 2\n"
		assert.Equal(t, expected, out.String())
	})
}
