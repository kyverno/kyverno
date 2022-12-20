package openapi

import (
	openapiv2 "github.com/google/gnostic/openapiv2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Manager interface {
	UseOpenAPIDocument(*openapiv2.Document) error
	DeleteCRDFromPreviousSync()
	ParseCRD(unstructured.Unstructured)
	UpdateKindToAPIVersions([]*metav1.APIResourceList, []*metav1.APIResourceList)
	GetCrdList() []string
}
