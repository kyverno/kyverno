package compiler

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// Every CEL program in this repository must be built through NewProgram and evaluated
// through ContextEval.
//
// NewProgram is what attaches the cost sentinel, and ContextEval is what charges the
// request budget. A program built with a bare env.Program still inherits the per-program
// limit from the env's program options, and a bare Eval still honours it -- so a bypass
// here is not unbounded. What it loses is the request-wide total, and an
// ImageValidatingPolicy may declare any number of matchImageReferences and attestors (the
// CRD sets no maxItems), so many individually-capped expressions can still add up.
//
// This is a class guard rather than a per-site test: the five request-budget gaps the
// 1.19 audit found were each a single call site, found one at a time, and a grep for one
// shape (env.Program) could not see the other (bare Eval). A new site now fails here
// instead of silently losing its accounting.
func TestNoUnbudgetedCELEvaluationPaths(t *testing.T) {
	root := repoRoot(t)

	// Bare construction and bare evaluation. forbidigo is not enabled in .golangci.yml, so
	// this test is the only thing standing between a new call site and lost accounting.
	//
	// Keyed to the receiver dot, not to the argument name: `.Program(ast` would not see
	// `env.Program(compiled)`, and the dot is what separates a construction call from a
	// conversion to a same-named local type.
	bareProgram := regexp.MustCompile(`\.Program\(`)
	bareEval := regexp.MustCompile(`\.Eval\(`)
	// A budgetless ContextEval is the third bypass shape, and the one the first two
	// patterns cannot see: the program is built correctly and evaluated through the right
	// method, but on a context that carries no budget, so Charge finds nothing and returns
	// nil. Two call sites in this series were exactly this.
	budgetlessEval := regexp.MustCompile(`ContextEval\(context\.(TODO|Background)\(\)`)

	// The chokepoint itself, and the test that deliberately exercises the bare shape.
	exempt := map[string]bool{
		"pkg/cel/compiler/program.go":         true,
		"pkg/cel/compiler/costbypass_test.go": true,
		"pkg/cel/compiler/evalpaths_test.go":  true,
	}

	var offenders []string
	for _, dir := range []string{"pkg", "cmd"} {
		err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if exempt[rel] || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for i, line := range strings.Split(string(body), "\n") {
				if strings.Contains(line, "nolint:forbidigo") {
					continue
				}
				if bareProgram.MatchString(line) {
					offenders = append(offenders, rel+":"+itoa(i+1)+" bare env.Program (use compiler.NewProgram)")
				}
				if bareEval.MatchString(line) && !strings.Contains(line, "ContextEval") {
					offenders = append(offenders, rel+":"+itoa(i+1)+" bare Eval (use ContextEval so the request budget charges)")
				}
				if budgetlessEval.MatchString(line) {
					offenders = append(offenders, rel+":"+itoa(i+1)+" ContextEval on a budgetless context (thread the caller's ctx, or establish a budget)")
				}
			}
			return nil
		})
		assert.NilError(t, err)
	}
	assert.Assert(t, len(offenders) == 0, "unbudgeted CEL evaluation paths:\n  %s", strings.Join(offenders, "\n  "))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	assert.NilError(t, err)
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repository root (no go.mod found)")
		}
		dir = parent
	}
}
