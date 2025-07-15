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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"
)

const kyvernoSource = "kyverno"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=".scope.kind"
// +kubebuilder:printcolumn:name="Name",type=string,JSONPath=".scope.name"
// +kubebuilder:printcolumn:name="Pass",type=integer,JSONPath=".summary.pass"
// +kubebuilder:printcolumn:name="Fail",type=integer,JSONPath=".summary.fail"
// +kubebuilder:printcolumn:name="Warn",type=integer,JSONPath=".summary.warn"
// +kubebuilder:printcolumn:name="Error",type=integer,JSONPath=".summary.error"
// +kubebuilder:printcolumn:name="Skip",type=integer,JSONPath=".summary.skip"
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
	Results []PolicyReportResult `json:"results,omitempty"`
}

func (r *PolicyReport) GetResults() []PolicyReportResult {
	return r.Results
}

func (r *PolicyReport) SetResults(results []PolicyReportResult) {
	r.Results = results
}

func (r *PolicyReport) SetSummary(summary PolicyReportSummary) {
	r.Summary = summary
}

func (polr *PolicyReport) ToOpenReports() *openreportsv1alpha1.Report {
	res := []openreportsv1alpha1.ReportResult{}
	for _, r := range polr.GetResults() {
		res = append(res, openreportsv1alpha1.ReportResult{
			Source:           r.Source,
			Policy:           r.Policy,
			Rule:             r.Rule,
			Category:         r.Category,
			Timestamp:        r.Timestamp,
			Severity:         openreportsv1alpha1.ResultSeverity(r.Severity),
			Result:           openreportsv1alpha1.Result(r.Result),
			Subjects:         r.Resources,
			ResourceSelector: r.ResourceSelector,
			Scored:           r.Scored,
			Description:      r.Message,
			Properties:       r.Properties,
		})
	}
	return &openreportsv1alpha1.Report{
		ObjectMeta:    polr.ObjectMeta,
		Scope:         polr.Scope,
		ScopeSelector: polr.ScopeSelector,
		Source:        kyvernoSource,
		Summary: openreportsv1alpha1.ReportSummary{
			Pass:  polr.Summary.Pass,
			Fail:  polr.Summary.Fail,
			Warn:  polr.Summary.Warn,
			Error: polr.Summary.Error,
			Skip:  polr.Summary.Skip,
		},
		Results: res,
	}
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyReportList contains a list of PolicyReport
type PolicyReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PolicyReport `json:"items"`
}
