package jmespath

import (
	"testing"

	gojmespath "github.com/jmespath/go-jmespath"
)

func TestFunctionEntry_String(t *testing.T) {
	type fields struct {
		FunctionEntry gojmespath.FunctionEntry
		Note          string
		ReturnType    []jpType
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{{
		fields: fields{
			FunctionEntry: gojmespath.FunctionEntry{
				Name: compare,
				Arguments: []argSpec{
					{Types: []jpType{jpString}},
					{Types: []jpType{jpString}},
				},
				Handler: jpfCompare,
			},
			ReturnType: []jpType{jpNumber},
			Note:       "compares two strings lexicographically",
		},
		want: "compare(string, string) number (compares two strings lexicographically)",
	}, {
		fields: fields{
			Note: "compares two strings lexicographically",
		},
		want: "",
	}, {
		fields: fields{
			FunctionEntry: gojmespath.FunctionEntry{
				Name: compare,
				Arguments: []argSpec{
					{Types: []jpType{jpString}},
					{Types: []jpType{jpString}},
				},
				Handler: jpfCompare,
			},
			ReturnType: []jpType{jpNumber},
		},
		want: "compare(string, string) number",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := FunctionEntry{
				FunctionEntry: tt.fields.FunctionEntry,
				Note:          tt.fields.Note,
				ReturnType:    tt.fields.ReturnType,
			}
			if got := f.String(); got != tt.want {
				t.Errorf("FunctionEntry.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
