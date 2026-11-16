package compiler

import (
	"github.com/google/cel-go/cel"
)

// Compile is a thin, drop-in wrapper around env.Compile: with no tenant/cluster scope
// distinction in this cut, it does nothing env.Compile does not already do. Kept so
// regression tests exercise the same call a real compiler would, and as the natural place
// to reintroduce a compile-time check later without touching every call site again.
func Compile(env *cel.Env, expr string) (*cel.Ast, error) {
	ast, iss := env.Compile(expr)
	if err := iss.Err(); err != nil {
		return nil, err
	}
	return ast, nil
}
