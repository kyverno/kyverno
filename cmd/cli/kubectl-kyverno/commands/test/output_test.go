package test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
	"github.com/stretchr/testify/assert"
)

func TestPrintDiffs(t *testing.T) {
	tests := []struct {
		name            string
		rows            []table.Row
		removeColor     bool
		wantEmpty       bool
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "failed row with diff message prints output",
			rows: []table.Row{
				{
					RowCompact: table.RowCompact{
						IsFailure: true,
						Policy:    "set-foo-annotation",
						Rule:      "set-annotation",
						Resource:  "default/Pod/sample-pod",
						Reason:    resourceDiffReason,
					},
					Message: "Patched resource didn't match\n- foo: bar\n+ foo: baz",
				},
			},
			wantContains: []string{
				"set-foo-annotation",
				"set-annotation",
				"default/Pod/sample-pod",
				"Patched resource didn't match",
			},
		},
		{
			name: "passed row produces no output",
			rows: []table.Row{
				{
					RowCompact: table.RowCompact{
						IsFailure: false,
						Policy:    "my-policy",
						Rule:      "my-rule",
						Reason:    resourceDiffReason,
					},
					Message: "some message",
				},
			},
			wantEmpty: true,
		},
		{
			name: "failed non-diff row produces no output",
			rows: []table.Row{
				{
					RowCompact: table.RowCompact{
						IsFailure: true,
						Policy:    "my-policy",
						Rule:      "my-rule",
						Reason:    "Want pass, got fail",
					},
					Message: "Resource Default/Pod/sample-pod fails rule check",
				},
			},
			wantEmpty: true,
		},
		{
			name: "failed row with empty message produces no output",
			rows: []table.Row{
				{
					RowCompact: table.RowCompact{
						IsFailure: true,
						Policy:    "my-policy",
						Rule:      "my-rule",
						Reason:    resourceDiffReason,
					},
					Message: "",
				},
			},
			wantEmpty: true,
		},
		{
			name: "multiple failed rows each print their diff",
			rows: []table.Row{
				{
					RowCompact: table.RowCompact{IsFailure: true, Policy: "policy-a", Rule: "rule-a", Resource: "Pod/ns/pod-a", Reason: resourceDiffReason},
					Message:    "diff for pod-a",
				},
				{
					RowCompact: table.RowCompact{IsFailure: false, Policy: "policy-b", Rule: "rule-b", Reason: resourceDiffReason},
					Message:    "should not appear",
				},
				{
					RowCompact: table.RowCompact{IsFailure: true, Policy: "policy-c", Rule: "rule-c", Resource: "Pod/ns/pod-c", Reason: resourceDiffReason},
					Message:    "diff for pod-c",
				},
			},
			wantContains: []string{"diff for pod-a", "diff for pod-c"},
		},
		{
			name:        "removeColor strips ANSI codes from output",
			removeColor: true,
			rows: []table.Row{
				{
					RowCompact: table.RowCompact{IsFailure: true, Policy: "my-policy", Rule: "my-rule", Resource: "Pod/ns/my-pod", Reason: resourceDiffReason},
					Message:    "\x1b[31mred diff\x1b[0m",
				},
			},
			wantContains:    []string{"red diff"},
			wantNotContains: []string{"\x1b["},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printDiffs(&buf, tt.rows, tt.removeColor)
			out := buf.String()
			if tt.wantEmpty {
				assert.Empty(t, out)
				return
			}
			assert.NotEmpty(t, out)
			for _, want := range tt.wantContains {
				assert.True(t, strings.Contains(out, want), "expected output to contain %q, got:\n%s", want, out)
			}
			for _, notWant := range tt.wantNotContains {
				assert.False(t, strings.Contains(out, notWant), "expected output to not contain %q, got:\n%s", notWant, out)
			}
		})
	}
}

func TestPrintFailedTestResult(t *testing.T) {
	t.Run("prints diffs for failed resource-diff rows when detailedResults is false", func(t *testing.T) {
		var buf bytes.Buffer
		resultsTable := table.Table{
			RawRows: []table.Row{
				{
					RowCompact: table.RowCompact{
						IsFailure: true,
						Policy:    "my-policy",
						Rule:      "my-rule",
						Resource:  "default/Pod/my-pod",
						Reason:    resourceDiffReason,
						Result:    "Fail",
					},
					Message: "- cpu: 200m\n+ cpu: 100m",
				},
			},
		}
		printFailedTestResult(&buf, resultsTable, false, true)
		out := buf.String()
		assert.Contains(t, out, "my-policy")
		assert.Contains(t, out, "cpu: 200m")
	})

	t.Run("does not print diff header when detailedResults is true", func(t *testing.T) {
		var buf bytes.Buffer
		resultsTable := table.Table{
			RawRows: []table.Row{
				{
					RowCompact: table.RowCompact{
						IsFailure: true,
						Policy:    "my-policy",
						Rule:      "my-rule",
						Resource:  "default/Pod/my-pod",
						Reason:    resourceDiffReason,
						Result:    "Fail",
					},
					Message: "- cpu: 200m\n+ cpu: 100m",
				},
			},
		}
		printFailedTestResult(&buf, resultsTable, true, true)
		out := buf.String()
		// With detailedResults=true, printDiffs is skipped — the "--- policy/rule/resource ---" header should not appear
		assert.NotContains(t, out, "--- my-policy / my-rule / default/Pod/my-pod ---")
	})
}
