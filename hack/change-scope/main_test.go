package main

import (
	"reflect"
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		wantCategory Category
		wantAutoOK   bool
	}{
		// Generated code is matched before the broader api/ and source
		// rules that would otherwise claim these paths.
		{"generated clientset", "pkg/client/clientset/versioned/clientset.go", CategoryGenerated, false},
		{"generated client wrapper", "pkg/clients/kyverno/clientset.go", CategoryGenerated, false},
		{"generated crd", "config/crds/kyverno/kyverno.io_clusterpolicies.yaml", CategoryGenerated, false},
		{"generated deepcopy under api", "api/kyverno/v1/zz_generated.deepcopy.go", CategoryGenerated, false},

		{"api types", "api/kyverno/v1/policy_types.go", CategoryAPI, false},

		{"image verifier", "pkg/image/verifiers/ivpol/notary/verifier.go", CategorySecuritySensitive, false},
		{"sigstore tuf", "pkg/sigstoretuf/tuf.go", CategorySecuritySensitive, false},
		{"tls", "pkg/tls/renewer.go", CategorySecuritySensitive, false},

		{"unit test", "pkg/engine/engine_test.go", CategoryTest, true},
		{"conformance test", "test/conformance/chainsaw/foo/chainsaw-test.yaml", CategoryTest, true},

		{"docs dir", "docs/dev/controllers.md", CategoryDocs, true},
		{"markdown anywhere", "pkg/webhooks/AGENTS.md", CategoryDocs, true},

		{"chart", "charts/kyverno/values.yaml", CategoryChart, false},
		{"ci workflow", ".github/workflows/tests.yaml", CategoryCI, false},

		{"makefile", "Makefile", CategoryBuild, false},
		{"go.mod", "go.mod", CategoryBuild, false},

		{"ordinary source", "pkg/engine/engine.go", CategorySource, false},
		{"cmd source", "cmd/kyverno/main.go", CategorySource, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.path)
			if got.Category != tt.wantCategory {
				t.Errorf("Classify(%q).Category = %q, want %q", tt.path, got.Category, tt.wantCategory)
			}
			if got.AutonomousOK != tt.wantAutoOK {
				t.Errorf("Classify(%q).AutonomousOK = %v, want %v", tt.path, got.AutonomousOK, tt.wantAutoOK)
			}
			if got.Reason == "" {
				t.Errorf("Classify(%q) returned an empty reason", tt.path)
			}
		})
	}
}

func TestAnalyzeDocsOnlyIsAutonomous(t *testing.T) {
	got := Analyze([]string{"docs/a.md", "README.md"})
	if got.Scope != "docs-only" {
		t.Errorf("Scope = %q, want %q", got.Scope, "docs-only")
	}
	if !got.AutonomousOK {
		t.Error("AutonomousOK = false, want true for a docs-only change")
	}
	if len(got.RequiredChecks) != 0 {
		t.Errorf("RequiredChecks = %v, want none for docs", got.RequiredChecks)
	}
}

func TestAnalyzeMixedIsNotAutonomous(t *testing.T) {
	got := Analyze([]string{"docs/a.md", "pkg/engine/engine.go"})
	if got.Scope != "mixed" {
		t.Errorf("Scope = %q, want %q", got.Scope, "mixed")
	}
	if got.AutonomousOK {
		t.Error("AutonomousOK = true, want false when any path requires review")
	}
	want := []string{"lint", "test-unit"}
	if !reflect.DeepEqual(got.RequiredChecks, want) {
		t.Errorf("RequiredChecks = %v, want %v", got.RequiredChecks, want)
	}
}

// A diff touching only generated code should ask for codegen verification
// rather than unit tests, since generated packages have no tests of their own.
func TestAnalyzeGeneratedRequiresVerifyCodegen(t *testing.T) {
	got := Analyze([]string{"pkg/client/clientset/versioned/clientset.go"})
	if got.Scope != "generated-only" {
		t.Errorf("Scope = %q, want %q", got.Scope, "generated-only")
	}
	want := []string{"verify-codegen"}
	if !reflect.DeepEqual(got.RequiredChecks, want) {
		t.Errorf("RequiredChecks = %v, want %v", got.RequiredChecks, want)
	}
}

func TestAnalyzeCategoriesAreDeduplicatedAndSorted(t *testing.T) {
	got := Analyze([]string{"pkg/engine/a.go", "docs/a.md", "pkg/engine/b.go", "docs/b.md"})
	want := []Category{CategoryDocs, CategorySource}
	if !reflect.DeepEqual(got.Categories, want) {
		t.Errorf("Categories = %v, want %v", got.Categories, want)
	}
	if len(got.Files) != 4 {
		t.Errorf("len(Files) = %d, want 4", len(got.Files))
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	got := Analyze(nil)
	if got.Scope != "empty" {
		t.Errorf("Scope = %q, want %q", got.Scope, "empty")
	}
	if got.AutonomousOK {
		t.Error("AutonomousOK = true, want false for an empty change set")
	}
}
