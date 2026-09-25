package trace

import (
	"fmt"

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
		val, ok := state.Value(e.ID())
		if !ok {
			// this node never evaluated (e.g. short-circuited by &&/||), nothing to trace
			return
		}
		text, err := cel.ExprToString(e, sourceInfo)
		if err != nil || text == "" {
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
