package exception

import (
	"github.com/google/cel-go/cel"
)

type CompiledException struct {
	matchConditions []cel.Program
}
