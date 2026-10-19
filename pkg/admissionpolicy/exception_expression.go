package admissionpolicy

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/common"
	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/parser"
)

// replaceExpressions rewrites exception field selections, leaving string literals,
// similarly named fields, and comprehension-local identifiers unchanged.
func replaceExpressions(expression string, replacements map[string]string) (string, error) {
	if !strings.Contains(expression, "exceptions") || len(replacements) == 0 {
		return expression, nil
	}
	// Keep macro calls unexpanded so they can be printed without reconstructing
	// comprehensions. Optional selections are used by existing validating policies.
	p, err := parser.NewParser(parser.EnableOptionalSyntax(true))
	if err != nil {
		return "", err
	}
	parsed, parseErrors := p.Parse(common.NewTextSource(expression))
	if len(parseErrors.GetErrors()) != 0 {
		return "", fmt.Errorf("failed to parse exception expression: %s", parseErrors.ToDisplayString())
	}
	changed := false
	for _, node := range celast.MatchDescendants(celast.NavigateAST(parsed), celast.KindMatcher(celast.SelectKind)) {
		selection := node.AsSelect()
		operand := selection.Operand()
		if operand.Kind() != celast.IdentKind || (operand.AsIdent() != "exceptions" && operand.AsIdent() != ".exceptions") {
			continue
		}
		if operand.AsIdent() == "exceptions" && shadowsExceptions(node) {
			continue
		}
		replacement, ok := replacements["exceptions."+selection.FieldName()]
		if !ok {
			continue
		}
		// Both exception fields always exist, including when their lists are empty.
		// Inlining a list as the argument of has() would not be valid CEL.
		if parent, ok := node.Parent(); ok && parent.Kind() == celast.CallKind {
			call := parent.AsCall()
			if !call.IsMemberFunction() && call.FunctionName() == "has" && len(call.Args()) == 1 {
				node = parent
				replacement = "true"
			}
		}
		rewritten, errs := p.Parse(common.NewTextSource(replacement))
		if len(errs.GetErrors()) != 0 {
			return "", fmt.Errorf("failed to parse exception replacement: %s", errs.ToDisplayString())
		}
		node.SetKindCase(rewritten.Expr())
		changed = true
	}
	if !changed {
		return expression, nil
	}
	return parser.Unparse(parsed.Expr(), parsed.SourceInfo())
}

func shadowsExceptions(node celast.NavigableExpr) bool {
	for child := node; ; {
		parent, ok := child.Parent()
		if !ok {
			return false
		}
		if parent.Kind() == celast.CallKind {
			call := parent.AsCall()
			args := call.Args()
			switch call.FunctionName() {
			case "all", "exists", "exists_one", "filter", "map":
				if len(args) >= 2 && args[0].Kind() == celast.IdentKind && args[0].AsIdent() == "exceptions" &&
					child.ID() != args[0].ID() && (!call.IsMemberFunction() || child.ID() != call.Target().ID()) {
					return true
				}
			case "bind":
				// cel.bind evaluates its initializer in the enclosing scope.
				if len(args) == 3 && args[0].Kind() == celast.IdentKind && args[0].AsIdent() == "exceptions" && child.ID() == args[2].ID() {
					return true
				}
			}
		}
		child = parent
	}
}
