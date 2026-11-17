package schema

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
)

type computedProgram struct {
	name string
	prog cel.Program
}

// compileExpr builds the program through compiler.NewProgram so the per-program
// cost limit applies; it is evaluated with ContextEval so the request budget
// is charged.
func (s *Schema) compileExpr(expr string) (cel.Program, *cel.Ast, error) {
	ast, iss := s.env.Compile(expr)
	if iss.Err() != nil {
		return nil, nil, iss.Err()
	}
	prog, err := compiler.NewProgram(s.env, ast)
	return prog, ast, err
}

// Expr is a compiled CEL expression bound to a schema's environment.
type Expr struct {
	prog   cel.Program
	isBool bool
}

// IsBool reports whether the expression can evaluate to a bool: statically
// bool, or dyn (as with a bare `derived.x` reference).
func (e *Expr) IsBool() bool { return e.isBool }

// Run evaluates the expression against a payload.
func (e *Expr) Run(ctx context.Context, p *Payload) (any, error) {
	out, _, err := e.prog.ContextEval(ctx, p.activation())
	if err != nil {
		return nil, err
	}
	return out.Value(), nil
}

// Compile type-checks expr against the schema and returns a reusable program.
func (s *Schema) Compile(expr string) (*Expr, error) {
	prog, ast, err := s.compileExpr(expr)
	if err != nil {
		return nil, err
	}
	t := ast.OutputType()
	return &Expr{prog: prog, isBool: t.IsExactType(cel.BoolType) || t.IsExactType(cel.DynType)}, nil
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

// ParseJSON parses one JSON document losslessly (UseNumber). It rejects
// trailing data, duplicate object keys (the decoder would otherwise keep only
// the last value, so selection and validation could disagree about the
// payload) and nesting deeper than maxDepth. The result is not yet validated.
func ParseJSON(raw []byte, maxDepth int) (any, error) {
	if err := checkStructure(raw, maxDepth); err != nil {
		return nil, err
	}
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

func checkStructure(raw []byte, maxDepth int) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := walkTokens(dec, 0, maxDepth); err != nil {
		return fmt.Errorf("decoding payload: %w", err)
	}
	return nil
}

func walkTokens(dec *json.Decoder, depth, maxDepth int) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	if depth+1 > maxDepth {
		return fmt.Errorf("payload nesting exceeds maxDepth %d", maxDepth)
	}
	switch d {
	case '{':
		seen := map[string]struct{}{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return err
			}
			key, _ := kt.(string)
			if _, dup := seen[key]; dup {
				return fmt.Errorf("duplicate object key %q", key)
			}
			seen[key] = struct{}{}
			if err := walkTokens(dec, depth+1, maxDepth); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := walkTokens(dec, depth+1, maxDepth); err != nil {
				return err
			}
		}
	}
	_, err = dec.Token()
	return err
}

// Decode is ParseJSON followed by Build with empty transport metadata.
func (s *Schema) Decode(ctx context.Context, raw []byte) (*Payload, error) {
	v, err := ParseJSON(raw, defaultMaxDepth)
	if err != nil {
		return nil, err
	}
	return s.Build(ctx, v, nil)
}

// Build validates a parsed document against the schema, applies the exposure
// projection, converts numbers to the declared CEL types, and evaluates
// computed fields. transport is trusted metadata (method, path, headers,
// principal) supplied by the listener, never by the payload.
func (s *Schema) Build(ctx context.Context, v any, transport map[string]any) (*Payload, error) {
	v, err := walkNumbers(v, &s.jsonSchema, false)
	if err != nil {
		return nil, err
	}
	if res := s.validator.Validate(v); !res.IsValid() {
		return nil, fmt.Errorf("payload does not match schema: %w", res.AsError())
	}
	typed, err := walkNumbers(s.exposure.apply(v), &s.jsonSchema, true)
	if err != nil {
		return nil, fmt.Errorf("payload does not match schema: %w", err)
	}
	obj, ok := typed.(map[string]any)
	if !ok {
		return nil, errors.New("payload root must be an object")
	}
	if transport == nil {
		transport = map[string]any{}
	}
	p := &Payload{object: obj, derived: map[string]any{}, transport: transport}
	for _, c := range s.computed {
		out, _, err := c.prog.ContextEval(ctx, p.activation())
		if err != nil {
			return nil, fmt.Errorf("computed %q: %w", c.name, err)
		}
		p.derived[c.name] = out.Value()
	}
	return p, nil
}

// Evaluate compiles and runs expr against the payload.
func (s *Schema) Evaluate(ctx context.Context, expr string, p *Payload) (any, error) {
	e, err := s.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("compiling: %w", err)
	}
	return e.Run(ctx, p)
}

// Check type-checks expr without a payload.
func (s *Schema) Check(expr string) error {
	_, err := s.Compile(expr)
	return err
}
