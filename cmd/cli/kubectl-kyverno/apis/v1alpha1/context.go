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
	// GlobalContext provides fixture data for the kyverno.globalcontext CEL library.
	GlobalContext []GlobalContextEntry `json:"globalContext,omitempty"`
	// HTTP provides fixture data for the kyverno.http CEL library.
	HTTP []HTTPStub `json:"http,omitempty"`
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

// GlobalContextEntry represents a single globalcontext fixture.
// If both Value and ValueFile are set, ValueFile takes precedence.
type GlobalContextEntry struct {
	Name       string       `json:"name"`
	Projection string       `json:"projection,omitempty"`
	Value      *kyverno.Any `json:"value,omitempty"`
	ValueFile  string       `json:"valueFile,omitempty"`
}

// HTTPStub represents a single HTTP interaction fixture.
// If both Body and BodyFile are set, BodyFile takes precedence.
type HTTPStub struct {
	Method   string            `json:"method"`
	URL      string            `json:"url"`
	Status   int               `json:"status,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     *kyverno.Any      `json:"body,omitempty"`
	BodyFile string            `json:"bodyFile,omitempty"`
}
