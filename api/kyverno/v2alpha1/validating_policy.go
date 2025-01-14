package v2alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster

type ValidatingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ValidatingPolicySpec   `json:"spec,omitempty"`
	Status            ValidatingPolicyStatus `json:"status,omitempty"`
}

type ValidatingPolicySpec struct {
	admissionregistrationv1.ValidatingAdmissionPolicySpec `json:",inline"`
	Background                                            bool   `json:"background"`
	ScanSchedule                                          string `json:"scanSchedule"`
}

func (s *ValidatingPolicy) GetFailurePolicy() admissionregistrationv1.FailurePolicyType {
	if s.Spec.FailurePolicy == nil {
		return admissionregistrationv1.Fail
	}
	return *s.Spec.FailurePolicy
}

type ValidatingPolicyStatus struct {
	// Information when was the last time the scan was successfully scheduled.
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ValidatingPolicyList is a list of Validating Policies
type ValidatingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ValidatingPolicy `json:"items"`
}
