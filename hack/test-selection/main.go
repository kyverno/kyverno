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
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	var (
		baseRef      = flag.String("base", "", "Git base ref (e.g. origin/main)")
		headRef      = flag.String("head", "HEAD", "Git head ref")
		files        = flag.String("files", "", "Comma-separated list of changed files (overrides git diff)")
		filesFrom    = flag.String("files-from", "", "Path to file containing newline-separated changed files")
		format       = flag.String("format", "list", "Output format: list, lines, json, summary")
		summaryFile  = flag.String("summary-file", "", "Optional path to append markdown summary to (e.g. GITHUB_STEP_SUMMARY)")
		outFile      = flag.String("out", "", "Output file path (default stdout)")
		moduleRoot   = flag.String("module-root", ".", "Root directory of the Go module")
		modulePath   = flag.String("module-path", "github.com/kyverno/kyverno", "Go module import path")
		goBinary     = flag.String("go-binary", "go", "Path to go executable")
		runTests     = flag.Bool("run-tests", false, "Execute go test on the selected packages in shadow mode")
		testTimeout  = flag.String("test-timeout", "15m", "Timeout flag to pass to go test")
		checkFullLog = flag.String("check-full-suite-log", "", "Path to full suite go test log file to compare against")
	)
	flag.Parse()

	start := time.Now()

	rootAbs, err := filepath.Abs(*moduleRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving module root: %v\n", err)
		os.Exit(1)
	}

	// 1. Determine changed files
	var changedFiles []string
	if *files != "" {
		for _, f := range strings.Split(*files, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				changedFiles = append(changedFiles, f)
			}
		}
	} else if *filesFrom != "" {
		data, err := os.ReadFile(*filesFrom)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading files-from (%s): %v\n", *filesFrom, err)
			os.Exit(1)
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				changedFiles = append(changedFiles, line)
			}
		}
	} else if *baseRef != "" {
		var err error
		changedFiles, err = GetGitDiffFiles(rootAbs, *baseRef, *headRef)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting git diff files: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintln(os.Stderr, "Error: must specify either -base, -files, or -files-from")
		os.Exit(1)
	}

	// 2. Fetch package metadata via `go list -json ./...`
	listCmd := exec.Command(*goBinary, "list", "-json", "./...")
	listCmd.Dir = rootAbs
	listOut, err := listCmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing go list -json ./...: %v\n", err)
		os.Exit(1)
	}

	pkgs, err := ParsePackageStream(bytes.NewReader(listOut))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing package stream: %v\n", err)
		os.Exit(1)
	}

	// 3. Compute affected packages
	cfg := SelectionConfig{
		ModuleRoot:   rootAbs,
		ModulePath:   *modulePath,
		BaseRef:      *baseRef,
		HeadRef:      *headRef,
		ChangedFiles: changedFiles,
	}

	result, err := ComputeSelection(cfg, pkgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error computing affected packages: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(start)

	// 4. If -run-tests is set, execute the shadow test run
	var testDuration time.Duration
	var testPassed bool
	var testOutput string
	if *runTests && len(result.AffectedPackages) > 0 {
		testStart := time.Now()
		testArgs := []string{"test", "-v", "-race", "-covermode", "atomic", fmt.Sprintf("-timeout=%s", *testTimeout)}
		testArgs = append(testArgs, result.AffectedPackages...)

		cmd := exec.Command(*goBinary, testArgs...)
		cmd.Dir = rootAbs
		out, err := cmd.CombinedOutput()
		testDuration = time.Since(testStart)
		testPassed = (err == nil)
		testOutput = string(out)

		fmt.Fprintf(os.Stderr, "Shadow test run completed in %s (passed: %t)\n", testDuration.Round(time.Millisecond), testPassed)
		if !testPassed {
			fmt.Fprintf(os.Stderr, "Shadow test failures:\n%s\n", testOutput)
		}
	}

	// 5. Full suite comparison if log provided
	var fullSuiteFailing []string
	var divergences []string
	var fullSuiteChecked bool
	if *checkFullLog != "" {
		data, err := os.ReadFile(*checkFullLog)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to read full suite log (%s): %v\n", *checkFullLog, err)
		} else {
			fullSuiteChecked = true
			fullSuiteFailing = FindFailingPackages(string(data))
			divergences = CompareWithFullSuite(result, fullSuiteFailing)
			if len(divergences) > 0 {
				fmt.Fprintf(os.Stderr, "WARNING: Full suite failed in package(s) omitted by selection: %v\n", divergences)
			}
		}
	}

	report := &ShadowReport{
		Result:           result,
		Elapsed:          elapsed,
		ShadowExecuted:   *runTests && len(result.AffectedPackages) > 0,
		ShadowDuration:   testDuration,
		ShadowPassed:     testPassed,
		FullSuiteChecked: fullSuiteChecked,
		FullSuiteFailing: fullSuiteFailing,
		Divergences:      divergences,
	}

	// 6. Append markdown summary if requested
	if *summaryFile != "" {
		summary := FormatMarkdownSummary(report)
		f, err := os.OpenFile(*summaryFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to open summary file: %v\n", err)
		} else {
			_, _ = f.WriteString(summary)
			_ = f.Close()
		}
	}

	// 7. Output result in requested format
	var outputText string
	switch *format {
	case "lines":
		outputText = strings.Join(result.AffectedPackages, "\n")
		if outputText != "" {
			outputText += "\n"
		}
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error serializing JSON output: %v\n", err)
			os.Exit(1)
		}
		outputText = string(data) + "\n"
	case "summary":
		outputText = FormatMarkdownSummary(report)
	case "list":
		fallthrough
	default:
		outputText = strings.Join(result.AffectedPackages, " ")
		if outputText != "" {
			outputText += "\n"
		}
	}

	if *outFile != "" {
		if err := os.WriteFile(*outFile, []byte(outputText), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Print(outputText)
	}
}
