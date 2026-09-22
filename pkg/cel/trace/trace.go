package trace

import (
	"fmt"
	"sort"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
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
	native := ast.NativeRep()
	sourceInfo := native.SourceInfo()

	idToExpr := map[int64]celast.Expr{}
	celast.PreOrderVisit(native.Expr(), celast.NewExprVisitor(func(e celast.Expr) {
		idToExpr[e.ID()] = e
	}))

	type located struct {
		start int32
		node  NodeTrace
	}
	var entries []located
	for _, id := range state.IDs() {
		node, ok := idToExpr[id]
		if !ok {
			continue
		}
		text, err := cel.ExprToString(node, sourceInfo)
		if err != nil || text == "" {
			continue
		}
		val, ok := state.Value(id)
		nt := NodeTrace{Expression: text}
		if ok {
			nt.Value = stringify(val)
		}
		offset, _ := sourceInfo.GetOffsetRange(id)
		entries = append(entries, located{start: offset.Start, node: nt})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].start < entries[j].start })
	for _, e := range entries {
		et.Nodes = append(et.Nodes, e.node)
	}
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
