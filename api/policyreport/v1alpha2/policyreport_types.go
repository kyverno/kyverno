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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

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
	SeverityHigh   = "high"
	SeverityMedium = "medium"
	SeverityLow    = "low"
)

// PolicyReportSummary provides a status count summary
type PolicyReportSummary struct {

	// Pass provides the count of policies whose requirements were met
	// +optional
	Pass int `json:"pass"`

	// Fail provides the count of policies whose requirements were not met
	// +optional
	Fail int `json:"fail"`

	// Warn provides the count of unscored policies whose requirements were not met
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

// PolicyResult has one of the following values:
//   - pass: indicates that the policy requirements are met
//   - fail: indicates that the policy requirements are not met
//   - warn: indicates that the policy requirements and not met, and the policy is not scored
//   - error: indicates that the policy could not be evaluated
//   - skip: indicates that the policy was not selected based on user inputs or applicability
//
// +kubebuilder:validation:Enum=pass;fail;warn;error;skip
type PolicyResult string

// PolicySeverity has one of the following values:
//   - high
//   - low
//   - medium
// +kubebuilder:validation:Enum=high;low;medium
type PolicySeverity string

// PolicyReportResult provides the result for an individual policy
type PolicyReportResult struct {

	// Source is an identifier for the policy engine that manages this report
	// +optional
	Source string `json:"source"`

	// Policy is the name of the policy
	Policy string `json:"policy"`

	// Rule is the name of the policy rule
	// +optional
	Rule string `json:"rule,omitempty"`

	// Resources is an optional reference to the resource checked by the policy and rule
	// +optional
	Resources []*corev1.ObjectReference `json:"resources,omitempty"`

	// ResourceSelector is an optional selector for policy results that apply to multiple resources.
	// For example, a policy result may apply to all pods that match a label.
	// Either a Resource or a ResourceSelector can be specified. If neither are provided, the
	// result is assumed to be for the policy report scope.
	// +optional
	ResourceSelector *metav1.LabelSelector `json:"resourceSelector,omitempty"`

	// Message is a short user friendly description of the policy rule
	Message string `json:"message,omitempty"`

	// Result indicates the outcome of the policy rule execution
	Result PolicyResult `json:"result,omitempty"`

	// Scored indicates if this policy rule is scored
	Scored bool `json:"scored,omitempty"`

	// Properties provides additional information for the policy rule
	Properties map[string]string `json:"properties,omitempty"`

	// Timestamp indicates the time the result was found
	Timestamp metav1.Timestamp `json:"timestamp,omitempty"`

	// Category indicates policy category
	// +optional
	Category string `json:"category,omitempty"`

	// Severity indicates policy severity
	// +optional
	Severity PolicySeverity `json:"severity,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=`.scope.kind`,priority=1
// +kubebuilder:printcolumn:name="Name",type=string,JSONPath=`.scope.name`,priority=1
// +kubebuilder:printcolumn:name="Pass",type=integer,JSONPath=`.summary.pass`
// +kubebuilder:printcolumn:name="Fail",type=integer,JSONPath=`.summary.fail`
// +kubebuilder:printcolumn:name="Warn",type=integer,JSONPath=`.summary.warn`
// +kubebuilder:printcolumn:name="Error",type=integer,JSONPath=`.summary.error`
// +kubebuilder:printcolumn:name="Skip",type=integer,JSONPath=`.summary.skip`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:shortName=polr

// PolicyReport is the Schema for the policyreports API
type PolicyReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Scope is an optional reference to the report scope (e.g. a Deployment, Namespace, or Node)
	// +optional
	Scope *corev1.ObjectReference `json:"scope,omitempty"`

	// ScopeSelector is an optional selector for multiple scopes (e.g. Pods).
	// Either one of, or none of, but not both of, Scope or ScopeSelector should be specified.
	// +optional
	ScopeSelector *metav1.LabelSelector `json:"scopeSelector,omitempty"`

	// PolicyReportSummary provides a summary of results
	// +optional
	Summary PolicyReportSummary `json:"summary,omitempty"`

	// PolicyReportResult provides result details
	// +optional
	Results []*PolicyReportResult `json:"results,omitempty"`
}

// PolicyReportList contains a list of PolicyReport
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PolicyReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PolicyReport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PolicyReport{}, &PolicyReportList{})
}
