package compiler

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestCompileGlobalContextProjection_Success(t *testing.T) {
	prog, errs := CompileGlobalContextProjection(field.NewPath("expression"), "object.items.size()")
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if prog == nil {
		t.Fatal("expected a compiled program, got nil")
	}
}

func TestCompileGlobalContextProjection_CompileError(t *testing.T) {
	_, errs := CompileGlobalContextProjection(field.NewPath("expression"), "object.items.sizee()")
	if len(errs) == 0 {
		t.Fatal("expected a compile error for an unknown method, got none")
	}
}

func TestCompileGlobalContextProjection_UnknownVariableFails(t *testing.T) {
	_, errs := CompileGlobalContextProjection(field.NewPath("expression"), "notObject.size()")
	if len(errs) == 0 {
		t.Fatal("expected a compile error for an undeclared variable, got none")
	}
}

func TestCompileGlobalContextProjection_LibraryScopeExcludesHTTPAndGlobalContext(t *testing.T) {
	// Assert the specific undeclared-reference error, not merely that something failed:
	// a typo in the expression would also produce "some error" and pass vacuously.
	for ident, expr := range map[string]string{
		"http":          "http.Get('x')",
		"globalContext": "globalContext.Get('x')",
	} {
		_, errs := CompileGlobalContextProjection(field.NewPath("expression"), expr)
		if len(errs) == 0 {
			t.Errorf("expected expression %q to fail to compile (library out of scope for GCE projections), got no errors", expr)
			continue
		}
		want := "undeclared reference to '" + ident + "'"
		if !strings.Contains(errs.ToAggregate().Error(), want) {
			t.Errorf("expression %q: expected an error containing %q, got %v", expr, want, errs.ToAggregate())
		}
	}
}

// TestCompileGlobalContextProjection_ConcurrentCompile compiles from many goroutines at
// once against the shared package-level env. Note it does NOT exercise the lazy sync.Once
// construction: earlier tests in this package have already triggered it, so the env is
// built by the time this runs. What this covers is concurrent use of an already-built env.
func TestCompileGlobalContextProjection_ConcurrentCompile(t *testing.T) {
	const n = 50
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			expr := "object.size()"
			if i%2 == 0 {
				expr = "object.items.size()"
			}
			prog, errs := CompileGlobalContextProjection(field.NewPath("expression"), expr)
			if len(errs) != 0 {
				errCh <- errs.ToAggregate()
				return
			}
			if prog == nil {
				errCh <- errors.New("expected a compiled program, got nil")
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent compile failed: %v", err)
	}
}
