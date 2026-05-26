package table

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	ec "github.com/kyverno/kyverno/ext/output/color"
)

// To update golden files, run: go test ./cmd/cli/kubectl-kyverno/output/table/... -args -update
var update = flag.Bool("update-table-golden", false, "update golden files")

// Test for newTableWriter functionality
func TestNewTableWriter(t *testing.T) {
	tw := newTableWriter()

	if !tw.Style().Options.DrawBorder {
		t.Error("Expected to draw border")
	}
	if tw.Style().Options.SeparateRows {
		t.Error("Expected rows to not be separated")
	}
	if !tw.Style().Options.SeparateHeader {
		t.Error("Expected header to be separated")
	}
	if tw.Style().Size.WidthMax != 300 {
		t.Error("Expected max width to be 300")
	}
}

// Table Driven Test for getHeader
func TestGetHeader(t *testing.T) {
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
	tests := []struct {
		name    string
		table   Table
		detail  bool
		wantErr bool
		wantOut string
		color   bool
	}{
		{
			name:    "empty table",
			table:   Table{},
			detail:  false,
			wantErr: false,
			wantOut: "printer_test_empty.txt",
		},
		{
			name:    "empty table - color",
			table:   Table{},
			detail:  false,
			wantErr: false,
			wantOut: "printer_test_color_empty.txt",
			color:   true,
		},
		{
			name: "single row compact",
			table: Table{
				RawRows: []Row{{ID: 1, Policy: "Policy1", Rule: "Rule1", Resource: "Resource1", Result: "Pass", Reason: "Valid"}},
			},
			detail:  false,
			wantErr: false,
			wantOut: "printer_test_single_row_compact.txt",
		},
		{
			name: "single row compact - color",
			table: Table{
				RawRows: []Row{{ID: 1, Policy: "Policy1", Rule: "Rule1", Resource: "Resource1", Result: "Pass", Reason: "Valid"}},
			},
			detail:  false,
			wantErr: false,
			wantOut: "printer_test_color_single_row_compact.txt",
			color:   true,
		},
		{
			name: "single row detailed",
			table: Table{
				RawRows: []Row{{ID: 1, Policy: "Policy1", Rule: "Rule1", Resource: "Resource1", Result: "Pass", Reason: "Valid", Message: "No issues found"}},
			},
			detail:  true,
			wantErr: false,
			wantOut: "printer_test_single_row_detailed.txt",
		},
		{
			name: "single row detailed - color",
			table: Table{
				RawRows: []Row{{ID: 1, Policy: "Policy1", Rule: "Rule1", Resource: "Resource1", Result: "Pass", Reason: "Valid", Message: "No issues found"}},
			},
			detail:  true,
			wantErr: false,
			wantOut: "printer_test_color_single_row_detailed.txt",
			color:   true,
		},
		{
			name: "multiple rows detailed - color",
			table: Table{
				RawRows: []Row{
					{ID: 1, Policy: "Policy1", Rule: "Rule1", Resource: "Resource1", Result: "Pass", Reason: "Valid", Message: "No issues found"},
					{ID: 2, Policy: "Policy2", Rule: "Rule2", Resource: "Resource2", Result: "Fail", Reason: "Invalid", Message: "Error detected"},
					{ID: 3, Policy: "Policy3", Rule: "Rule3", Resource: "Resource3", Result: "Warn", Reason: "Warning", Message: "This is a very long warning message that contains extensive details about the warning condition. The warning has been triggered due to multiple policy violations detected during the validation process. This includes but is not limited to configuration mismatches, potential security vulnerabilities, deprecated API usage, missing required annotations, insufficient resource quotas, non-compliant naming conventions, and various other issues that require immediate attention from the operations team to prevent potential runtime failures or security breaches in the production environment."},
				},
			},
			detail:  true,
			wantErr: false,
			wantOut: "printer_test_color_multiple_rows_detailed.txt",
			color:   true,
		},
		{
			name:    "expect error",
			table:   Table{},
			detail:  true,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			originalColorState := ec.Enabled()
			t.Cleanup(func() {
				ec.Force(originalColorState)
			})
			ec.Force(tc.color)

			if tc.wantErr {
				err := Print(&errorWriter{}, tc.table, tc.detail)
				if err == nil {
					t.Fatalf("Print() expected error, got nil")
				}
				return
			}
			var buf bytes.Buffer
			err := Print(&buf, tc.table, tc.detail)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Print() error = %v, wantErr %v", err, tc.wantErr)
			}

			goldenPath := filepath.Join("testdata", tc.wantOut)
			gotBytes := buf.Bytes()
			// Normalize line endings for cross-platform compatibility
			gotBytes = bytes.ReplaceAll(gotBytes, []byte("\r\n"), []byte("\n"))

			if *update {
				// Create directory if it doesn't exist
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
					t.Fatalf("Failed to create golden file directory: %v", err)
				}
				// Update golden file
				if err := os.WriteFile(goldenPath, gotBytes, 0644); err != nil {
					t.Fatalf("Failed to update golden file %s: %v", tc.wantOut, err)
				}
				t.Logf("Updated golden file: %s", tc.wantOut)
				return
			}

			wantOutBytes, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("Failed to read expected output file %s: %v (run with -update-table-golden flag to create it)", tc.wantOut, err)
			}

			// Normalize line endings for cross-platform compatibility
			wantOutBytes = bytes.ReplaceAll(wantOutBytes, []byte("\r\n"), []byte("\n"))

			if !bytes.Equal(gotBytes, wantOutBytes) {
				t.Errorf("Output mismatch for %s\nExpected:\n%s\nGot:\n%s\n\nRun 'go test ./cmd/cli/kubectl-kyverno/output/table/... -args -update-table-golden' to update golden files",
					tc.wantOut, string(wantOutBytes), string(gotBytes))
			}
		})
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (n int, err error) {
	return 0, errors.New("expected error")
}
