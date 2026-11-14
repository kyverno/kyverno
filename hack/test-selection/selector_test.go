/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestIsFallbackFile(t *testing.T) {
	tests := []struct {
		name         string
		file         string
		wantFallback bool
	}{
		{"API file", "api/kyverno/v1/clusterpolicy_types.go", true},
		{"API root", "api/kyverno/any.go", true},
		{"go.mod", "go.mod", true},
		{"go.sum", "go.sum", true},
		{"go.work", "go.work", true},
		{"vendor", "vendor/modules.txt", true},
		{"GitHub workflow", ".github/workflows/check-unit-tests.yaml", true},
		{"GitHub action", ".github/actions/go/setup/action.yaml", true},
		{"GolangCI lint", ".golangci.yml", true},
		{"Makefile", "Makefile", true},
		{"Makefile local", "Makefile.local", true},
		{"Hack script", "hack/client-wrapper/main.go", true},
		{"Scripts dir", "scripts/check-perf-regression.sh", true},
		{"Generated deepcopy", "pkg/engine/zz_generated.deepcopy.go", true},
		{"Generated client", "pkg/client/clientset/versioned/clientset.go", true},
		{"Generated CRD", "config/crds/kyverno.io_clusterpolicies.yaml", true},
		{"Self tool edit allowed", "hack/test-selection/selector.go", false},
		{"Regular pkg file", "pkg/engine/engine.go", false},
		{"Regular cmd file", "cmd/kyverno/main.go", false},
		{"Docs file", "docs/dev/proposals/test.md", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := IsFallbackFile(tc.file)
			if got != tc.wantFallback {
				t.Errorf("IsFallbackFile(%q) = %v, want %v", tc.file, got, tc.wantFallback)
			}
		})
	}
}

func TestIsSafeNonCodeFile(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		wantSafe bool
	}{
		{"Docs markdown", "docs/dev/logging.md", true},
		{"Root README", "README.md", true},
		{"Gitignore", ".gitignore", true},
		{"License", "LICENSE", true},
		{"Image asset", "docs/images/arch.png", true},
		{"Go source", "pkg/engine/engine.go", false},
		{"YAML fixture", "pkg/engine/testdata/rule.yaml", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsSafeNonCodeFile(tc.file)
			if got != tc.wantSafe {
				t.Errorf("IsSafeNonCodeFile(%q) = %v, want %v", tc.file, got, tc.wantSafe)
			}
		})
	}
}

func TestComputeSelection_ReverseDependencies(t *testing.T) {
	// Simulated module layout:
	// pkg/a -> no deps
	// pkg/b -> imports pkg/a
	// pkg/c -> imports pkg/b (transitive dep on a)
	// pkg/d -> imports pkg/e (unrelated)
	// pkg/e -> no deps

	mockPkgs := []*PackageInfo{
		{
			Dir:         filepath.Join("repo", "pkg", "a"),
			ImportPath:  "github.com/kyverno/kyverno/pkg/a",
			GoFiles:     []string{"a.go"},
			TestGoFiles: []string{"a_test.go"},
		},
		{
			Dir:         filepath.Join("repo", "pkg", "b"),
			ImportPath:  "github.com/kyverno/kyverno/pkg/b",
			GoFiles:     []string{"b.go"},
			TestGoFiles: []string{"b_test.go"},
			Imports:     []string{"github.com/kyverno/kyverno/pkg/a"},
		},
		{
			Dir:         filepath.Join("repo", "pkg", "c"),
			ImportPath:  "github.com/kyverno/kyverno/pkg/c",
			GoFiles:     []string{"c.go"},
			TestGoFiles: []string{"c_test.go"},
			Imports:     []string{"github.com/kyverno/kyverno/pkg/b"},
		},
		{
			Dir:         filepath.Join("repo", "pkg", "d"),
			ImportPath:  "github.com/kyverno/kyverno/pkg/d",
			GoFiles:     []string{"d.go"},
			TestGoFiles: []string{"d_test.go"},
			Imports:     []string{"github.com/kyverno/kyverno/pkg/e"},
		},
		{
			Dir:         filepath.Join("repo", "pkg", "e"),
			ImportPath:  "github.com/kyverno/kyverno/pkg/e",
			GoFiles:     []string{"e.go"},
			TestGoFiles: []string{"e_test.go"},
		},
	}

	cfg := SelectionConfig{
		ModuleRoot:   "repo",
		ModulePath:   "github.com/kyverno/kyverno",
		ChangedFiles: []string{"pkg/a/a.go"},
	}

	res, err := ComputeSelection(cfg, mockPkgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Fallback {
		t.Fatalf("did not expect fallback, got reason: %s", res.FallbackReason)
	}

	expected := []string{
		"github.com/kyverno/kyverno/pkg/a",
		"github.com/kyverno/kyverno/pkg/b",
		"github.com/kyverno/kyverno/pkg/c",
	}

	sort.Strings(res.AffectedPackages)
	sort.Strings(expected)

	if !reflect.DeepEqual(res.AffectedPackages, expected) {
		t.Errorf("AffectedPackages = %v, want %v", res.AffectedPackages, expected)
	}
}

func TestComputeSelection_TestOnlyModification(t *testing.T) {
	// When modifying only a_test.go, dependents (pkg/b) should NOT be selected.
	mockPkgs := []*PackageInfo{
		{
			Dir:         filepath.Join("repo", "pkg", "a"),
			ImportPath:  "github.com/kyverno/kyverno/pkg/a",
			GoFiles:     []string{"a.go"},
			TestGoFiles: []string{"a_test.go"},
		},
		{
			Dir:         filepath.Join("repo", "pkg", "b"),
			ImportPath:  "github.com/kyverno/kyverno/pkg/b",
			GoFiles:     []string{"b.go"},
			TestGoFiles: []string{"b_test.go"},
			Imports:     []string{"github.com/kyverno/kyverno/pkg/a"},
		},
	}

	cfg := SelectionConfig{
		ModuleRoot:   "repo",
		ModulePath:   "github.com/kyverno/kyverno",
		ChangedFiles: []string{"pkg/a/a_test.go"},
	}

	res, err := ComputeSelection(cfg, mockPkgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Fallback {
		t.Fatalf("did not expect fallback: %s", res.FallbackReason)
	}

	expected := []string{
		"github.com/kyverno/kyverno/pkg/a",
	}

	if !reflect.DeepEqual(res.AffectedPackages, expected) {
		t.Errorf("AffectedPackages = %v, want %v", res.AffectedPackages, expected)
	}
}

func TestComputeSelection_TestdataFixture(t *testing.T) {
	mockPkgs := []*PackageInfo{
		{
			Dir:         filepath.Join("repo", "pkg", "engine"),
			ImportPath:  "github.com/kyverno/kyverno/pkg/engine",
			GoFiles:     []string{"engine.go"},
			TestGoFiles: []string{"engine_test.go"},
		},
		{
			Dir:         filepath.Join("repo", "pkg", "webhooks"),
			ImportPath:  "github.com/kyverno/kyverno/pkg/webhooks",
			GoFiles:     []string{"webhook.go"},
			TestGoFiles: []string{"webhook_test.go"},
			Imports:     []string{"github.com/kyverno/kyverno/pkg/engine"},
		},
	}

	cfg := SelectionConfig{
		ModuleRoot:   "repo",
		ModulePath:   "github.com/kyverno/kyverno",
		ChangedFiles: []string{"pkg/engine/testdata/fixtures/policy.yaml"},
	}

	res, err := ComputeSelection(cfg, mockPkgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Fallback {
		t.Fatalf("did not expect fallback: %s", res.FallbackReason)
	}

	expected := []string{
		"github.com/kyverno/kyverno/pkg/engine",
		"github.com/kyverno/kyverno/pkg/webhooks",
	}

	sort.Strings(res.AffectedPackages)
	sort.Strings(expected)

	if !reflect.DeepEqual(res.AffectedPackages, expected) {
		t.Errorf("AffectedPackages = %v, want %v", res.AffectedPackages, expected)
	}
}

func TestComputeSelection_DocsOnlyPR(t *testing.T) {
	mockPkgs := []*PackageInfo{
		{
			Dir:         filepath.Join("repo", "pkg", "engine"),
			ImportPath:  "github.com/kyverno/kyverno/pkg/engine",
			GoFiles:     []string{"engine.go"},
			TestGoFiles: []string{"engine_test.go"},
		},
	}

	cfg := SelectionConfig{
		ModuleRoot:   "repo",
		ModulePath:   "github.com/kyverno/kyverno",
		ChangedFiles: []string{"docs/dev/proposals/foo.md", "README.md"},
	}

	res, err := ComputeSelection(cfg, mockPkgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Fallback {
		t.Fatalf("did not expect fallback for docs-only PR")
	}

	if len(res.AffectedPackages) != 0 {
		t.Errorf("expected 0 affected packages for docs PR, got %v", res.AffectedPackages)
	}
}

func TestComputeSelection_UnrecognizedFileTriggersFallback(t *testing.T) {
	mockPkgs := []*PackageInfo{
		{
			Dir:         filepath.Join("repo", "pkg", "engine"),
			ImportPath:  "github.com/kyverno/kyverno/pkg/engine",
			GoFiles:     []string{"engine.go"},
			TestGoFiles: []string{"engine_test.go"},
		},
	}

	cfg := SelectionConfig{
		ModuleRoot:   "repo",
		ModulePath:   "github.com/kyverno/kyverno",
		ChangedFiles: []string{"some_random_untracked_script.sh"},
	}

	res, err := ComputeSelection(cfg, mockPkgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.Fallback {
		t.Errorf("expected fallback for unrecognized file, got false")
	}

	if len(res.AffectedPackages) != 1 || res.AffectedPackages[0] != "github.com/kyverno/kyverno/pkg/engine" {
		t.Errorf("expected all testable packages selected on fallback, got %v", res.AffectedPackages)
	}
}

func TestFindFailingPackages(t *testing.T) {
	sampleLog := `
=== RUN   TestA
--- PASS: TestA (0.01s)
PASS
ok  	github.com/kyverno/kyverno/pkg/a	0.05s
=== RUN   TestB
--- FAIL: TestB (0.02s)
FAIL
FAIL	github.com/kyverno/kyverno/pkg/b	0.10s
=== RUN   TestC
--- FAIL: TestC (0.01s)
FAIL
FAIL	github.com/kyverno/kyverno/pkg/c	0.08s
`
	got := FindFailingPackages(sampleLog)
	want := []string{
		"github.com/kyverno/kyverno/pkg/b",
		"github.com/kyverno/kyverno/pkg/c",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("FindFailingPackages() = %v, want %v", got, want)
	}
}

func TestCompareWithFullSuite(t *testing.T) {
	res := &SelectionResult{
		AffectedPackages: []string{
			"github.com/kyverno/kyverno/pkg/a",
			"github.com/kyverno/kyverno/pkg/b",
		},
	}

	// Case 1: All failures were selected -> zero divergence
	div1 := CompareWithFullSuite(res, []string{"github.com/kyverno/kyverno/pkg/a"})
	if len(div1) != 0 {
		t.Errorf("expected 0 divergences, got %v", div1)
	}

	// Case 2: Failure occurred in pkg/c which was NOT selected -> divergence detected!
	div2 := CompareWithFullSuite(res, []string{"github.com/kyverno/kyverno/pkg/a", "github.com/kyverno/kyverno/pkg/c"})
	wantDiv2 := []string{"github.com/kyverno/kyverno/pkg/c"}
	if !reflect.DeepEqual(div2, wantDiv2) {
		t.Errorf("CompareWithFullSuite() = %v, want %v", div2, wantDiv2)
	}
}
