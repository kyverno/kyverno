package v1

import (
	"testing"

	"gotest.tools/assert"
)

func TestRule_HasValidateAllowExistingViolations(t *testing.T) {
	tests := []struct {
		name string
		rule Rule
		want bool
	}{
		{
			name: "Validation is nil",
			rule: Rule{
				Validation: nil,
			},
			want: true,
		},
		{
			name: "Validation.AllowExistingViolations is nil",
			rule: Rule{
				Validation: &Validation{
					AllowExistingViolations: nil,
				},
			},
			want: true,
		},
		{
			name: "Validation.AllowExistingViolations is true",
			rule: Rule{
				Validation: &Validation{
					AllowExistingViolations: func() *bool { b := true; return &b }(),
				},
			},
			want: true,
		},
		{
			name: "Validation.AllowExistingViolations is false",
			rule: Rule{
				Validation: &Validation{
					AllowExistingViolations: func() *bool { b := false; return &b }(),
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.rule.HasValidateAllowExistingViolations(), tt.want)
		})
	}
}
