package annotations

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"
)

func TestShouldSkipReport(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    bool
	}{
		{
			name:        "no annotations",
			annotations: nil,
			expected:    false,
		},
		{
			name:        "skip reports true",
			annotations: map[string]string{AnnotationSkipReports: "true"},
			expected:    true,
		},
		{
			name:        "skip reports false",
			annotations: map[string]string{AnnotationSkipReports: "false"},
			expected:    false,
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			expected:    false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &metav1.ObjectMeta{
				Annotations: tt.annotations,
			}
			result := ShouldSkipReport(obj)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldSkipReportStatus(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		status      string
		expected    bool
	}{
		{
			name:        "skip specific status",
			annotations: map[string]string{AnnotationSkipReportStatuses: "PolicyApplied,PolicySkipped"},
			status:      "PolicyApplied",
			expected:    true,
		},
		{
			name:        "don't skip other status",
			annotations: map[string]string{AnnotationSkipReportStatuses: "PolicyApplied"},
			status:      "PolicyViolation",
			expected:    false,
		},
		{
			name:        "skip all reports overrides status",
			annotations: map[string]string{AnnotationSkipReports: "true"},
			status:      "PolicyViolation",
			expected:    true,
		},
		{
			name:        "whitespace handling",
			annotations: map[string]string{AnnotationSkipReportStatuses: " PolicyApplied , PolicySkipped "},
			status:      "PolicySkipped",
			expected:    true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &metav1.ObjectMeta{
				Annotations: tt.annotations,
			}
			result := ShouldSkipReportStatus(obj, tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}
