package color

import (
	"testing"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name    string
		noColor bool
	}{{
		noColor: true,
	}, {
		noColor: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(tt.noColor)
		})
	}
}
