package compiler

import (
	"github.com/google/cel-go/cel"
)

type Validation struct {
	Message           string
	MessageExpression cel.Program
	// MessageExpressionAST is retained to determine whether failure messages may expose Secret data.
	MessageExpressionAST *cel.Ast
	// Program decides the validation. Traced and AST are only set when compiled for tracing:
	// Traced is the explain-only tracking twin of Program, see TracedProgram for why they differ.
	Program cel.Program
	Traced  cel.Program
	AST     *cel.Ast
}
