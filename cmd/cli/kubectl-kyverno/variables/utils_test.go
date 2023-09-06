package variables

import (
	"testing"
)

func TestNeedsVariable(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{{
		name: "",
		want: false,
	}, {
		name: "request.object.spec",
		want: false,
	}, {
		name: "request.operation",
		want: false,
	}, {
		name: "element.spec.container",
		want: false,
	}, {
		name: "elementIndex",
		want: false,
	}, {
		name: "foo",
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsVariable(tt.name); got != tt.want {
				t.Errorf("NeedsVariable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNeedsVariables(t *testing.T) {
	tests := []struct {
		name      string
		variables []string
		want      bool
	}{{
		name:      "nil",
		variables: nil,
		want:      false,
	}, {
		name:      "empty",
		variables: []string{},
		want:      false,
	}, {
		name: "false",
		variables: []string{
			"request.object.spec",
			"request.operation",
			"element.spec.container",
			"elementIndex",
		},
		want: false,
	}, {
		name: "true",
		variables: []string{
			"request.object.spec",
			"request.operation",
			"element.spec.container",
			"elementIndex",
			"foo",
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsVariables(tt.variables...); got != tt.want {
				t.Errorf("NeedsVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}
