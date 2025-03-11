package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	Images    []ImageData                 `json:"images,omitempty"`
}

type ImageData struct {
	Image         string               `json:"image"`
	ResolvedImage string               `json:"resolvedImage"`
	Registry      string               `json:"registry"`
	Repository    string               `json:"repository"`
	Tag           string               `json:"tag,omitempty"`
	Digest        string               `json:"digest,omitempty"`
	ImageIndex    runtime.RawExtension `json:"imageIndex,omitempty"`
	Manifest      runtime.RawExtension `json:"manifest"`
	Config        runtime.RawExtension `json:"config"`
}
