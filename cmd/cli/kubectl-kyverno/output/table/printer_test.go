package table

import (
	"bytes"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
)

// Test for newTableWriter functionality
func TestNewTableWriter(t *testing.T) {
	tw := newTableWriter()

	if tw.Style().Options.DrawBorder != true {
		t.Error("Expected to draw border")
	}
	if tw.Style().Options.SeparateRows != false {
		t.Error("Expected rows to not be separated")
	}
	if tw.Style().Options.SeparateHeader != true {
		t.Error("Expected header to be separated")
	}
	if tw.Style().Size.WidthMax != 300 {
		t.Error("Expected max width to be 300")
	}
}

// Table Driven Test for getHeader
func TestGetHeader(t *testing.T) {
	color.Init(true)
	tests := []struct {
		name        string
		detail      bool
		wantColumns []any
	}{
		{"compact", false, []any{"ID", "POLICY", "RULE", "RESOURCE", "RESULT", "REASON"}},
		{"detailed", true, []any{"ID", "POLICY", "RULE", "RESOURCE", "RESULT", "REASON", "MESSAGE"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			header := getHeader(tc.detail)
			if len(header) != len(tc.wantColumns) {
				t.Fatalf("Expected %d columns, got %d", len(tc.wantColumns), len(header))
			}

			for i, col := range header {
				if col != tc.wantColumns[i] {
					t.Errorf("Expected column %v, got %v", tc.wantColumns[i], col)
				}
			}
		})
	}
}

// Table Driven Test for Print
func TestPrint(t *testing.T) {
	color.Init(true)
	tests := []struct {
		name    string
		table   Table
		detail  bool
		wantErr bool
		wantOut string
	}{
		{
			name:    "empty table",
			table:   Table{},
			detail:  false,
			wantErr: false,
			wantOut: `╭────┬────────┬──────┬──────────┬────────┬────────╮
│ ID │ POLICY │ RULE │ RESOURCE │ RESULT │ REASON │
├────┼────────┼──────┼──────────┼────────┼────────┤
╰────┴────────┴──────┴──────────┴────────┴────────╯`,
		},
		{
			name: "single row compact",
			table: Table{
				RawRows: []Row{{ID: 1, Policy: "Policy1", Rule: "Rule1", Resource: "Resource1", Result: "Pass", Reason: "Valid"}},
			},
			detail:  false,
			wantErr: false,
			wantOut: `╭────┬─────────┬───────┬───────────┬────────┬────────╮
│ ID │ POLICY  │ RULE  │ RESOURCE  │ RESULT │ REASON │
├────┼─────────┼───────┼───────────┼────────┼────────┤
│  1 │ Policy1 │ Rule1 │ Resource1 │ Pass   │ Valid  │
╰────┴─────────┴───────┴───────────┴────────┴────────╯`,
		},
		{
			name: "single row detailed",
			table: Table{
				RawRows: []Row{{ID: 1, Policy: "Policy1", Rule: "Rule1", Resource: "Resource1", Result: "Pass", Reason: "Valid", Message: "No issues found"}},
			},
			detail:  true,
			wantErr: false,
			wantOut: `╭────┬─────────┬───────┬───────────┬────────┬────────┬─────────────────╮
│ ID │ POLICY  │ RULE  │ RESOURCE  │ RESULT │ REASON │ MESSAGE         │
├────┼─────────┼───────┼───────────┼────────┼────────┼─────────────────┤
│  1 │ Policy1 │ Rule1 │ Resource1 │ Pass   │ Valid  │ No issues found │
╰────┴─────────┴───────┴───────────┴────────┴────────┴─────────────────╯`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := Print(&buf, tc.table, tc.detail)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Print() error = %v, wantErr %v", err, tc.wantErr)
			}
			if gotOut := buf.String(); gotOut != tc.wantOut {
				t.Errorf("Expected output:\n%s\nGot:\n%s", tc.wantOut, gotOut)
			}
		})
	}
}
