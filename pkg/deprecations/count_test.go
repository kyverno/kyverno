package deprecations

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestCountLegacyPolicies(t *testing.T) {
	tests := []struct {
		name     string
		counters map[string]KindCounter
		want     map[string]int
		wantErr  bool
	}{
		{
			name:     "no counters",
			counters: map[string]KindCounter{},
			want:     map[string]int{},
		},
		{
			name: "mixed zero and non-zero counts",
			counters: map[string]KindCounter{
				"ClusterPolicy": func() (int, error) { return 3, nil },
				"Policy":        func() (int, error) { return 0, nil },
			},
			want: map[string]int{
				"ClusterPolicy": 3,
				"Policy":        0,
			},
		},
		{
			name: "all counters return non-zero",
			counters: map[string]KindCounter{
				"CleanupPolicy":        func() (int, error) { return 1, nil },
				"ClusterCleanupPolicy": func() (int, error) { return 2, nil },
			},
			want: map[string]int{
				"CleanupPolicy":        1,
				"ClusterCleanupPolicy": 2,
			},
		},
		{
			name: "a failing counter is aggregated into the error but does not block others",
			counters: map[string]KindCounter{
				"ClusterPolicy": func() (int, error) { return 5, nil },
				"Policy":        func() (int, error) { return 0, errors.New("list failed") },
			},
			want: map[string]int{
				"ClusterPolicy": 5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CountLegacyPolicies(tt.counters)
			if tt.wantErr && err == nil {
				t.Fatalf("CountLegacyPolicies() expected an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("CountLegacyPolicies() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CountLegacyPolicies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLegacyPolicyEventNote(t *testing.T) {
	tests := []struct {
		name   string
		counts map[string]int
		wantOK bool
	}{
		{
			name:   "no legacy policies",
			counts: map[string]int{"ClusterPolicy": 0},
			wantOK: false,
		},
		{
			name: "all five legacy kinds present",
			counts: map[string]int{
				"ClusterPolicy":        100000,
				"Policy":               100000,
				"CleanupPolicy":        100000,
				"ClusterCleanupPolicy": 100000,
				"PolicyException":      100000,
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note, ok := LegacyPolicyEventNote(tt.counts)
			if ok != tt.wantOK {
				t.Fatalf("LegacyPolicyEventNote() ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			// the Kubernetes Event Note field is truncated at 1024 bytes; stay
			// comfortably under that even in the worst case (all kinds present).
			if len(note) > 1024 {
				t.Errorf("LegacyPolicyEventNote() length = %d, want <= 1024", len(note))
			}
			if note == "" {
				t.Errorf("LegacyPolicyEventNote() expected a non-empty message")
			}
		})
	}
}

func TestLegacyPolicySummary(t *testing.T) {
	tests := []struct {
		name   string
		counts map[string]int
		wantOK bool
	}{
		{
			name:   "no legacy policies",
			counts: map[string]int{"ClusterPolicy": 0, "Policy": 0},
			wantOK: false,
		},
		{
			name:   "empty counts",
			counts: map[string]int{},
			wantOK: false,
		},
		{
			name:   "one legacy kind present",
			counts: map[string]int{"ClusterPolicy": 2, "Policy": 0},
			wantOK: true,
		},
		{
			name:   "multiple legacy kinds present",
			counts: map[string]int{"ClusterPolicy": 2, "CleanupPolicy": 1},
			wantOK: true,
		},
		{
			name:   "unknown kind is ignored",
			counts: map[string]int{"SomeOtherKind": 5},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, ok := LegacyPolicySummary(tt.counts)
			if ok != tt.wantOK {
				t.Fatalf("LegacyPolicySummary() ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK && message == "" {
				t.Errorf("LegacyPolicySummary() expected a non-empty message")
			}
			if !tt.wantOK && message != "" {
				t.Errorf("LegacyPolicySummary() expected an empty message, got %q", message)
			}
		})
	}
}

// TestLegacyPolicySummaryShape locks in the compact, non-repetitive rendering:
// the removal notice and migration URL appear exactly once (in the preamble),
// and each kind is listed as "Kind=count (→ replacement)" without repeating the
// per-kind boilerplate.
func TestLegacyPolicySummaryShape(t *testing.T) {
	counts := map[string]int{
		"ClusterPolicy":        1,
		"Policy":               2,
		"CleanupPolicy":        3,
		"ClusterCleanupPolicy": 4,
		"PolicyException":      5,
	}
	message, ok := LegacyPolicySummary(counts)
	if !ok {
		t.Fatalf("LegacyPolicySummary() ok = false, want true")
	}

	// URL stated once, in the preamble -- not repeated per kind.
	if got := strings.Count(message, MigrationGuideURL); got != 1 {
		t.Errorf("migration URL appears %d times, want exactly 1; message = %q", got, message)
	}
	// The old per-kind boilerplate must be gone.
	if strings.Contains(message, "is deprecated and will be removed") {
		t.Errorf("message still repeats per-kind deprecation boilerplate: %q", message)
	}
	// Each kind rendered as "Kind=count (→ replacement)", drawing the replacement
	// from the shared deprecations table.
	for kind, count := range counts {
		warning, _ := BuildKindWarning("kyverno.io", "", kind)
		fragment := fmt.Sprintf("%s=%d (→ %s)", kind, count, warning.Replacement)
		if !strings.Contains(message, fragment) {
			t.Errorf("message missing fragment %q; message = %q", fragment, message)
		}
	}
}
