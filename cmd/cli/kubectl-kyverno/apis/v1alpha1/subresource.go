package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Subresource declares subresource/parent resource mapping
type Subresource struct {
	// Subresource declares the subresource api
	Subresource metav1.APIResource `json:"subresource"`

	// ParentResource declares the parent resource api
	ParentResource metav1.APIResource `json:"parentResource"`
}
