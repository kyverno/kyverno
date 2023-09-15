package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Subresource struct {
	APIResource    metav1.APIResource `json:"subresource"`
	ParentResource metav1.APIResource `json:"parentResource"`
}
