package compiler

import (
	"github.com/google/cel-go/cel"
)

type Validation struct {
	Message           string
	MessageExpression cel.Program
	Program           cel.Program
}
