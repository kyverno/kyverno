package deprecations

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestFindOrphanedWebhookConfigs(t *testing.T) {
	tests := []struct {
		name     string
		checkers map[string]WebhookExistenceChecker
		want     []string
		wantErr  bool
	}{
		{
			name:     "no checkers",
			checkers: map[string]WebhookExistenceChecker{},
			want:     nil,
		},
		{
			name: "mixed present and absent",
			checkers: map[string]WebhookExistenceChecker{
				"kyverno-validating-webhook-cfg":              func() (bool, error) { return true, nil },
				"kyverno-resource-mutating-webhook-cfg-debug": func() (bool, error) { return false, nil },
			},
			want: []string{"kyverno-validating-webhook-cfg"},
		},
		{
			name: "all present",
			checkers: map[string]WebhookExistenceChecker{
				"kyverno-policy-validating-webhook-cfg-debug": func() (bool, error) { return true, nil },
				"kyverno-policy-mutating-webhook-cfg-debug":   func() (bool, error) { return true, nil },
			},
			want: []string{"kyverno-policy-mutating-webhook-cfg-debug", "kyverno-policy-validating-webhook-cfg-debug"},
		},
		{
			name: "a failing checker is aggregated into the error but does not block others",
			checkers: map[string]WebhookExistenceChecker{
				"kyverno-validating-webhook-cfg-debug": func() (bool, error) { return true, nil },
				"kyverno-verify-mutating-webhook-cfg-debug": func() (bool, error) {
					return false, errors.New("get failed")
				},
			},
			want:    []string{"kyverno-validating-webhook-cfg-debug"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindOrphanedWebhookConfigs(tt.checkers)
			if tt.wantErr && err == nil {
				t.Fatalf("FindOrphanedWebhookConfigs() expected an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("FindOrphanedWebhookConfigs() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindOrphanedWebhookConfigs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrphanedWebhookConfigSummary(t *testing.T) {
	tests := []struct {
		name   string
		found  []string
		wantOK bool
	}{
		{
			name:   "none found",
			found:  nil,
			wantOK: false,
		},
		{
			name:   "empty slice",
			found:  []string{},
			wantOK: false,
		},
		{
			name:   "one found",
			found:  []string{"kyverno-validating-webhook-cfg"},
			wantOK: true,
		},
		{
			name:   "multiple found",
			found:  []string{"kyverno-validating-webhook-cfg", "kyverno-resource-mutating-webhook-cfg-debug"},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, ok := OrphanedWebhookConfigSummary(tt.found)
			if ok != tt.wantOK {
				t.Fatalf("OrphanedWebhookConfigSummary() ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK {
				if message == "" {
					t.Errorf("OrphanedWebhookConfigSummary() expected a non-empty message")
				}
				if !strings.Contains(message, "kubectl delete") {
					t.Errorf("OrphanedWebhookConfigSummary() = %q, expected remediation guidance", message)
				}
				for _, name := range tt.found {
					if !strings.Contains(message, name) {
						t.Errorf("OrphanedWebhookConfigSummary() = %q, missing name %q", message, name)
					}
				}
			} else if message != "" {
				t.Errorf("OrphanedWebhookConfigSummary() expected an empty message, got %q", message)
			}
		})
	}
}

func TestOrphanedWebhookConfigEventNote(t *testing.T) {
	tests := []struct {
		name   string
		found  []string
		wantOK bool
	}{
		{
			name:   "none found",
			found:  nil,
			wantOK: false,
		},
		{
			name: "all known names present",
			found: append(
				append([]string(nil), OrphanedValidatingWebhookConfigNames...),
				OrphanedMutatingWebhookConfigNames...,
			),
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note, ok := OrphanedWebhookConfigEventNote(tt.found)
			if ok != tt.wantOK {
				t.Fatalf("OrphanedWebhookConfigEventNote() ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			// the Kubernetes Event Note field is truncated at 1024 bytes; stay
			// comfortably under that even in the worst case (all names present).
			if len(note) > 1024 {
				t.Errorf("OrphanedWebhookConfigEventNote() length = %d, want <= 1024", len(note))
			}
			if note == "" {
				t.Errorf("OrphanedWebhookConfigEventNote() expected a non-empty message")
			}
		})
	}
}
