package compiler

import (
	"fmt"

	"github.com/google/cel-go/cel"
)

// TraceEntry is a single captured expression evaluation. One flat type is
// reused across all layers (match, variable, verdict, exception).
type TraceEntry struct {
	Layer      string
	Name       string
	Expression string
	Value      string
	Error      string
	Evaluated  bool
}

// CaptureTrace turns tracked eval state into TraceEntry values. Returns nil
// unless the program was compiled with cel.OptTrackState.
func CaptureTrace(layer, name string, ast *cel.Ast, details *cel.EvalDetails) []TraceEntry {
	if details == nil {
		return nil
	}
	state := details.State()
	if state == nil {
		return nil
	}

	ids := state.IDs()
	entries := make([]TraceEntry, 0, len(ids))
	for _, id := range ids {
		val, found := state.Value(id)
		if !found {
			continue
		}

		expr, err := cel.AstToString(ast)
		if err != nil {
			expr = ""
		}

		entry := TraceEntry{
			Layer:      layer,
			Name:       name,
			Expression: expr,
			Evaluated:  true,
		}
		if val != nil {
			entry.Value = fmt.Sprintf("%v", val.Value())
		}
		entries = append(entries, entry)
	}
	return entries
}

// NotEvaluated marks an expression that was skipped, e.g. an unreferenced
// variable or a short-circuited match condition.
func NotEvaluated(layer, name, expression string) TraceEntry {
	return TraceEntry{
		Layer:      layer,
		Name:       name,
		Expression: expression,
		Evaluated:  false,
	}
}
