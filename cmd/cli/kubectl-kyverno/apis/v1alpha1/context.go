package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"

// Values declares values to be loaded by the Kyverno CLI
type Context struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	ContextSpec `json:"spec"`
}

type ContextSpec struct {
	Resources []unstructured.Unstructured `json:"resources,omitempty"`
}
