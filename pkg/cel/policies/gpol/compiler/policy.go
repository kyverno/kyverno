package compiler

import (
	"github.com/google/cel-go/cel"
)

type Policy struct {
	matchConditions []cel.Program
	variables       map[string]cel.Program
	generations     []cel.Program
}
