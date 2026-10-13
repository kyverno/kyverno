package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageMatches(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		patterns []string
		want     bool
	}{
		{"exact match", "nginx:latest", []string{"nginx:latest"}, true},
		{"wildcard match", "nginx:latest", []string{"nginx:*"}, true},
		{"registry match", "ghcr.io/repo/img:v1", []string{"ghcr.io/*"}, true},
		{"no match", "redis:6", []string{"nginx:*"}, false},
		{"multiple patterns", "busybox:1.36", []string{"nginx:*", "busybox:*"}, true},
		{"empty patterns", "nginx:latest", []string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ImageMatches(tt.image, tt.patterns))
		})
	}
}
