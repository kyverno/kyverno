package api

import (
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestGetValidationActionsFromStrings(t *testing.T) {
	tests := []struct {
		name     string
		actions  []string
		expected sets.Set[admissionregistrationv1.ValidationAction]
	}{
		{
			name:     "empty actions",
			actions:  []string{},
			expected: sets.New[admissionregistrationv1.ValidationAction](),
		},
		{
			name:     "single Deny action",
			actions:  []string{"Deny"},
			expected: sets.New(admissionregistrationv1.Deny),
		},
		{
			name:     "single Warn action",
			actions:  []string{"Warn"},
			expected: sets.New(admissionregistrationv1.Warn),
		},
		{
			name:     "single Audit action",
			actions:  []string{"Audit"},
			expected: sets.New(admissionregistrationv1.Audit),
		},
		{
			name:     "multiple actions",
			actions:  []string{"Warn", "Audit"},
			expected: sets.New(admissionregistrationv1.Warn, admissionregistrationv1.Audit),
		},
		{
			name:     "lowercase actions",
			actions:  []string{"warn", "audit", "deny"},
			expected: sets.New(admissionregistrationv1.Warn, admissionregistrationv1.Audit, admissionregistrationv1.Deny),
		},
		{
			name:     "mixed case actions",
			actions:  []string{"Warn", "audit", "Deny"},
			expected: sets.New(admissionregistrationv1.Warn, admissionregistrationv1.Audit, admissionregistrationv1.Deny),
		},
		{
			name:     "invalid actions are ignored",
			actions:  []string{"Warn", "Invalid", "Audit"},
			expected: sets.New(admissionregistrationv1.Warn, admissionregistrationv1.Audit),
		},
		{
			name:     "duplicate actions",
			actions:  []string{"Warn", "Warn", "Audit"},
			expected: sets.New(admissionregistrationv1.Warn, admissionregistrationv1.Audit),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetValidationActionsFromStrings(tt.actions)

			if result.Len() != tt.expected.Len() {
				t.Errorf("GetValidationActionsFromStrings() length = %v, expected %v", result.Len(), tt.expected.Len())
			}

			for action := range tt.expected {
				if !result.Has(action) {
					t.Errorf("GetValidationActionsFromStrings() missing expected action %v", action)
				}
			}

			for action := range result {
				if !tt.expected.Has(action) {
					t.Errorf("GetValidationActionsFromStrings() has unexpected action %v", action)
				}
			}
		})
	}
}
