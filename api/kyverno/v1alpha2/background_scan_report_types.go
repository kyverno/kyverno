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
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=bgscanr
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=`.owner.kind`,priority=1
// +kubebuilder:printcolumn:name="Name",type=string,JSONPath=`.owner.name`,priority=1
// +kubebuilder:printcolumn:name="Pass",type=integer,JSONPath=`.summary.pass`
// +kubebuilder:printcolumn:name="Fail",type=integer,JSONPath=`.summary.fail`
// +kubebuilder:printcolumn:name="Warn",type=integer,JSONPath=`.summary.warn`
// +kubebuilder:printcolumn:name="Error",type=integer,JSONPath=`.summary.error`
// +kubebuilder:printcolumn:name="Skip",type=integer,JSONPath=`.summary.skip`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// BackgroundScanReport is the Schema for the BackgroundScanReports API
type BackgroundScanReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Owner is a reference to the report owner (e.g. a Deployment, Namespace, or Node)
	Owner metav1.OwnerReference `json:"owner"`

	// PolicyReportSummary provides a summary of results
	// +optional
	Summary policyreportv1alpha2.PolicyReportSummary `json:"summary,omitempty"`

	// PolicyReportResult provides result details
	// +optional
	Results []policyreportv1alpha2.PolicyReportResult `json:"results,omitempty"`
}

func (r *BackgroundScanReport) GetResults() []policyreportv1alpha2.PolicyReportResult {
	return r.Results
}

func (r *BackgroundScanReport) SetResults(results []policyreportv1alpha2.PolicyReportResult) {
	r.Results = results
}

func (r *BackgroundScanReport) SetSummary(summary policyreportv1alpha2.PolicyReportSummary) {
	r.Summary = summary
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:scope=Cluster,shortName=cbgscanr
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=`.owner.kind`,priority=1
// +kubebuilder:printcolumn:name="Name",type=string,JSONPath=`.owner.name`,priority=1
// +kubebuilder:printcolumn:name="Pass",type=integer,JSONPath=`.summary.pass`
// +kubebuilder:printcolumn:name="Fail",type=integer,JSONPath=`.summary.fail`
// +kubebuilder:printcolumn:name="Warn",type=integer,JSONPath=`.summary.warn`
// +kubebuilder:printcolumn:name="Error",type=integer,JSONPath=`.summary.error`
// +kubebuilder:printcolumn:name="Skip",type=integer,JSONPath=`.summary.skip`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterBackgroundScanReport is the Schema for the ClusterBackgroundScanReports API
type ClusterBackgroundScanReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Owner is a reference to the report owner (e.g. a Deployment, Namespace, or Node)
	Owner metav1.OwnerReference `json:"owner"`

	// PolicyReportSummary provides a summary of results
	// +optional
	Summary policyreportv1alpha2.PolicyReportSummary `json:"summary,omitempty"`

	// PolicyReportResult provides result details
	// +optional
	Results []policyreportv1alpha2.PolicyReportResult `json:"results,omitempty"`
}

func (r *ClusterBackgroundScanReport) GetResults() []policyreportv1alpha2.PolicyReportResult {
	return r.Results
}

func (r *ClusterBackgroundScanReport) SetResults(results []policyreportv1alpha2.PolicyReportResult) {
	r.Results = results
}

func (r *ClusterBackgroundScanReport) SetSummary(summary policyreportv1alpha2.PolicyReportSummary) {
	r.Summary = summary
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackgroundScanReportList contains a list of BackgroundScanReport
type BackgroundScanReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BackgroundScanReport `json:"items"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterBackgroundScanReportList contains a list of ClusterBackgroundScanReport
type ClusterBackgroundScanReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterBackgroundScanReport `json:"items"`
}
