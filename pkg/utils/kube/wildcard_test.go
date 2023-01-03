package kube

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLabelSelectorContainsWildcardPositive(t *testing.T) {
	// Test that the LabelSelectorContainsWildcard function returns true when a wildcard is present in the MatchLabels map
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"key": "val*",
		},
	}
	result := LabelSelectorContainsWildcard(selector)
	if !result {
		t.Errorf("Expected LabelSelectorContainsWildcard to return true, got %v", result)
	}
}

func TestLabelSelectorContainsWildcardNegative(t *testing.T) {
	// Test that the LabelSelectorContainsWildcard function returns false when no wildcards are present in the MatchLabels map
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"key": "val",
		},
	}
	result := LabelSelectorContainsWildcard(selector)
	if result {
		t.Errorf("Expected LabelSelectorContainsWildcard to return false, got %v", result)
	}
}

func TestLabelSelectorContainsWildcardEmptySelector(t *testing.T) {
	// Test that the LabelSelectorContainsWildcard function returns false when given an empty LabelSelector
	var selector metav1.LabelSelector
	result := LabelSelectorContainsWildcard(&selector)
	if result {
		t.Errorf("Expected LabelSelectorContainsWildcard to return false, got %v", result)
	}
}

func TestLabelSelectorContainsWildcardNilSelector(t *testing.T) {
	// Test that the LabelSelectorContainsWildcard function returns false when given a nil LabelSelector
	result := LabelSelectorContainsWildcard(nil)
	if result {
		t.Errorf("Expected LabelSelectorContainsWildcard to return false, got %v", result)
	}
}
