package jmespath

import (
	"reflect"
	"testing"

	"github.com/google/cel-go/cel"
)

func TestJMESPath(t *testing.T) {
	env, err := cel.NewEnv(Lib())
	if err != nil {
		t.Fatalf("failed to create env: %v", err)
	}

	tests := []struct {
		name    string
		expr    string
		vars    map[string]any
		want    any
		wantErr bool
	}{
		{
			name: "simple object match",
			expr: `jmespath("a", object)`,
			vars: map[string]any{
				"object": map[string]any{"a": "b"},
			},
			want: "b",
		},
		{
			name: "array projection",
			expr: `jmespath("items[].name", object)`,
			vars: map[string]any{
				"object": map[string]any{
					"items": []any{
						map[string]any{"name": "foo"},
						map[string]any{"name": "bar"},
					},
				},
			},
			want: []any{"foo", "bar"},
		},
		{
			name: "bad query",
			expr: `jmespath("items[", object)`,
			vars: map[string]any{
				"object": map[string]any{"items": []any{}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, iss := env.Compile(tt.expr)
			if iss.Err() != nil {
				t.Fatalf("failed to compile: %v", iss.Err())
			}

			prg, err := env.Program(ast)
			if err != nil {
				t.Fatalf("failed to create program: %v", err)
			}

			out, _, err := prg.Eval(tt.vars)
			if tt.wantErr {
				if err == nil {
					// Eval doesn't necessarily return a go error for CEL errors (it returns an error ref.Val sometimes)
					if _, isErr := out.Value().(error); !isErr {
						t.Errorf("expected error, got %v", out.Value())
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("evaluation failed: %v", err)
			}

			if !reflect.DeepEqual(out.Value(), tt.want) {
				t.Errorf("got %v, want %v", out.Value(), tt.want)
			}
		})
	}
}
