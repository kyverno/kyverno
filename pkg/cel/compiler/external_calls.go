package compiler

import (
	"strings"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
)

// notRepeatableCalls are the CEL library functions an explain-only re-run must never repeat,
// keyed by the prefix of their overload IDs. They either reach outside the evaluation, where a
// second call would do something again (http.Post sends a request, resource.Post creates a
// resource) or could get a different answer (http.Get, resource.Get and List, image data), or
// return a different value on every call (random, time.now). Matching overload IDs rather than
// names also catches calls whose receiver is not the library variable itself, such as
// http.Client(ca).Post(...).
var notRepeatableCalls = []struct {
	overloadPrefix string
	name           string
}{
	{"http_get_", "http.Get"},
	{"http_post_", "http.Post"},
	{"resource_get_", "resource.Get"},
	{"resource_list_", "resource.List"},
	{"list_resources_", "resource.List"},
	{"resource_post_", "resource.Post"},
	{"imagedata_get_", "image.GetMetadata"},
	{"random_string_", "random"},
	{"time_now", "time.now"},
}

// NotRepeatable returns the name of the first function in the expression that must not be called
// a second time (see notRepeatableCalls), or "" when the expression only reads its inputs. An
// expression it names gets no explain-only tracking twin, so --explain never repeats its calls.
func NotRepeatable(ast *cel.Ast) string {
	if ast == nil {
		return ""
	}
	native := ast.NativeRep()
	references := native.ReferenceMap()
	name := ""
	celast.PreOrderVisit(native.Expr(), celast.NewExprVisitor(func(e celast.Expr) {
		if name != "" || e.Kind() != celast.CallKind {
			return
		}
		reference, ok := references[e.ID()]
		if !ok {
			return
		}
		for _, overload := range reference.OverloadIDs {
			for _, call := range notRepeatableCalls {
				if strings.HasPrefix(overload, call.overloadPrefix) {
					name = call.name
					return
				}
			}
		}
	}))
	return name
}

// NoBreakdownReason is the note a trace shows for an expression that has no tracking twin because
// of NotRepeatable, or "" when the expression is broken down as usual.
func NoBreakdownReason(ast *cel.Ast) string {
	if name := NotRepeatable(ast); name != "" {
		return "it calls " + name + ", which is not run a second time"
	}
	return ""
}
