package annotations

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

const (
	AnnotationSkipReports       = "kyverno.io/skip-reports"
	AnnotationSkipReportStatuses = "kyverno.io/skip-report-statuses"
)

// ShouldSkipReport checks if reports should be skipped for a policy
func ShouldSkipReport(obj metav1.Object) bool {
	if obj == nil {
		return false
	}
	
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}
	
	if skipAll, exists := annotations[AnnotationSkipReports]; exists {
		return skipAll == "true"
	}
	
	return false
}

// ShouldSkipReportStatus checks if a specific status should be skipped
func ShouldSkipReportStatus(obj metav1.Object, status string) bool {
	if obj == nil {
		return false
	}
	
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}
	
	// First check if all reports are skipped
	if ShouldSkipReport(obj) {
		return true
	}
	
	// Check for specific status skipping
	if skipStatuses, exists := annotations[AnnotationSkipReportStatuses]; exists {
		statuses := strings.Split(skipStatuses, ",")
		for _, s := range statuses {
			if strings.TrimSpace(s) == status {
				return true
			}
		}
	}
	
	return false
}
