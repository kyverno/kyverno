package openreports

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"
)

// Status specifies state of a policy result
const (
	StatusPass  = "pass"
	StatusFail  = "fail"
	StatusWarn  = "warn"
	StatusError = "error"
	StatusSkip  = "skip"
)

// Severity specifies priority of a policy result
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
	SeverityInfo     = "info"
)

// PolicyResult has one of the following values:
//   - pass: indicates that the policy requirements are met
//   - fail: indicates that the policy requirements are not met
//   - warn: indicates that the policy requirements and not met, and the policy is not scored
//   - error: indicates that the policy could not be evaluated
//   - skip: indicates that the policy was not selected based on user inputs or applicability
type PolicyResult string

// PolicySeverity has one of the following values:
// - critical
// - high
// - low
// - medium
// - info
type Severity string

var SeverityLevel = map[openreportsv1alpha1.ResultSeverity]int{
	"":               -1,
	SeverityInfo:     0,
	SeverityLow:      1,
	SeverityMedium:   2,
	SeverityHigh:     3,
	SeverityCritical: 4,
}

type ReportInterface interface {
	metav1.Object
	GetResults() []openreportsv1alpha1.ReportResult
	SetResults([]openreportsv1alpha1.ReportResult)
	SetSummary(openreportsv1alpha1.ReportSummary)
}
