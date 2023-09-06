package variables

import (
	"reflect"
	"testing"

	valuesapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/values"
)

func TestVariables_HasVariables(t *testing.T) {
	tests := []struct {
		name      string
		values    *valuesapi.Values
		variables map[string]string
		want      bool
	}{{
		name:      "nil",
		values:    nil,
		variables: nil,
		want:      false,
	}, {
		name:      "empty",
		values:    nil,
		variables: map[string]string{},
		want:      false,
	}, {
		name:   "not empty",
		values: nil,
		variables: map[string]string{
			"foo": "bar",
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Variables{
				values:    tt.values,
				variables: tt.variables,
			}
			if got := v.HasVariables(); got != tt.want {
				t.Errorf("Variables.HasVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVariables_Subresources(t *testing.T) {
	tests := []struct {
		name      string
		values    *valuesapi.Values
		variables map[string]string
		want      []valuesapi.Subresource
	}{{
		name:      "nil values",
		values:    nil,
		variables: nil,
		want:      nil,
	}, {
		name: "nil subresources",
		values: &valuesapi.Values{
			Subresources: nil,
		},
		variables: nil,
		want:      nil,
	}, {
		name: "empty subresources",
		values: &valuesapi.Values{
			Subresources: []valuesapi.Subresource{},
		},
		variables: nil,
		want:      nil,
	}, {
		name: "subresources",
		values: &valuesapi.Values{
			Subresources: []valuesapi.Subresource{{}},
		},
		variables: nil,
		want:      []valuesapi.Subresource{{}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Variables{
				values:    tt.values,
				variables: tt.variables,
			}
			if got := v.Subresources(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Variables.Subresources() = %v, want %v", got, tt.want)
			}
		})
	}
}
