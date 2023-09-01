/*
Copyright 2020 The Kubernetes authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha2

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Status specifies state of a policy result
const (
	StatusPass  PolicyResult = "pass"
	StatusFail  PolicyResult = "fail"
	StatusWarn  PolicyResult = "warn"
	StatusError PolicyResult = "error"
	StatusSkip  PolicyResult = "skip"
)

// Severity specifies priority of a policy result
const (
	SeverityCritical PolicySeverity = "critical"
	SeverityHigh     PolicySeverity = "high"
	SeverityMedium   PolicySeverity = "medium"
	SeverityLow      PolicySeverity = "low"
	SeverityInfo     PolicySeverity = "info"
)

// PolicyReportSummary provides a status count summary
type PolicyReportSummary struct {
	// Pass provides the count of policies whose requirements were met
	// +optional
	Pass int `json:"pass"`

	// Fail provides the count of policies whose requirements were not met
	// +optional
	Fail int `json:"fail"`

	// Warn provides the count of non-scored policies whose requirements were not met
	// +optional
	Warn int `json:"warn"`

	// Error provides the count of policies that could not be evaluated
	// +optional
	Error int `json:"error"`

	// Skip indicates the count of policies that were not selected for evaluation
	// +optional
	Skip int `json:"skip"`
}

func (prs PolicyReportSummary) ToMap() map[string]interface{} {
	b, _ := json.Marshal(&prs)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	return m
}

// +kubebuilder:validation:Enum=pass;fail;warn;error;skip

// PolicyResult has one of the following values:
//   - pass: indicates that the policy requirements are met
//   - fail: indicates that the policy requirements are not met
//   - warn: indicates that the policy requirements and not met, and the policy is not scored
//   - error: indicates that the policy could not be evaluated
//   - skip: indicates that the policy was not selected based on user inputs or applicability
type PolicyResult string

// +kubebuilder:validation:Enum=critical;high;low;medium;info

// PolicySeverity has one of the following values:
// - critical
// - high
// - low
// - medium
// - info
type PolicySeverity string

// PolicyReportResult provides the result for an individual policy
type PolicyReportResult struct {
	// Source is an identifier for the policy engine that manages this report
	// +optional
	Source string `json:"source"`

	// Policy is the name or identifier of the policy
	Policy string `json:"policy"`

	// Rule is the name or identifier of the rule within the policy
	// +optional
	Rule string `json:"rule,omitempty"`

	// Subjects is an optional reference to the checked Kubernetes resources
	// +optional
	Resources []corev1.ObjectReference `json:"resources,omitempty"`

	// SubjectSelector is an optional label selector for checked Kubernetes resources.
	// For example, a policy result may apply to all pods that match a label.
	// Either a Subject or a SubjectSelector can be specified.
	// If neither are provided, the result is assumed to be for the policy report scope.
	// +optional
	ResourceSelector *metav1.LabelSelector `json:"resourceSelector,omitempty"`

	// Description is a short user friendly message for the policy rule
	Message string `json:"message,omitempty"`

	// Result indicates the outcome of the policy rule execution
	Result PolicyResult `json:"result,omitempty"`

	// Scored indicates if this result is scored
	Scored bool `json:"scored,omitempty"`

	// Properties provides additional information for the policy rule
	Properties map[string]string `json:"properties,omitempty"`

	// Timestamp indicates the time the result was found
	Timestamp metav1.Timestamp `json:"timestamp,omitempty"`

	// Category indicates policy category
	// +optional
	Category string `json:"category,omitempty"`

	// Severity indicates policy check result criticality
	// +optional
	Severity PolicySeverity `json:"severity,omitempty"`
}
