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
	require.NoError(t, os.WriteFile(file, []byte(source), 0644))
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

func TestCompareFields(t *testing.T) {
	tests := []struct {
		name         string
		sourceFields map[string]struct{}
		targetFields map[string]struct{}
		sourceName   string
		targetName   string
		expectPass   bool
	}{
		{
			name: "all fields present",
			sourceFields: map[string]struct{}{
				"Spec":   {},
				"Status": {},
			},
			targetFields: map[string]struct{}{
				"Spec":   {},
				"Status": {},
				"Extra":  {},
			},
			sourceName: "v1.Policy",
			targetName: "v2.Policy",
			expectPass: true,
		},
		{
			name: "missing field in target",
			sourceFields: map[string]struct{}{
				"Spec":   {},
				"Status": {},
			},
			targetFields: map[string]struct{}{
				"Spec": {},
			},
			sourceName: "v1.Policy",
			targetName: "v2.Policy",
			expectPass: false,
		},
		{
			name:         "empty source",
			sourceFields: map[string]struct{}{},
			targetFields: map[string]struct{}{
				"Spec": {},
			},
			sourceName: "v1.Policy",
			targetName: "v2.Policy",
			expectPass: true,
		},
		{
			name: "empty target with source fields",
			sourceFields: map[string]struct{}{
				"Spec": {},
			},
			targetFields: map[string]struct{}{},
			sourceName:   "v1.Policy",
			targetName:   "v2.Policy",
			expectPass:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareFields(tt.sourceFields, tt.targetFields, tt.sourceName, tt.targetName)
			if tt.expectPass {
				require.True(t, result, "Expected comparison to pass")
			} else {
				require.False(t, result, "Expected comparison to fail")
			}
		})
	}
}

func TestPolicyTypeComparisons(t *testing.T) {
	comparisons := []policyTypeComparison{
		{
			name:       "Policy",
			sourcePath: "../../api/kyverno/v1/policy_types.go",
			sourceAPI:  "v1",
			targetPath: "../../api/kyverno/v2beta1/policy_types.go",
			targetAPI:  "v2beta1",
			structName: "Policy",
		},
		{
			name:       "ClusterPolicy",
			sourcePath: "../../api/kyverno/v1/clusterpolicy_types.go",
			sourceAPI:  "v1",
			targetPath: "../../api/kyverno/v2beta1/clusterpolicy_types.go",
			targetAPI:  "v2beta1",
			structName: "ClusterPolicy",
		},
		{
			name:       "CleanupPolicy",
			sourcePath: "../../api/kyverno/v2/cleanup_policy_types.go",
			sourceAPI:  "v2",
			targetPath: "../../api/kyverno/v2beta1/cleanup_policy_types.go",
			targetAPI:  "v2beta1",
			structName: "CleanupPolicy",
		},
		{
			name:       "ClusterCleanupPolicy",
			sourcePath: "../../api/kyverno/v2/cleanup_policy_types.go",
			sourceAPI:  "v2",
			targetPath: "../../api/kyverno/v2beta1/cleanup_policy_types.go",
			targetAPI:  "v2beta1",
			structName: "ClusterCleanupPolicy",
		},
		{
			name:       "PolicyException",
			sourcePath: "../../api/kyverno/v2/policy_exception_types.go",
			sourceAPI:  "v2",
			targetPath: "../../api/kyverno/v2beta1/policy_exception_types.go",
			targetAPI:  "v2beta1",
			structName: "PolicyException",
		},
	}

	for _, comp := range comparisons {
		t.Run(comp.name, func(t *testing.T) {
			_, err := os.Stat(comp.sourcePath)
			require.NoError(t, err, "Source file should exist: %s", comp.sourcePath)

			_, err = os.Stat(comp.targetPath)
			require.NoError(t, err, "Target file should exist: %s", comp.targetPath)

			sourceFields := getFieldSet(comp.sourcePath, comp.structName)
			require.NotEmpty(t, sourceFields, "Source struct %s should have fields", comp.structName)

			targetFields := getFieldSet(comp.targetPath, comp.structName)
			require.NotEmpty(t, targetFields, "Target struct %s should have fields", comp.structName)

			allPresent := compareFields(sourceFields, targetFields,
				comp.sourceAPI+"."+comp.structName,
				comp.targetAPI+"."+comp.structName)
			require.True(t, allPresent,
				"All fields from %s.%s should be present in %s.%s",
				comp.sourceAPI, comp.structName, comp.targetAPI, comp.structName)
		})
	}
}
