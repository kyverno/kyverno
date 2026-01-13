package v1alpha1

import (
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"

// HTTPResponse declares HTTP specific responses to be loaded by the Kyverno CLI
type HTTPResponse struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              HTTPResponseSpec
}

type HTTPResponseSpec struct {
	Response *http.Response `json:"response"`
}
