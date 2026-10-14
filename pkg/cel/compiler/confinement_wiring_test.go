package compiler_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Namespace confinement for CEL policies is applied at the point each policy
// compiler builds its activation. The confining wrappers themselves are unit
// tested, but nothing pinned the wiring: every call site could be replaced with
// the raw provider and the whole suite stayed green, silently removing the
// defence behind CVE-2026-100703.
//
// This test pins the invariant instead of any one call site, so a new policy
// kind that forgets to confine fails here rather than shipping.
var confinementRules = []struct {
	importSuffix string   // import path of the activation's package
	confinedBy   []string // at least one must appear in the value expression
}{
	{importSuffix: "/libs/globalcontext", confinedBy: []string{"ConfineGlobalContext"}},
	{importSuffix: "/libs/http", confinedBy: []string{"NewLazyCELHTTPContext"}},
}

func TestNamespaceConfinementIsWiredEverywhere(t *testing.T) {
	root := moduleRoot(t)
	checked := 0

	for _, dir := range []string{"pkg", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "vendor" || d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			checked += checkFile(t, root, path)
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}

	// Guard against the test silently passing because it found nothing to check,
	// for example after a package move.
	if checked == 0 {
		t.Fatal("no namespace-confined activations found; this test is no longer checking anything")
	}
	t.Logf("verified %d confined activations", checked)
}

func checkFile(t *testing.T, root, path string) int {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	// Map each file-local package name to its import path, so an aliased import
	// cannot slip past the match.
	imports := map[string]string{}
	for _, imp := range file.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		name := p[strings.LastIndex(p, "/")+1:]
		if imp.Name != nil {
			name = imp.Name.Name
		}
		imports[name] = p
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}

	found := 0
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Context" {
			return true
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		importPath, ok := imports[pkgIdent.Name]
		if !ok {
			return true
		}

		for _, rule := range confinementRules {
			if !strings.HasSuffix(importPath, rule.importSuffix) {
				continue
			}
			value := contextInterfaceValue(lit)
			if value == nil {
				continue
			}
			found++
			if !mentionsAny(value, rule.confinedBy) {
				t.Errorf("%s:%d: %s.Context activation is not namespace-confined; wrap the inner context with one of %v",
					rel, fset.Position(lit.Pos()).Line, pkgIdent.Name, rule.confinedBy)
			}
		}
		return true
	})
	return found
}

func contextInterfaceValue(lit *ast.CompositeLit) ast.Expr {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "ContextInterface" {
			return kv.Value
		}
	}
	return nil
}

func mentionsAny(expr ast.Expr, names []string) bool {
	hit := false
	ast.Inspect(expr, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		for _, name := range names {
			if ident.Name == name {
				hit = true
				return false
			}
		}
		return true
	})
	return hit
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.mod")
		}
		dir = parent
	}
}
