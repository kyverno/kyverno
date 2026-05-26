package table

import (
	"testing"

	pt "github.com/jedib0t/go-pretty/v6/table"
	"github.com/stretchr/testify/assert"
)

func TestRow_forTable(t *testing.T) {
	tests := []struct {
		name     string
		row      Row
		detail   bool
		expected pt.Row
	}{
		{
			name: "Without detail",
			row: Row{
				IsFailure: false,
				ID:        1,
				Policy:    "policy-1",
				Rule:      "rule-1",
				Resource:  "resource-1",
				Result:    "pass",
				Reason:    "reason-1",
				Message:   "message-1",
			},
			detail:   false,
			expected: pt.Row{1, "policy-1", "rule-1", "resource-1", "pass", "reason-1"},
		},
		{
			name: "With detail included",
			row: Row{
				IsFailure: false,
				ID:        2,
				Policy:    "policy-2",
				Rule:      "rule-2",
				Resource:  "resource-2",
				Result:    "fail",
				Reason:    "reason-2",
				Message:   "detailed message",
			},
			detail:   true,
			expected: pt.Row{2, "policy-2", "rule-2", "resource-2", "fail", "reason-2", "detailed message"},
		},
		{
			name: "Empty row without detail",
			row: Row{
				IsFailure: false,
				ID:        0,
				Policy:    "",
				Rule:      "",
				Resource:  "",
				Result:    "",
				Reason:    "",
				Message:   "",
			},
			detail:   false,
			expected: pt.Row{0, "", "", "", "", ""},
		},
		{
			name: "Empty row with detail",
			row: Row{
				IsFailure: false,
				ID:        0,
				Policy:    "",
				Rule:      "",
				Resource:  "",
				Result:    "",
				Reason:    "",
				Message:   "",
			},
			detail:   true,
			expected: pt.Row{0, "", "", "", "", "", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.row.forTable(tt.detail)
			assert.Equal(t, tt.expected, result)
		})
	}
}
