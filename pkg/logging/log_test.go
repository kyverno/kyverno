package logging

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"testing"
	"time"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	originalStderr := os.Stderr
	os.Stderr = w

	done := make(chan string, 1)
	go func() {
		defer r.Close()
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	restore := func() {
		os.Stderr = originalStderr
		if w != nil {
			_ = w.Close()
			w = nil
		}
	}
	defer restore()

	fn()

	restore()
	return <-done
}

func TestSetup_TextNoColorFormat_NoANSIEscapeCodes(t *testing.T) {
	output := captureStderr(t, func() {
		if err := Setup(TextNoColorFormat, DefaultTime, 0, false); err != nil {
			t.Fatalf("Setup() error = %v", err)
		}
		GlobalLogger().Info("test message")
	})

	if ansiEscapePattern.MatchString(output) {
		t.Fatalf("unexpected ANSI escape codes in text-nocolor format output: %q", output)
	}
}

func TestSetup_TextFormat_DisableColor_NoANSIEscapeCodes(t *testing.T) {
	output := captureStderr(t, func() {
		if err := Setup(TextFormat, DefaultTime, 0, true); err != nil {
			t.Fatalf("Setup() error = %v", err)
		}
		GlobalLogger().Info("test message")
	})

	if ansiEscapePattern.MatchString(output) {
		t.Fatalf("unexpected ANSI escape codes when disableColor is true: %q", output)
	}
}

func TestSetup_InvalidFormat(t *testing.T) {
	if err := Setup("invalid", DefaultTime, 0, false); err == nil {
		t.Fatal("expected error for invalid log format")
	}
}

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
