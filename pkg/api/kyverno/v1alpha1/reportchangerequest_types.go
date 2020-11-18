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

package v1alpha1

import (
	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=`.scope.kind`,priority=1
// +kubebuilder:printcolumn:name="Name",type=string,JSONPath=`.scope.name`,priority=1
// +kubebuilder:printcolumn:name="Pass",type=integer,JSONPath=`.summary.pass`
// +kubebuilder:printcolumn:name="Fail",type=integer,JSONPath=`.summary.fail`
// +kubebuilder:printcolumn:name="Warn",type=integer,JSONPath=`.summary.warn`
// +kubebuilder:printcolumn:name="Error",type=integer,JSONPath=`.summary.error`
// +kubebuilder:printcolumn:name="Skip",type=integer,JSONPath=`.summary.skip`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ReportChangeRequest is the Schema for the ReportChangeRequests API
type ReportChangeRequest struct {
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
	Summary report.PolicyReportSummary `json:"summary,omitempty"`

	// PolicyReportResult provides result details
	// +optional
	Results []*report.PolicyReportResult `json:"results,omitempty"`
}

// ReportChangeRequestList contains a list of ReportChangeRequest
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ReportChangeRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReportChangeRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReportChangeRequest{}, &ReportChangeRequestList{})
}
