package regex

import (
	"testing"
)

func TestIsVariable(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid variable", "{{request.object.metadata.name}}", true},
		{"multiple variables", "{{a}}-{{b}}", true},
		{"no variable", "just-a-string", false},
		{"incomplete variable", "{{request.object", false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsVariable(tt.input); got != tt.want {
				t.Errorf("IsVariable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsReference(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid reference", "$(./../name)", true},
		{"no reference", "just-a-string", false},
		{"incomplete reference", "$(./name", false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsReference(tt.input); got != tt.want {
				t.Errorf("IsReference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObjectHasVariables(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{
			name:    "map with variable",
			input:   map[string]string{"key": "{{val}}"},
			wantErr: true,
		},
		{
			name:    "map without variable",
			input:   map[string]string{"key": "val"},
			wantErr: false,
		},
		{
			name:    "nested object with variable",
			input:   map[string]interface{}{"metadata": map[string]string{"name": "{{name}}"}},
			wantErr: true,
		},
		{
			name:    "unsupported type (function)",
			input:   func() {},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ObjectHasVariables(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ObjectHasVariables() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
