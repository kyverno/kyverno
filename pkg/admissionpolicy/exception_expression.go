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

// shadowsExceptions preserves the original exception rewrite behavior while
// sharing the lexical-scope inspection with native compatibility checks.
func shadowsExceptions(node celast.NavigableExpr) bool {
	return shadowsIdentifier(node, "exceptions")
}

func shadowsIdentifier(node celast.NavigableExpr, name string) bool {
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
				if len(args) >= 2 && args[0].Kind() == celast.IdentKind && args[0].AsIdent() == name &&
					child.ID() != args[0].ID() && (!call.IsMemberFunction() || child.ID() != call.Target().ID()) {
					return true
				}
			case "bind":
				// cel.bind evaluates its initializer in the enclosing scope.
				if len(args) == 3 && args[0].Kind() == celast.IdentKind && args[0].AsIdent() == name && child.ID() == args[2].ID() {
					return true
				}
			}
		}
		child = parent
	}
}

// referencesGlobalExceptions detects Kyverno-only exceptions identifiers.
func referencesGlobalExceptions(expression string) (bool, error) {
	return referencesGlobalIdentifier(expression, "exceptions")
}

// referencesGlobalVariables detects native-incompatible variables references
// in match conditions without rejecting string literals or locally shadowed names.
func referencesGlobalVariables(expression string) (bool, error) {
	return referencesGlobalIdentifier(expression, "variables")
}

// referencesGlobalIdentifier inspects the CEL AST rather than matching substrings:
// object.variables, string literals, and local comprehension/bind identifiers
// must not be mistaken for a reference to the policy-level global variable.
func referencesGlobalIdentifier(expression, name string) (bool, error) {
	if !strings.Contains(expression, name) {
		return false, nil
	}
	p, err := parser.NewParser(parser.EnableOptionalSyntax(true))
	if err != nil {
		return false, err
	}
	parsed, parseErrors := p.Parse(common.NewTextSource(expression))
	if len(parseErrors.GetErrors()) != 0 {
		return false, fmt.Errorf("failed to parse %s expression: %s", name, parseErrors.ToDisplayString())
	}
	for _, node := range celast.MatchDescendants(celast.NavigateAST(parsed), celast.KindMatcher(celast.IdentKind)) {
		identifier := node.AsIdent()
		if identifier != name && identifier != "."+name {
			continue
		}
		if identifier == name && (shadowsIdentifier(node, name) || declaresLocalIdentifier(node, name)) {
			continue
		}
		return true, nil
	}
	return false, nil
}

// declaresLocalIdentifier is used on original CEL expressions to detect a
// variable binding that could capture a generated root reference.
func declaresLocalVariables(expression string) (bool, error) {
	if !strings.Contains(expression, "variables") {
		return false, nil
	}
	p, err := parser.NewParser(parser.EnableOptionalSyntax(true))
	if err != nil {
		return false, err
	}
	parsed, parseErrors := p.Parse(common.NewTextSource(expression))
	if len(parseErrors.GetErrors()) != 0 {
		return false, fmt.Errorf("failed to parse variables expression: %s", parseErrors.ToDisplayString())
	}
	for _, node := range celast.MatchDescendants(celast.NavigateAST(parsed), celast.KindMatcher(celast.IdentKind)) {
		if node.AsIdent() == "variables" && declaresLocalIdentifier(node, "variables") {
			return true, nil
		}
	}
	return false, nil
}

func declaresLocalIdentifier(node celast.NavigableExpr, name string) bool {
	parent, ok := node.Parent()
	if !ok || parent.Kind() != celast.CallKind {
		return false
	}
	call := parent.AsCall()
	args := call.Args()
	if len(args) == 0 || args[0].ID() != node.ID() || node.AsIdent() != name {
		return false
	}
	switch call.FunctionName() {
	case "all", "exists", "exists_one", "filter", "map", "bind":
		return true
	}
	return false
}
