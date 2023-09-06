package command

import (
	"testing"
)

func TestFormatDescription(t *testing.T) {
	tests := []struct {
		name         string
		short        bool
		url          string
		experimental bool
		lines        []string
		want         string
	}{{
		name:         "empty (short)",
		short:        true,
		url:          "",
		experimental: false,
		lines:        []string{},
		want:         "",
	}, {
		name:         "empty (long)",
		short:        true,
		url:          "https://example.com",
		experimental: true,
		lines:        []string{},
		want:         "",
	}, {
		name:         "one line (short)",
		short:        true,
		url:          "",
		experimental: false,
		lines:        []string{"this is one line"},
		want:         "this is one line",
	}, {
		name:         "one line with url (short)",
		short:        true,
		url:          "https://example.com",
		experimental: false,
		lines:        []string{"this is one line"},
		want:         "this is one line",
	}, {
		name:         "one line with experimental (short)",
		short:        true,
		url:          "",
		experimental: true,
		lines:        []string{"this is one line"},
		want:         "this is one line",
	}, {

		name:         "multiple line (short)",
		short:        true,
		url:          "",
		experimental: false,
		lines:        []string{"this is one line", "this is a second line"},
		want:         "this is one line",
	}, {
		name:         "multiple line with url (short)",
		short:        true,
		url:          "https://example.com",
		experimental: false,
		lines:        []string{"this is one line", "this is a second line"},
		want:         "this is one line",
	}, {
		name:         "multiple line with experimental (short)",
		short:        true,
		url:          "",
		experimental: true,
		lines:        []string{"this is one line", "this is a second line"},
		want:         "this is one line",
	}, {
		name:         "one line (long)",
		short:        false,
		url:          "",
		experimental: false,
		lines:        []string{"this is one line"},
		want:         "this is one line",
	}, {
		name:         "one line with url (long)",
		short:        false,
		url:          "https://example.com",
		experimental: false,
		lines:        []string{"this is one line"},
		want:         "this is one line\n\n  For more information visit https://example.com",
	}, {
		name:         "one line with experimental (long)",
		short:        false,
		url:          "",
		experimental: true,
		lines:        []string{"this is one line"},
		want:         "this is one line\n\n  NOTE: This is an experimental command, use `KYVERNO_EXPERIMENTAL=true` to enable it.",
	}, {

		name:         "multiple line (long)",
		short:        false,
		url:          "",
		experimental: false,
		lines:        []string{"this is one line", "this is a second line"},
		want:         "this is one line\n  this is a second line",
	}, {
		name:         "multiple line with url (long)",
		short:        false,
		url:          "https://example.com",
		experimental: false,
		lines:        []string{"this is one line", "this is a second line"},
		want:         "this is one line\n  this is a second line\n\n  For more information visit https://example.com",
	}, {
		name:         "multiple line with experimental (long)",
		short:        false,
		url:          "",
		experimental: true,
		lines:        []string{"this is one line", "this is a second line"},
		want:         "this is one line\n  this is a second line\n\n  NOTE: This is an experimental command, use `KYVERNO_EXPERIMENTAL=true` to enable it.",
	}, {
		name:         "multiple line with url and experimental (long)",
		short:        false,
		url:          "https://example.com",
		experimental: true,
		lines:        []string{"this is one line", "this is a second line"},
		want:         "this is one line\n  this is a second line\n\n  NOTE: This is an experimental command, use `KYVERNO_EXPERIMENTAL=true` to enable it.\n\n  For more information visit https://example.com",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatDescription(tt.short, tt.url, tt.experimental, tt.lines...); got != tt.want {
				t.Errorf("FormatDescription() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatExamples(t *testing.T) {
	tests := []struct {
		name string
		in   [][]string
		want string
	}{{
		name: "nil",
		in:   nil,
		want: "",
	}, {
		name: "empty",
		in:   [][]string{},
		want: "",
	}, {
		name: "one",
		in: [][]string{{
			`# Fix Kyverno test files`,
			`KYVERNO_EXPERIMENTAL=true kyverno fix test . --save`,
		}},
		want: "  # Fix Kyverno test files\n  KYVERNO_EXPERIMENTAL=true kyverno fix test . --save",
	}, {
		name: "multiple",
		in: [][]string{{
			`# Test a git repository containing Kyverno test cases`,
			`kyverno test https://github.com/kyverno/policies/pod-security --git-branch main`,
		}, {
			`# Test a local folder containing test cases`,
			`kyverno test .`,
		}, {
			`# Test some specific test cases out of many test cases in a local folder`,
			`kyverno test . --test-case-selector "policy=disallow-latest-tag, rule=require-image-tag, resource=test-require-image-tag-pass"`,
		}},
		want: `  # Test a git repository containing Kyverno test cases
  kyverno test https://github.com/kyverno/policies/pod-security --git-branch main

  # Test a local folder containing test cases
  kyverno test .

  # Test some specific test cases out of many test cases in a local folder
  kyverno test . --test-case-selector "policy=disallow-latest-tag, rule=require-image-tag, resource=test-require-image-tag-pass"`,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatExamples(tt.in...); got != tt.want {
				t.Errorf("FormatExamples() = %v, want %v", got, tt.want)
			}
		})
	}
}
