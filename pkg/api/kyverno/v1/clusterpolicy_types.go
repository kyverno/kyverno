package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterPolicy ...
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=clusterpolicies,scope="Cluster",shortName=cpol
// +kubebuilder:printcolumn:name="Background",type="string",JSONPath=".spec.background"
// +kubebuilder:printcolumn:name="Validatoin Failure Action",type="string",JSONPath=".spec.validationFailureAction"
type ClusterPolicy struct {
	metav1.TypeMeta   `json:",inline,omitempty" yaml:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	// Spec is the information to identify the policy
	Spec Spec `json:"spec" yaml:"spec"`
	// Status contains statistics related to policy
	Status PolicyStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// ClusterPolicyList ...
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterPolicyList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata" yaml:"metadata"`
	Items           []ClusterPolicy `json:"items" yaml:"items"`
}
