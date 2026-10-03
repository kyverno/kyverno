package trace

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
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

// stringify renders a CEL value for human consumption. Scalars and maps go through
// fmt.Sprintf("%v", v.Value()) exactly as before. Lists get one extra step: Value() only unwraps
// the outermost layer, so a list's elements can still be un-rendered CEL values -- a JSONPatch
// mutation's result, for instance, is a list of *mutation.JSONPatchVal (from
// k8s.io/apiserver/pkg/cel/mutation), a hand-written CEL type whose Value() just returns itself,
// with no plain-Go form at all. Formatting that bare pointer is fine on its own (Go's fmt
// dereferences a struct pointer passed directly to Sprintf), but not once it is nested inside a
// slice (fmt does not dereference a pointer found while formatting a compound value's elements).
// Recursing element by element gives each one that same direct, top-level Sprintf treatment.
func stringify(v ref.Val) string {
	if v == nil {
		return ""
	}
	if lister, ok := v.(traits.Lister); ok {
		var sb strings.Builder
		sb.WriteByte('[')
		first := true
		for it := lister.Iterator(); it.HasNext() == types.True; {
			if !first {
				sb.WriteString(" ")
			}
			first = false
			sb.WriteString(stringify(it.Next()))
		}
		sb.WriteByte(']')
		return sb.String()
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
	// Mutations holds one entry per mutation expression that actually ran, in order. Unlike
	// Verdict (vpol's single deciding validation), a MutatingPolicy has no one expression that
	// "decides" the outcome -- every mutation that runs contributes to the result, so this is a
	// list rather than a single ExpressionTrace. Empty for policy kinds with no mutations (vpol).
	Mutations []MutationTrace
}

// MutationTrace is the trace of one mutation expression that ran. Name identifies it, e.g.
// "mutations[0] (applyConfiguration)".
type MutationTrace struct {
	Name string
	ExpressionTrace
	// Error is set when evaluating or applying this specific mutation failed outright (the
	// Go-level error Patch() returned), distinct from a Nodes[i].Error, which marks a single
	// failing sub-expression inside an otherwise-evaluated CEL expression.
	Error string
}
