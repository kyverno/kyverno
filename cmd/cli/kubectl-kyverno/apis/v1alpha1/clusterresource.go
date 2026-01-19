package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"

// ClusterResource declares Kubernetes specific resources to be loaded by the Kyverno CLI
type ClusterResource struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ClusterResourceSpec `json:"spec"`
}

type ClusterResourceSpec struct {
	CRDs      []string                     `json:"crds,omitempty"`
	Resources []*unstructured.Unstructured `json:"resources,omitempty"`
}
