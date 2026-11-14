package compiler

import "cel.dev/cel-go/cel"

// warmUpExpression references an undeclared qualified name and an undeclared
// function, so the type checker reports undeclared references.
const warmUpExpression = "__kyverno_checker_warmup__.field && __kyverno_checker_warmup__.fn()"

// WarmUpChecker initializes the lazily built symbol catalog of an
// environment's type checker before the environment is shared.
//
// cel.dev/cel-go v0.33.2 builds that catalog on the first undeclared
// reference (checker.(*Env).Catalog) and copies it unsynchronized when it
// enters a comprehension scope (checker.(*Env).cloneWithDecls). Two
// goroutines that compile on the same environment, one with an undeclared
// reference and one with a comprehension, therefore race. Reporting an
// undeclared reference once, before any concurrent use, makes later catalog
// accesses read-only. The compile error is expected and ignored.
func WarmUpChecker(envs ...*cel.Env) {
	for _, env := range envs {
		if env != nil {
			_, _ = env.Compile(warmUpExpression)
		}
	}
}
