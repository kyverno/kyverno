package v1alpha1

import (
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"

// UserInfo declares user infos to be loaded by the Kyverno CLI
type UserInfo struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// RequestInfo declares user infos
	kyvernov1beta1.RequestInfo `json:",inline"`
}
