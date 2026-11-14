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
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PackageInfo holds metadata about a Go package parsed from `go list -json`.
type PackageInfo struct {
	Dir             string   `json:"Dir"`
	ImportPath      string   `json:"ImportPath"`
	Name            string   `json:"Name"`
	GoFiles         []string `json:"GoFiles"`
	TestGoFiles     []string `json:"TestGoFiles"`
	XTestGoFiles    []string `json:"XTestGoFiles"`
	EmbedFiles      []string `json:"EmbedFiles"`
	TestEmbedFiles  []string `json:"TestEmbedFiles"`
	XTestEmbedFiles []string `json:"XTestEmbedFiles"`
	Imports         []string `json:"Imports"`
	TestImports     []string `json:"TestImports"`
	XTestImports    []string `json:"XTestImports"`
}

// HasTests returns true if the package contains unit tests.
func (p *PackageInfo) HasTests() bool {
	return len(p.TestGoFiles) > 0 || len(p.XTestGoFiles) > 0
}

// SelectionConfig configures the affected packages evaluation.
type SelectionConfig struct {
	ModuleRoot   string
	ModulePath   string
	BaseRef      string
	HeadRef      string
	ChangedFiles []string
}

// SelectionResult represents the outcome of the affected packages calculation.
type SelectionResult struct {
	Fallback         bool     `json:"fallback"`
	FallbackReason   string   `json:"fallback_reason,omitempty"`
	ChangedFiles     []string `json:"changed_files"`
	DirectPackages   []string `json:"direct_packages"`
	AffectedPackages []string `json:"affected_packages"`
	TotalPackages    int      `json:"total_packages"`
	TestablePackages int      `json:"testable_packages"`
	SelectedCount    int      `json:"selected_count"`
}

// IsFallbackFile returns whether a file change must trigger fallback to the full test suite.
func IsFallbackFile(relPath string) (bool, string) {
	norm := filepath.ToSlash(filepath.Clean(relPath))

	// API definition changes require full validation across the entire engine.
	if strings.HasPrefix(norm, "api/") {
		return true, fmt.Sprintf("API definition modified: %s", norm)
	}

	// Dependency manifests require full validation.
	if norm == "go.mod" || norm == "go.sum" || norm == "go.work" || norm == "go.work.sum" || strings.HasPrefix(norm, "vendor/") {
		return true, fmt.Sprintf("dependency manifest modified: %s", norm)
	}

	// Toolchains, CI workflows, and linter configs.
	if strings.HasPrefix(norm, ".github/") || norm == ".golangci.yml" || norm == ".goreleaser.yaml" || norm == ".ko.yaml" || strings.HasPrefix(norm, "Dockerfile") {
		return true, fmt.Sprintf("CI / toolchain configuration modified: %s", norm)
	}

	// Build scripts and codegen helpers (except test-selection itself).
	if strings.HasPrefix(norm, "Makefile") || strings.HasPrefix(norm, "scripts/") {
		return true, fmt.Sprintf("build script modified: %s", norm)
	}
	if strings.HasPrefix(norm, "hack/") && !strings.HasPrefix(norm, "hack/test-selection/") {
		return true, fmt.Sprintf("hack / codegen tooling modified: %s", norm)
	}

	// Generated code changes.
	base := filepath.Base(norm)
	if strings.HasPrefix(base, "zz_generated.") ||
		strings.HasSuffix(norm, ".generated.go") ||
		strings.HasPrefix(norm, "pkg/client/") ||
		strings.HasPrefix(norm, "config/crds/") ||
		strings.HasPrefix(norm, "cmd/cli/kubectl-kyverno/config/crds/") ||
		strings.HasPrefix(norm, "cmd/cli/kubectl-kyverno/data/crds/") ||
		strings.HasPrefix(norm, "charts/kyverno/charts/crds/") {
		return true, fmt.Sprintf("generated file modified: %s", norm)
	}

	return false, ""
}

// IsSafeNonCodeFile returns true for files whose modifications do not require running unit tests.
func IsSafeNonCodeFile(relPath string) bool {
	norm := filepath.ToSlash(filepath.Clean(relPath))
	if strings.HasPrefix(norm, "docs/") || strings.HasPrefix(norm, "site/") {
		return true
	}
	if strings.HasSuffix(norm, ".md") || strings.HasSuffix(norm, ".png") || strings.HasSuffix(norm, ".jpg") || strings.HasSuffix(norm, ".svg") {
		return true
	}
	if norm == ".gitignore" || norm == ".gitattributes" || norm == "LICENSE" || norm == "OWNERS" || norm == "SECURITY.md" {
		return true
	}
	return false
}

// ParsePackageStream parses `go list -json` output into a slice of PackageInfo.
func ParsePackageStream(r io.Reader) ([]*PackageInfo, error) {
	var pkgs []*PackageInfo
	dec := json.NewDecoder(r)
	for dec.More() {
		var p PackageInfo
		if err := dec.Decode(&p); err != nil {
			return nil, err
		}
		pkgs = append(pkgs, &p)
	}
	return pkgs, nil
}

// normalizePkgDir ensures p.Dir is an absolute path anchored to absModuleRoot.
func normalizePkgDir(pkgDir, moduleRoot, absModuleRoot string) string {
	if filepath.IsAbs(pkgDir) {
		return filepath.Clean(pkgDir)
	}
	if rel, err := filepath.Rel(moduleRoot, pkgDir); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.Clean(filepath.Join(absModuleRoot, rel))
	}
	return filepath.Clean(filepath.Join(absModuleRoot, pkgDir))
}

// MapFileToPackage maps a modified relative file path to its owning Go package import path.
func MapFileToPackage(relPath string, pkgs []*PackageInfo, moduleRoot string) (string, bool) {
	absModuleRoot, err := filepath.Abs(moduleRoot)
	if err != nil {
		absModuleRoot = filepath.Clean(moduleRoot)
	}

	absTarget, err := filepath.Abs(filepath.Join(absModuleRoot, relPath))
	if err != nil {
		absTarget = filepath.Clean(filepath.Join(absModuleRoot, relPath))
	}

	// 1. Direct match on Go files or Embed files
	for _, p := range pkgs {
		absPkgDir := normalizePkgDir(p.Dir, moduleRoot, absModuleRoot)

		relToPkg, err := filepath.Rel(absPkgDir, absTarget)
		if err != nil || strings.HasPrefix(relToPkg, "..") {
			continue
		}

		relSlash := filepath.ToSlash(relToPkg)
		// Check Go source and test files
		for _, f := range p.GoFiles {
			if f == relSlash {
				return p.ImportPath, true
			}
		}
		for _, f := range p.TestGoFiles {
			if f == relSlash {
				return p.ImportPath, true
			}
		}
		for _, f := range p.XTestGoFiles {
			if f == relSlash {
				return p.ImportPath, true
			}
		}
		// Check embedded files
		for _, f := range p.EmbedFiles {
			if f == relSlash {
				return p.ImportPath, true
			}
		}
		for _, f := range p.TestEmbedFiles {
			if f == relSlash {
				return p.ImportPath, true
			}
		}
		for _, f := range p.XTestEmbedFiles {
			if f == relSlash {
				return p.ImportPath, true
			}
		}
	}

	// 2. Fixtures/testdata enclosing directory check: walk up from file dir
	fileDir := filepath.Dir(absTarget)
	for len(fileDir) >= len(absModuleRoot) {
		for _, p := range pkgs {
			absPkgDir := normalizePkgDir(p.Dir, moduleRoot, absModuleRoot)
			if absPkgDir == fileDir {
				return p.ImportPath, true
			}
		}
		parent := filepath.Dir(fileDir)
		if parent == fileDir {
			break
		}
		fileDir = parent
	}

	return "", false
}

// FindEmbeddedPackage returns the package import path and whether the embed is test-only
// if relPath matches an embedded file in any package (EmbedFiles, TestEmbedFiles, XTestEmbedFiles).
func FindEmbeddedPackage(relPath string, pkgs []*PackageInfo, moduleRoot string) (string, bool, bool) {
	absModuleRoot, err := filepath.Abs(moduleRoot)
	if err != nil {
		absModuleRoot = filepath.Clean(moduleRoot)
	}

	absTarget, err := filepath.Abs(filepath.Join(absModuleRoot, relPath))
	if err != nil {
		absTarget = filepath.Clean(filepath.Join(absModuleRoot, relPath))
	}

	for _, p := range pkgs {
		absPkgDir := normalizePkgDir(p.Dir, moduleRoot, absModuleRoot)

		relToPkg, err := filepath.Rel(absPkgDir, absTarget)
		if err != nil || strings.HasPrefix(relToPkg, "..") {
			continue
		}

		relSlash := filepath.ToSlash(relToPkg)

		// Production embeds affect package runtime/compilation and reverse dependencies
		for _, f := range p.EmbedFiles {
			if f == relSlash {
				return p.ImportPath, false, true
			}
		}
		// Test embeds only affect package tests
		for _, f := range p.TestEmbedFiles {
			if f == relSlash {
				return p.ImportPath, true, true
			}
		}
		for _, f := range p.XTestEmbedFiles {
			if f == relSlash {
				return p.ImportPath, true, true
			}
		}
	}

	return "", false, false
}

// ComputeSelection calculates affected packages based on changed files and package dependency graph.
func ComputeSelection(cfg SelectionConfig, pkgs []*PackageInfo) (*SelectionResult, error) {
	result := &SelectionResult{
		ChangedFiles:  cfg.ChangedFiles,
		TotalPackages: len(pkgs),
	}

	// Count testable packages
	testableCount := 0
	for _, p := range pkgs {
		if p.HasTests() {
			testableCount++
		}
	}
	result.TestablePackages = testableCount

	// Check fallback triggers and collect directly modified packages
	directPkgMap := make(map[string]bool)
	testOnlyDirect := make(map[string]bool)

	for _, file := range cfg.ChangedFiles {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}

		// Fallback check
		if isFallback, reason := IsFallbackFile(file); isFallback {
			result.Fallback = true
			result.FallbackReason = reason
			return selectAllTestable(result, pkgs), nil
		}

		// Check if file is an embedded asset of any package before safe non-code check
		if pkgImport, isTestOnly, ok := FindEmbeddedPackage(file, pkgs, cfg.ModuleRoot); ok {
			directPkgMap[pkgImport] = true
			if isTestOnly {
				if _, exists := testOnlyDirect[pkgImport]; !exists {
					testOnlyDirect[pkgImport] = true
				}
			} else {
				testOnlyDirect[pkgImport] = false
			}
			continue
		}

		// Safe non-code check (docs, images, ignore files not embedded in any package)
		if IsSafeNonCodeFile(file) {
			continue
		}

		// Map file to package
		pkgImport, ok := MapFileToPackage(file, pkgs, cfg.ModuleRoot)
		if !ok {
			// Unrecognized file change: fail closed
			result.Fallback = true
			result.FallbackReason = fmt.Sprintf("unrecognized non-package file modified: %s", file)
			return selectAllTestable(result, pkgs), nil
		}

		directPkgMap[pkgImport] = true
		if isTestOnlyFile(file) {
			// If all modifications to this package are test-only, reverse deps don't need re-testing
			if _, exists := testOnlyDirect[pkgImport]; !exists {
				testOnlyDirect[pkgImport] = true
			}
		} else {
			testOnlyDirect[pkgImport] = false
		}
	}

	for p := range directPkgMap {
		result.DirectPackages = append(result.DirectPackages, p)
	}
	sort.Strings(result.DirectPackages)

	// If no packages are directly modified (e.g. docs-only PR), return empty selection
	if len(result.DirectPackages) == 0 {
		return result, nil
	}

	// Build reverse dependency graphs within the module:
	// - reverseProdGraph: A imported by B in production code (transitive propagation)
	// - reverseTestGraph: A imported by B in test code only (B's tests affected, no further propagation)
	pkgByImport := make(map[string]*PackageInfo, len(pkgs))
	reverseProdGraph := make(map[string][]string)
	reverseTestGraph := make(map[string][]string)

	for _, p := range pkgs {
		pkgByImport[p.ImportPath] = p
	}

	for _, p := range pkgs {
		prodImports := make(map[string]bool)
		for _, imp := range p.Imports {
			if strings.HasPrefix(imp, cfg.ModulePath) {
				prodImports[imp] = true
				reverseProdGraph[imp] = append(reverseProdGraph[imp], p.ImportPath)
			}
		}

		testImports := make(map[string]bool)
		for _, imp := range p.TestImports {
			if strings.HasPrefix(imp, cfg.ModulePath) && !prodImports[imp] {
				testImports[imp] = true
			}
		}
		for _, imp := range p.XTestImports {
			if strings.HasPrefix(imp, cfg.ModulePath) && !prodImports[imp] {
				testImports[imp] = true
			}
		}

		for imp := range testImports {
			reverseTestGraph[imp] = append(reverseTestGraph[imp], p.ImportPath)
		}
	}

	// Traverse reverse dependencies (transitive closure)
	affectedMap := make(map[string]bool)
	queue := make([]string, 0)

	for _, p := range result.DirectPackages {
		affectedMap[p] = true
		// If changes were solely inside test files of p, reverse deps of p don't need testing
		if testOnlyDirect[p] {
			continue
		}
		queue = append(queue, p)
	}

	visited := make(map[string]bool)
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if visited[curr] {
			continue
		}
		visited[curr] = true

		// Packages whose test code depends on curr must run tests
		for _, testDependent := range reverseTestGraph[curr] {
			affectedMap[testDependent] = true
		}

		// Packages whose production code depends on curr must run tests and propagate transitively
		for _, prodDependent := range reverseProdGraph[curr] {
			affectedMap[prodDependent] = true
			if !visited[prodDependent] {
				queue = append(queue, prodDependent)
			}
		}
	}

	// Filter down to testable packages that actually have tests
	for p := range affectedMap {
		info, ok := pkgByImport[p]
		if ok && info.HasTests() {
			result.AffectedPackages = append(result.AffectedPackages, p)
		}
	}
	sort.Strings(result.AffectedPackages)
	result.SelectedCount = len(result.AffectedPackages)

	return result, nil
}

func isTestOnlyFile(file string) bool {
	norm := filepath.ToSlash(file)
	base := filepath.Base(norm)
	return strings.HasSuffix(base, "_test.go")
}

func selectAllTestable(res *SelectionResult, pkgs []*PackageInfo) *SelectionResult {
	res.AffectedPackages = nil
	for _, p := range pkgs {
		if p.HasTests() {
			res.AffectedPackages = append(res.AffectedPackages, p.ImportPath)
		}
	}
	sort.Strings(res.AffectedPackages)
	res.SelectedCount = len(res.AffectedPackages)
	return res
}

// FindFailingPackages parses `go test -v` output and returns import paths of failing packages.
func FindFailingPackages(logContent string) []string {
	var failing []string
	seen := make(map[string]bool)
	lines := strings.Split(logContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Standard `go test` output format: "FAIL\tgithub.com/kyverno/kyverno/pkg/engine\t0.123s"
		if strings.HasPrefix(line, "FAIL\t") {
			parts := strings.Split(line, "\t")
			if len(parts) >= 2 {
				pkg := strings.TrimSpace(parts[1])
				if !seen[pkg] {
					seen[pkg] = true
					failing = append(failing, pkg)
				}
			}
		}
	}
	sort.Strings(failing)
	return failing
}

// CompareWithFullSuite checks whether any packages that failed in the full suite were omitted by the selection.
func CompareWithFullSuite(res *SelectionResult, failingPkgs []string) []string {
	var divergences []string
	selectedMap := make(map[string]bool)
	for _, p := range res.AffectedPackages {
		selectedMap[p] = true
	}
	for _, f := range failingPkgs {
		if !selectedMap[f] {
			divergences = append(divergences, f)
		}
	}
	sort.Strings(divergences)
	return divergences
}

// GetGitDiffFiles returns modified files between baseRef and headRef using git diff.
func GetGitDiffFiles(moduleRoot, baseRef, headRef string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", fmt.Sprintf("%s...%s", baseRef, headRef))
	cmd.Dir = moduleRoot
	out, err := cmd.Output()
	if err != nil {
		// Try two-dot diff as fallback
		cmdFallback := exec.Command("git", "diff", "--name-only", baseRef, headRef)
		cmdFallback.Dir = moduleRoot
		outFallback, errFallback := cmdFallback.Output()
		if errFallback != nil {
			return nil, fmt.Errorf("git diff failed (%v, %v): %s", err, errFallback, string(outFallback))
		}
		out = outFallback
	}

	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// ShadowReport encapsulates selection results and full-suite comparison metrics.
type ShadowReport struct {
	Result           *SelectionResult
	Elapsed          time.Duration
	ShadowExecuted   bool
	ShadowDuration   time.Duration
	ShadowPassed     bool
	FullSuiteChecked bool
	FullSuiteFailing []string
	Divergences      []string
}

// FormatMarkdownSummary produces a markdown report suitable for GitHub Step Summary.
func FormatMarkdownSummary(report *ShadowReport) string {
	res := report.Result
	var sb strings.Builder
	sb.WriteString("### 🎯 Affected-Package Test Selection (Shadow Mode Trial)\n\n")

	if res.Fallback {
		sb.WriteString("> [!WARNING]\n")
		sb.WriteString(fmt.Sprintf("> **Fallback Triggered:** Full test suite selected.\n> Reason: `%s`\n\n", res.FallbackReason))
	} else if len(res.AffectedPackages) == 0 {
		sb.WriteString("> [!NOTE]\n")
		sb.WriteString("> No Go code changes detected. Zero packages selected for unit testing.\n\n")
	} else {
		savings := 0.0
		if res.TestablePackages > 0 {
			savings = 100.0 * (1.0 - float64(res.SelectedCount)/float64(res.TestablePackages))
		}
		sb.WriteString("> [!NOTE]\n")
		sb.WriteString(fmt.Sprintf("> Computed affected packages based on base-to-head reverse dependencies.\n> Selection represents **%.1f%% potential package reduction** (%d of %d testable packages).\n\n",
			savings, res.SelectedCount, res.TestablePackages))
	}

	if report.FullSuiteChecked {
		if len(report.Divergences) > 0 {
			sb.WriteString("> [!CAUTION]\n")
			sb.WriteString("> **Divergence Detected!** The full test suite failed in package(s) that were **omitted** by the selection:\n")
			for _, d := range report.Divergences {
				sb.WriteString(fmt.Sprintf("> - `%s`\n", d))
			}
			sb.WriteString("> This gap must be investigated before gating PRs.\n\n")
		} else {
			sb.WriteString("> [!TIP]\n")
			if len(report.FullSuiteFailing) > 0 {
				sb.WriteString(fmt.Sprintf("> **Selection Verified:** %d full-suite failure(s) observed, and all were correctly identified by the affected-package selection!\n\n", len(report.FullSuiteFailing)))
			} else {
				sb.WriteString("> **Selection Verified:** Full suite passed with zero misses from the affected-package selection.\n\n")
			}
		}
	}

	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|---|---:|\n")
	sb.WriteString(fmt.Sprintf("| Total Module Packages | %d |\n", res.TotalPackages))
	sb.WriteString(fmt.Sprintf("| Testable Packages | %d |\n", res.TestablePackages))
	sb.WriteString(fmt.Sprintf("| Selected Packages | %d |\n", res.SelectedCount))
	sb.WriteString(fmt.Sprintf("| Changed Files | %d |\n", len(res.ChangedFiles)))
	sb.WriteString(fmt.Sprintf("| Direct Packages | %d |\n", len(res.DirectPackages)))
	if report.Elapsed > 0 {
		sb.WriteString(fmt.Sprintf("| Selection Computation Time | %s |\n", report.Elapsed.Round(time.Millisecond)))
	}
	sb.WriteString("\n")

	if report.ShadowExecuted {
		sb.WriteString("#### 🧪 Shadow Test Execution\n\n")
		statusStr := "❌ FAILED"
		if report.ShadowPassed {
			statusStr = "✅ PASSED"
		}
		sb.WriteString(fmt.Sprintf("- **Status:** %s\n", statusStr))
		sb.WriteString(fmt.Sprintf("- **Duration:** %s\n", report.ShadowDuration.Round(time.Millisecond)))
		sb.WriteString(fmt.Sprintf("- **Packages Tested:** %d\n\n", len(res.AffectedPackages)))
	}

	if len(res.DirectPackages) > 0 {
		sb.WriteString("<details><summary><b>Directly Modified Packages (" + fmt.Sprint(len(res.DirectPackages)) + ")</b></summary>\n\n")
		for _, p := range res.DirectPackages {
			sb.WriteString(fmt.Sprintf("- `%s`\n", p))
		}
		sb.WriteString("\n</details>\n\n")
	}

	if len(res.AffectedPackages) > 0 && !res.Fallback {
		sb.WriteString("<details><summary><b>Affected Packages (" + fmt.Sprint(len(res.AffectedPackages)) + ")</b></summary>\n\n")
		for _, p := range res.AffectedPackages {
			sb.WriteString(fmt.Sprintf("- `%s`\n", p))
		}
		sb.WriteString("\n</details>\n\n")
	}

	return sb.String()
}
