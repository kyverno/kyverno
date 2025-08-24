package annotations

import (
	"testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockPolicy embeds ObjectMeta to implement all required interface methods
type MockPolicy struct {
	metav1.ObjectMeta
	annotations map[string]string
}

// Override GetAnnotations to use our test data
func (m *MockPolicy) GetAnnotations() map[string]string {
	return m.annotations
}

func TestIntegrationSkipReports(t *testing.T) {
	// Test policy with skip annotation
	policyWithSkip := &MockPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
		annotations: map[string]string{
			AnnotationSkipReports: "true",
		},
	}

	// Test policy without skip annotation
	policyWithoutSkip := &MockPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "normal-policy"},
		annotations: map[string]string{},
	}

	// Test that skip annotation works
	if !ShouldSkipReport(policyWithSkip) {
		t.Error("Expected to skip report for policy with skip annotation")
	}

	// Test that normal policy doesn't skip
	if ShouldSkipReport(policyWithoutSkip) {
		t.Error("Expected NOT to skip report for policy without skip annotation")
	}
}
