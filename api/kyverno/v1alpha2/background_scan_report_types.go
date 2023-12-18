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

type BackgroundScanReportSpec struct {
	// PolicyReportSummary provides a summary of results
	// +optional
	Summary policyreportv1alpha2.PolicyReportSummary `json:"summary,omitempty"`

	// PolicyReportResult provides result details
	// +optional
	Results []policyreportv1alpha2.PolicyReportResult `json:"results,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=bgscanr,categories=kyverno
// +kubebuilder:printcolumn:name="ApiVersion",type=string,JSONPath=".metadata.ownerReferences[0].apiVersion"
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=".metadata.ownerReferences[0].kind"
// +kubebuilder:printcolumn:name="Subject",type=string,JSONPath=".metadata.ownerReferences[0].name"
// +kubebuilder:printcolumn:name="Pass",type=integer,JSONPath=".spec.summary.pass"
// +kubebuilder:printcolumn:name="Fail",type=integer,JSONPath=".spec.summary.fail"
// +kubebuilder:printcolumn:name="Warn",type=integer,JSONPath=".spec.summary.warn"
// +kubebuilder:printcolumn:name="Error",type=integer,JSONPath=".spec.summary.error"
// +kubebuilder:printcolumn:name="Skip",type=integer,JSONPath=".spec.summary.skip"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Hash",type=string,JSONPath=".metadata.labels['audit\\.kyverno\\.io/resource\\.hash']",priority=1

// BackgroundScanReport is the Schema for the BackgroundScanReports API
type BackgroundScanReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              BackgroundScanReportSpec `json:"spec"`
}

func (r *BackgroundScanReport) GetResults() []policyreportv1alpha2.PolicyReportResult {
	return r.Spec.Results
}

func (r *BackgroundScanReport) SetResults(results []policyreportv1alpha2.PolicyReportResult) {
	r.Spec.Results = results
}

func (r *BackgroundScanReport) SetSummary(summary policyreportv1alpha2.PolicyReportSummary) {
	r.Spec.Summary = summary
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:scope=Cluster,shortName=cbgscanr,categories=kyverno
// +kubebuilder:printcolumn:name="ApiVersion",type=string,JSONPath=".metadata.ownerReferences[0].apiVersion"
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=".metadata.ownerReferences[0].kind"
// +kubebuilder:printcolumn:name="Subject",type=string,JSONPath=".metadata.ownerReferences[0].name"
// +kubebuilder:printcolumn:name="Pass",type=integer,JSONPath=".spec.summary.pass"
// +kubebuilder:printcolumn:name="Fail",type=integer,JSONPath=".spec.summary.fail"
// +kubebuilder:printcolumn:name="Warn",type=integer,JSONPath=".spec.summary.warn"
// +kubebuilder:printcolumn:name="Error",type=integer,JSONPath=".spec.summary.error"
// +kubebuilder:printcolumn:name="Skip",type=integer,JSONPath=".spec.summary.skip"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Hash",type=string,JSONPath=".metadata.labels['audit\\.kyverno\\.io/resource\\.hash']",priority=1

// ClusterBackgroundScanReport is the Schema for the ClusterBackgroundScanReports API
type ClusterBackgroundScanReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              BackgroundScanReportSpec `json:"spec"`
}

func (r *ClusterBackgroundScanReport) GetResults() []policyreportv1alpha2.PolicyReportResult {
	return r.Spec.Results
}

func (r *ClusterBackgroundScanReport) SetResults(results []policyreportv1alpha2.PolicyReportResult) {
	r.Spec.Results = results
}

func (r *ClusterBackgroundScanReport) SetSummary(summary policyreportv1alpha2.PolicyReportSummary) {
	r.Spec.Summary = summary
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
