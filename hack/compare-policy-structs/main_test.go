package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTempFile(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "temp.go")
	require.NoError(t, os.WriteFile(file, []byte(source), 0o644))
	return file
}

func TestGetFieldSet(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		structName     string
		expectedFields []string
		skippedFields  []string
	}{
		{
			name: "simple struct with deprecated field",
			source: `
				package testpkg

				type Policy struct {
					Spec string
					// Deprecated: this field should be ignored
					Status string
					Ready bool
				}
			`,
			structName:     "Policy",
			expectedFields: []string{"Spec", "Ready"},
			skippedFields:  []string{"Status"},
		},
		{
			name: "only deprecated field",
			source: `
				package testpkg

				type Policy struct {
					// Deprecated: legacy field
					Old string
				}
			`,
			structName:     "Policy",
			expectedFields: []string{},
			skippedFields:  []string{"Old"},
		},
		{
			name: "struct with embedded field (should skip)",
			source: `
				package testpkg

				type Metadata struct {
					Name string
				}

				type Policy struct {
					Metadata
					Spec string
					// Deprecated: remove soon
					Old string
				}
			`,
			structName:     "Policy",
			expectedFields: []string{"Spec"},
			skippedFields:  []string{"Old"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTempFile(t, tt.source)
			fields := getFieldSet(path, tt.structName)

			for _, f := range tt.expectedFields {
				require.Contains(t, fields, f, "Expected field %q to be present", f)
			}

			for _, f := range tt.skippedFields {
				require.NotContains(t, fields, f, "Expected field %q to be skipped", f)
			}
		})
	}
}
