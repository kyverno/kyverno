package trace

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

type NodeTrace struct {
	Expression string
	Value      string
	Error      string
}

type ExpressionTrace struct {
	Source string
	Nodes  []NodeTrace
	Result string
}

func Build(source string, ast *cel.Ast, result ref.Val, details *cel.EvalDetails) ExpressionTrace {
	et := ExpressionTrace{
		Source: source,
		Result: stringify(result),
	}
	if details == nil || ast == nil {
		return et
	}
	state := details.State()
	if state == nil {
		return et
	}
	native := ast.NativeRep()
	sourceInfo := native.SourceInfo()

	celast.PreOrderVisit(native.Expr(), celast.NewExprVisitor(func(e celast.Expr) {
		if e.Kind() == celast.LiteralKind {
			// a literal's value is its own text, so tracing it only adds noise
			return
		}
		val, ok := state.Value(e.ID())
		if !ok {
			// this node never evaluated (e.g. short-circuited by &&/||), nothing to trace
			return
		}
		text, err := cel.ExprToString(e, sourceInfo)
		if err != nil || text == "" {
			return
		}
		if strings.Contains(text, "@") && referencesMacroInternal(e) {
			// all()/exists() and friends expand into helper nodes such as @result; they are
			// noise to a reader. The macro call itself renders without them and is kept.
			return
		}
		nt := NodeTrace{Expression: text}
		if types.IsError(val) {
			nt.Error = stringify(val)
		} else {
			nt.Value = stringify(val)
		}
		et.Nodes = append(et.Nodes, nt)
	}))
	return et
}

// referencesMacroInternal reports whether e mentions one of the synthetic identifiers (prefixed
// with @) that macro expansion introduces. Checking for the identifier rather than a bare "@"
// keeps nodes that merely contain an @ inside a string literal.
func referencesMacroInternal(e celast.Expr) bool {
	found := false
	celast.PreOrderVisit(e, celast.NewExprVisitor(func(n celast.Expr) {
		if n.Kind() == celast.IdentKind && strings.HasPrefix(n.AsIdent(), "@") {
			found = true
		}
	}))
	return found
}

func stringify(v ref.Val) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v.Value())
}

type NamedExpressionTrace struct {
	Name string
	ExpressionTrace
}

type ScopeTrace struct {
	Applied bool
	Reason  string
}

const (
	VerdictPass  = "PASS"
	VerdictFail  = "FAIL"
	VerdictError = "ERROR"
	// VerdictSkip means the policy did not reach a pass/fail decision: a match condition
	// excluded the resource, its constraints did not match, or an exception exempted it.
	VerdictSkip = "SKIP"
)

type VerdictTrace struct {
	Status string
	ExpressionTrace
	Message string
}

type Decision struct {
	PolicyName        string
	PolicyKind        string
	ResourceKind      string
	ResourceName      string
	ResourceNamespace string

	Scope     ScopeTrace
	Match     []NamedExpressionTrace
	Variables []NamedExpressionTrace
	Verdict   VerdictTrace
}
