package common

import (
	"strings"
	"testing"
)

func TestGenerateLabelConstants(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		contains string
	}{
		{"GeneratePolicyLabel", GeneratePolicyLabel, "policy-name"},
		{"GeneratePolicyNamespaceLabel", GeneratePolicyNamespaceLabel, "policy-namespace"},
		{"GenerateRuleLabel", GenerateRuleLabel, "rule-name"},
		{"GenerateTriggerNameLabel", GenerateTriggerNameLabel, "trigger-name"},
		{"GenerateTriggerUIDLabel", GenerateTriggerUIDLabel, "trigger-uid"},
		{"GenerateTriggerNSLabel", GenerateTriggerNSLabel, "trigger-namespace"},
		{"GenerateTriggerKindLabel", GenerateTriggerKindLabel, "trigger-kind"},
		{"GenerateTriggerVersionLabel", GenerateTriggerVersionLabel, "trigger-version"},
		{"GenerateTriggerGroupLabel", GenerateTriggerGroupLabel, "trigger-group"},
		{"GenerateSourceNameLabel", GenerateSourceNameLabel, "source-name"},
		{"GenerateSourceUIDLabel", GenerateSourceUIDLabel, "source-uid"},
		{"GenerateSourceNSLabel", GenerateSourceNSLabel, "source-namespace"},
		{"GenerateSourceKindLabel", GenerateSourceKindLabel, "source-kind"},
		{"GenerateSourceVersionLabel", GenerateSourceVersionLabel, "source-version"},
		{"GenerateSourceGroupLabel", GenerateSourceGroupLabel, "source-group"},
		{"GenerateTypeCloneSourceLabel", GenerateTypeCloneSourceLabel, "clone-source"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.HasPrefix(tt.label, "generate.kyverno.io/") {
				t.Errorf("%s = %v, should start with generate.kyverno.io/", tt.name, tt.label)
			}
			if !strings.Contains(tt.label, tt.contains) {
				t.Errorf("%s = %v, should contain %v", tt.name, tt.label, tt.contains)
			}
		})
	}
}

func TestLabelUniqueness(t *testing.T) {
	labels := []string{
		GeneratePolicyLabel,
		GeneratePolicyNamespaceLabel,
		GenerateRuleLabel,
		GenerateTriggerNameLabel,
		GenerateTriggerUIDLabel,
		GenerateTriggerNSLabel,
		GenerateTriggerKindLabel,
		GenerateTriggerVersionLabel,
		GenerateTriggerGroupLabel,
		GenerateSourceNameLabel,
		GenerateSourceUIDLabel,
		GenerateSourceNSLabel,
		GenerateSourceKindLabel,
		GenerateSourceVersionLabel,
		GenerateSourceGroupLabel,
		GenerateTypeCloneSourceLabel,
	}

	seen := make(map[string]bool)
	for _, label := range labels {
		if seen[label] {
			t.Errorf("duplicate label found: %v", label)
		}
		seen[label] = true
	}
}
