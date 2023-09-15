package v1alpha1

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/values"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"

// Values declares values to be loaded by the Kyverno CLI.
type Values struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec declares values.
	Spec values.Values `json:"spec"`
}
