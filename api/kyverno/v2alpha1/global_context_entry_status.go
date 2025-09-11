package v2alpha1

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// GlobalContextEntryConditionReady means that the globalcontextentry is ready
	GlobalContextEntryConditionReady = "Ready"
)

const (
	// GlobalContextEntryReasonSucceeded is the reason set when the globalcontextentry is ready
	GlobalContextEntryReasonSucceeded = "Succeeded"
	// GlobalContextEntryReasonFailed is the reason set when the globalcontextentry is not ready
	GlobalContextEntryReasonFailed = "Failed"
)

type GlobalContextEntryStatus struct {
	// Deprecated in favor of Conditions
	Ready bool `json:"ready"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +optional
	LastRefreshTime metav1.Time `json:"lastRefreshTime,omitempty"`
}

func (status *GlobalContextEntryStatus) SetReady(ready bool, message string) {
	condition := metav1.Condition{
		Type:    GlobalContextEntryConditionReady,
		Message: message,
	}
	if ready {
		condition.Status = metav1.ConditionTrue
		condition.Reason = GlobalContextEntryReasonSucceeded
	} else {
		condition.Status = metav1.ConditionFalse
		condition.Reason = GlobalContextEntryReasonFailed
	}
	status.Ready = ready
	meta.SetStatusCondition(&status.Conditions, condition)
}

// UpdateRefreshTime updates the lastRefreshTime field
func (status *GlobalContextEntryStatus) UpdateRefreshTime() {
	status.LastRefreshTime = metav1.Now()
}

// IsReady indicates if the globalcontextentry has loaded
func (status *GlobalContextEntryStatus) IsReady() bool {
	condition := meta.FindStatusCondition(status.Conditions, GlobalContextEntryConditionReady)
	return condition != nil && condition.Status == metav1.ConditionTrue
}
