package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"

// Values declares values to be loaded by the Kyverno CLI.
type Values struct {
	metav1.TypeMeta `json:",inline,omitempty"`
	// Spec declares values.
	Spec ValuesSpec `json:"spec"`
}
