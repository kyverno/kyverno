package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterPolicyReport is the Schema for the clusterpolicyreports API
type ValidatingAdmissionPoliciesPolicyReport struct {
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

func (r *ValidatingAdmissionPoliciesPolicyReport) GetResults() []PolicyReportResult {
	return r.Results
}

func (r *ValidatingAdmissionPoliciesPolicyReport) SetResults(results []PolicyReportResult) {
	r.Results = results
}

func (r *ValidatingAdmissionPoliciesPolicyReport) SetSummary(summary PolicyReportSummary) {
	r.Summary = summary
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterPolicyReportList contains a list of ClusterPolicyReport
type ValidatingAdmissionPoliciesPolicyReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ValidatingAdmissionPoliciesPolicyReport `json:"items"`
}