package logging

import (
	"testing"
	"time"
)

func TestResolveTimestampFormat(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		expected string
	}{
		{
			name:     "default format",
			format:   DefaultTime,
			expected: time.RFC3339,
		},
		{
			name:     "iso8601 format",
			format:   ISO8601,
			expected: time.RFC3339,
		},
		{
			name:     "rfc3339 format",
			format:   RFC3339,
			expected: time.RFC3339,
		},
		{
			name:     "millis format",
			format:   MILLIS,
			expected: time.StampMilli,
		},
		{
			name:     "nanos format",
			format:   NANOS,
			expected: time.StampNano,
		},
		{
			name:     "epoch format",
			format:   EPOCH,
			expected: time.UnixDate,
		},
		{
			name:     "rfc3339nano format",
			format:   RFC3339NANO,
			expected: time.RFC3339Nano,
		},
		{
			name:     "unknown format",
			format:   "unknown",
			expected: time.RFC3339,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveTimestampFormat(tt.format)
			if result != tt.expected {
				t.Errorf("resolveTimestampFormat(%s) = %s, want %s", tt.format, result, tt.expected)
			}
		})
	}
}
