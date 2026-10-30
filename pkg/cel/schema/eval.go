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

func (s *Schema) program(expr string) (cel.Program, error) {
	ast, iss := s.env.Compile(expr)
	if iss.Err() != nil {
		return nil, iss.Err()
	}
	return s.env.Program(ast)
}

// Payload is a validated, decoded payload with its computed fields.
type Payload struct {
	object  map[string]any
	derived map[string]any
}

// Decode parses raw JSON losslessly (UseNumber), rejects trailing documents,
// validates against the schema, then normalizes numbers for CEL.
func (s *Schema) Decode(raw []byte) (*Payload, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("decoding payload: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("decoding payload: trailing data after JSON document")
	}
	if res := s.validator.Validate(v); !res.IsValid() {
		return nil, fmt.Errorf("payload does not match schema: %w", res.AsError())
	}
	obj, ok := normalize(v).(map[string]any)
	if !ok {
		return nil, errors.New("payload root must be an object")
	}
	p := &Payload{object: obj, derived: map[string]any{}}
	for _, c := range s.computed {
		out, _, err := c.prog.Eval(map[string]any{"object": obj, "derived": p.derived})
		if err != nil {
			return nil, fmt.Errorf("computed %q: %w", c.name, err)
		}
		p.derived[c.name] = out.Value()
	}
	return p, nil
}

// Eval compiles and runs expr against the payload.
func (s *Schema) Eval(expr string, p *Payload) (any, error) {
	prog, err := s.program(expr)
	if err != nil {
		return nil, fmt.Errorf("compiling: %w", err)
	}
	out, _, err := prog.Eval(map[string]any{"object": p.object, "derived": p.derived})
	if err != nil {
		return nil, err
	}
	return out.Value(), nil
}

// Check type-checks expr without a payload.
func (s *Schema) Check(expr string) error {
	_, err := s.program(expr)
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
