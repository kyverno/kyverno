package v1alpha1

import (
	"github.com/kyverno/kyverno/api/kyverno"
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
	// deprecate, use ClusterResource to define Kubernetes specific resources
	Resources []unstructured.Unstructured `json:"resources,omitempty"`
	Images    []ImageData                 `json:"images,omitempty"`
}

type ImageData struct {
	Image         string       `json:"image"`
	ResolvedImage string       `json:"resolvedImage"`
	Registry      string       `json:"registry"`
	Repository    string       `json:"repository"`
	Tag           string       `json:"tag,omitempty"`
	Digest        string       `json:"digest,omitempty"`
	ImageIndex    *kyverno.Any `json:"imageIndex,omitempty"`
	Manifest      *kyverno.Any `json:"manifest,omitempty"`
	ConfigData    *kyverno.Any `json:"config,omitempty"`
}
