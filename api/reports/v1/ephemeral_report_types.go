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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"
)

type EphemeralReportSpec struct {
	// Owner is a reference to the report owner (e.g. a Deployment, Namespace, or Node)
	Owner metav1.OwnerReference `json:"owner"`

	// PolicyReportSummary provides a summary of results
	// +optional
	Summary openreportsv1alpha1.ReportSummary `json:"summary,omitempty"`

	// PolicyReportResult provides result details
	// +optional
	Results []openreportsv1alpha1.ReportResult `json:"results,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=ephr,categories=kyverno
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=".metadata.labels['audit\\.kyverno\\.io/source']"
// +kubebuilder:printcolumn:name="Group",type=string,JSONPath=".metadata.labels['audit\\.kyverno\\.io/resource\\.group']"
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=".metadata.labels['audit\\.kyverno\\.io/resource\\.kind']"
// +kubebuilder:printcolumn:name="Owner",type=string,JSONPath=".metadata.annotations['audit\\.kyverno\\.io/resource\\.name']"
// +kubebuilder:printcolumn:name="Pass",type=integer,JSONPath=".spec.summary.pass"
// +kubebuilder:printcolumn:name="Fail",type=integer,JSONPath=".spec.summary.fail"
// +kubebuilder:printcolumn:name="Warn",type=integer,JSONPath=".spec.summary.warn"
// +kubebuilder:printcolumn:name="Error",type=integer,JSONPath=".spec.summary.error"
// +kubebuilder:printcolumn:name="Skip",type=integer,JSONPath=".spec.summary.skip"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Uid",type=string,JSONPath=".metadata.labels['audit\\.kyverno\\.io/resource\\.uid']",priority=1
// +kubebuilder:printcolumn:name="Hash",type=string,JSONPath=".metadata.labels['audit\\.kyverno\\.io/resource\\.hash']",priority=1

// EphemeralReport is the Schema for the EphemeralReports API
type EphemeralReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              EphemeralReportSpec `json:"spec"`
}

func (r *EphemeralReport) GetResults() []openreportsv1alpha1.ReportResult {
	return r.Spec.Results
}

func (r *EphemeralReport) SetResults(results []openreportsv1alpha1.ReportResult) {
	r.Spec.Results = results
}

func (r *EphemeralReport) SetSummary(summary openreportsv1alpha1.ReportSummary) {
	r.Spec.Summary = summary
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:scope=Cluster,shortName=cephr,categories=kyverno
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=".metadata.labels['audit\\.kyverno\\.io/source']"
// +kubebuilder:printcolumn:name="Group",type=string,JSONPath=".metadata.labels['audit\\.kyverno\\.io/resource\\.group']"
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=".metadata.labels['audit\\.kyverno\\.io/resource\\.kind']"
// +kubebuilder:printcolumn:name="Owner",type=string,JSONPath=".metadata.annotations['audit\\.kyverno\\.io/resource\\.name']"
// +kubebuilder:printcolumn:name="Pass",type=integer,JSONPath=".spec.summary.pass"
// +kubebuilder:printcolumn:name="Fail",type=integer,JSONPath=".spec.summary.fail"
// +kubebuilder:printcolumn:name="Warn",type=integer,JSONPath=".spec.summary.warn"
// +kubebuilder:printcolumn:name="Error",type=integer,JSONPath=".spec.summary.error"
// +kubebuilder:printcolumn:name="Skip",type=integer,JSONPath=".spec.summary.skip"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Uid",type=string,JSONPath=".metadata.labels['audit\\.kyverno\\.io/resource\\.uid']"
// +kubebuilder:printcolumn:name="Hash",type=string,JSONPath=".metadata.labels['audit\\.kyverno\\.io/resource\\.hash']",priority=1

// ClusterEphemeralReport is the Schema for the ClusterEphemeralReports API
type ClusterEphemeralReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              EphemeralReportSpec `json:"spec"`
}

func (r *ClusterEphemeralReport) GetResults() []openreportsv1alpha1.ReportResult {
	return r.Spec.Results
}

func (r *ClusterEphemeralReport) SetResults(results []openreportsv1alpha1.ReportResult) {
	r.Spec.Results = results
}

func (r *ClusterEphemeralReport) SetSummary(summary openreportsv1alpha1.ReportSummary) {
	r.Spec.Summary = summary
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EphemeralReportList contains a list of EphemeralReport
type EphemeralReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EphemeralReport `json:"items"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterEphemeralReportList contains a list of ClusterEphemeralReport
type ClusterEphemeralReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterEphemeralReport `json:"items"`
}
