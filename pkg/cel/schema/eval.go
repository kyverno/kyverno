package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/google/cel-go/cel"
)

type computedProgram struct {
	name string
	prog cel.Program
}

func (s *Schema) program(expr string) (cel.Program, *cel.Ast, error) {
	ast, iss := s.env.Compile(expr)
	if iss.Err() != nil {
		return nil, nil, iss.Err()
	}
	prog, err := s.env.Program(ast)
	return prog, ast, err
}

// Expr is a compiled CEL expression bound to a schema's environment.
type Expr struct {
	prog   cel.Program
	isBool bool
}

// IsBool reports whether the expression statically evaluates to a bool.
func (e *Expr) IsBool() bool { return e.isBool }

// Eval runs the expression against a payload.
func (e *Expr) Eval(p *Payload) (any, error) {
	out, _, err := e.prog.Eval(p.activation())
	if err != nil {
		return nil, err
	}
	return out.Value(), nil
}

// Compile type-checks expr against the schema and returns a reusable program.
func (s *Schema) Compile(expr string) (*Expr, error) {
	prog, ast, err := s.program(expr)
	if err != nil {
		return nil, err
	}
	return &Expr{prog: prog, isBool: ast.OutputType() == cel.BoolType}, nil
}

// Payload is a validated, normalized payload with its computed fields.
type Payload struct {
	object    map[string]any
	derived   map[string]any
	transport map[string]any
}

func (p *Payload) activation() map[string]any {
	return map[string]any{"object": p.object, "derived": p.derived, "transport": p.transport}
}

// ParseJSON parses one JSON document losslessly (UseNumber) and rejects
// trailing data. The result is not yet validated.
func ParseJSON(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("decoding payload: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("decoding payload: trailing data after JSON document")
	}
	return v, nil
}

// Decode is ParseJSON followed by Build with empty transport metadata.
func (s *Schema) Decode(raw []byte) (*Payload, error) {
	v, err := ParseJSON(raw)
	if err != nil {
		return nil, err
	}
	return s.Build(v, nil)
}

// Build validates a parsed document against the schema, normalizes numbers
// for CEL, and evaluates computed fields. transport is trusted metadata
// (method, path, headers) supplied by the listener, never by the payload.
func (s *Schema) Build(v any, transport map[string]any) (*Payload, error) {
	if res := s.validator.Validate(v); !res.IsValid() {
		return nil, fmt.Errorf("payload does not match schema: %w", res.AsError())
	}
	obj, ok := normalize(s.exposure.apply(v)).(map[string]any)
	if !ok {
		return nil, errors.New("payload root must be an object")
	}
	if transport == nil {
		transport = map[string]any{}
	}
	p := &Payload{object: obj, derived: map[string]any{}, transport: transport}
	for _, c := range s.computed {
		out, _, err := c.prog.Eval(p.activation())
		if err != nil {
			return nil, fmt.Errorf("computed %q: %w", c.name, err)
		}
		p.derived[c.name] = out.Value()
	}
	return p, nil
}

// Eval compiles and runs expr against the payload.
func (s *Schema) Eval(expr string, p *Payload) (any, error) {
	e, err := s.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("compiling: %w", err)
	}
	return e.Eval(p)
}

// Check type-checks expr without a payload.
func (s *Schema) Check(expr string) error {
	_, err := s.Compile(expr)
	return err
}

// normalize turns json.Number into int64 when integral, float64 otherwise.
// Spike: the proposal wants this driven by the schema, not by the value.
func normalize(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		f, _ := t.Float64()
		return f
	case map[string]any:
		for k, e := range t {
			t[k] = normalize(e)
		}
	case []any:
		for i, e := range t {
			t[i] = normalize(e)
		}
	}
	return v
}
