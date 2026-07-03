package report

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestNewBackgroundScanReport_NameTruncation(t *testing.T) {
	t.Parallel()

	gvk := schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}
	uid := types.UID("test-uid-1234")

	// exactly 57 'a' characters
	exactly57 := strings.Repeat("a", 57)

	// 80-character name; first 57 chars are "this-is-a-very-long-validating-adm"... let's be explicit
	longName := "this-is-a-very-long-validating-admission-policy-binding-name-that-exceeds-limit"

	tests := []struct {
		name       string
		inputName  string
		wantLen    int    // expected len of GenerateName (includes trailing hyphen)
		wantPrefix string // first N chars before the hyphen
	}{
		{
			name:       "short name passes through unchanged",
			inputName:  "short-binding",
			wantLen:    len("short-binding") + 1, // +1 for trailing "-"
			wantPrefix: "short-binding",
		},
		{
			name:       "name exactly 57 chars passes through unchanged",
			inputName:  exactly57,
			wantLen:    58, // 57 + trailing "-"
			wantPrefix: exactly57,
		},
		{
			name:       "name over 57 chars is truncated to 57",
			inputName:  longName,
			wantLen:    58, // 57 + trailing "-"
			wantPrefix: longName[:57],
		},
		{
			name:       "57th character is a period gets stripped",
			inputName:  strings.Repeat("a", 56) + "." + "bbbbbbbbbbbbbbbbbbbbbbb",
			wantLen:    57, // 56 chars + "-"
			wantPrefix: strings.Repeat("a", 56),
		},
		{
			name:       "57th character is underscore gets stripped",
			inputName:  strings.Repeat("a", 56) + "_" + "ccccccccccccccccccccccc",
			wantLen:    57, // 56 chars + "-"
			wantPrefix: strings.Repeat("a", 56),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			report := NewBackgroundScanReport("test-namespace", tt.inputName, gvk, "test-owner", uid)
			got := report.GetGenerateName()

			if len(got) != tt.wantLen {
				t.Errorf("GetGenerateName() length = %d, want %d (value: %q)", len(got), tt.wantLen, got)
			}
			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("GetGenerateName() = %q, want prefix %q", got, tt.wantPrefix)
			}
			if !strings.HasSuffix(got, "-") {
				t.Errorf("GetGenerateName() = %q, must end with '-'", got)
			}
		})
	}
}