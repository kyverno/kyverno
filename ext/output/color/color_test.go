package color

import (
	"testing"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name    string
		noColor bool
		force   bool
	}{{
		noColor: true,
		force:   false,
	}, {
		noColor: true,
		force:   true,
	}, {
		noColor: false,
		force:   false,
	}, {
		noColor: false,
		force:   true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(tt.noColor, tt.force)
		})
	}
}
