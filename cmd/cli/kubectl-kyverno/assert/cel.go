package assert

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"google.golang.org/protobuf/types/known/structpb"
)

type Program func(object any, bindings Bindings) (any, error)

type Compiler interface {
	Compile(statement string) (Program, error)
}

var DefaultCompiler Compiler = &celCompiler{env: sync.OnceValues(newEnv)}

func execute(statement string, value any, bindings Bindings) (any, error) {
	program, err := DefaultCompiler.Compile(statement)
	if err != nil {
		return nil, fmt.Errorf("invalid expression %q (expressions must be CEL, JMESPath is no longer supported): %w", statement, err)
	}
	return program(value, bindings)
}

type celCompiler struct {
	env      func() (*cel.Env, error)
	programs sync.Map
}

func newEnv() (*cel.Env, error) {
	base, err := compiler.NewBaseEnv()
	if err != nil {
		return nil, err
	}
	return base.Extend(
		cel.Variable("object", cel.DynType),
		cel.Variable("bindings", cel.MapType(cel.StringType, cel.DynType)),
	)
}

func (c *celCompiler) Compile(statement string) (Program, error) {
	if program, ok := c.programs.Load(statement); ok {
		return program.(Program), nil
	}
	env, err := c.env()
	if err != nil {
		return nil, err
	}
	ast, issues := env.Compile(statement)
	if err := issues.Err(); err != nil {
		return nil, err
	}
	prg, err := env.Program(ast)
	if err != nil {
		return nil, err
	}
	program := Program(func(object any, bindings Bindings) (any, error) {
		object, err := toJSON(object)
		if err != nil {
			return nil, err
		}
		vars, err := toJSON(bindings)
		if err != nil {
			return nil, err
		}
		if vars == nil {
			vars = map[string]any{}
		}
		out, _, err := prg.Eval(map[string]any{"object": object, "bindings": vars})
		if err != nil {
			return nil, err
		}
		native, err := out.ConvertToNative(reflect.TypeFor[*structpb.Value]())
		if err != nil {
			return nil, err
		}
		return native.(*structpb.Value).AsInterface(), nil
	})
	c.programs.Store(statement, program)
	return program, nil
}

// toJSON converts Go values (e.g. structs) into plain JSON values usable by CEL.
func toJSON(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out any
	return out, json.Unmarshal(data, &out)
}
