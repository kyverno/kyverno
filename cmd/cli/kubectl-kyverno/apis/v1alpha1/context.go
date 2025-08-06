package v1alpha1

import (
	"github.com/kyverno/kyverno-json/pkg/apis/policy/v1alpha1"
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
	Images    []ImageData                 `json:"images,omitempty"`
}

type ImageData struct {
	Image         string       `json:"image"`
	ResolvedImage string       `json:"resolvedImage"`
	Registry      string       `json:"registry"`
	Repository    string       `json:"repository"`
	Tag           string       `json:"tag,omitempty"`
	Digest        string       `json:"digest,omitempty"`
	ImageIndex    v1alpha1.Any `json:"imageIndex,omitempty"`
	Manifest      v1alpha1.Any `json:"manifest,omitempty"`
	ConfigData    v1alpha1.Any `json:"config,omitempty"`
}
