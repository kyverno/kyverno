package compiler

import (
	"github.com/google/cel-go/cel"
)

type Validation struct {
	// Identifier is the stable name of the validation, taken from the
	// source spec's Identifier field when set. It is used to build
	// autogen rule names that survive reordering of the validations list.
	Identifier        string
	Message           string
	MessageExpression cel.Program
	// Program decides the validation. Traced and AST are only set when compiled for tracing:
	// Traced is the explain-only tracking twin of Program, see TracedProgram for why they differ.
	Program cel.Program
	Traced  cel.Program
	AST     *cel.Ast
}
