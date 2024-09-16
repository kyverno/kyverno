package v2alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GlobalContextEntryStatus struct {
	// Deprecated in favor of Conditions
	Ready *bool `json:"ready,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Indicates the time when the globalcontextentry was last refreshed successfully for the API Call
	// +optional
	LastRefreshTime metav1.Time `json:"lastRefreshTime,omitempty"`
}

func (status *GlobalContextEntryStatus) UpdateRefreshTime() {
	status.LastRefreshTime = metav1.Now()
}
