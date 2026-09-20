package internal

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	PolicyConfigMediaType = "application/vnd.cncf.kyverno.config.v1+json"
	PolicyLayerMediaType  = "application/vnd.cncf.kyverno.policy.layer.v1+yaml"
	AnnotationKind        = "io.kyverno.image.kind"
	AnnotationName        = "io.kyverno.image.name"
	AnnotationNamespace   = "io.kyverno.image.namespace"
	AnnotationApiVersion  = "io.kyverno.image.apiVersion"
)

// Annotations builds OCI layer annotations for any Kubernetes object that is
// also a runtime.Object (so GroupVersionKind is available).
func Annotations(obj interface {
	metav1.Object
	runtime.Object
}) map[string]string {
	if obj == nil {
		return nil
	}
	gvk := obj.GetObjectKind().GroupVersionKind()
	ann := map[string]string{
		AnnotationKind:       gvk.Kind,
		AnnotationName:       obj.GetName(),
		AnnotationApiVersion: gvk.GroupVersion().String(),
	}
	if ns := obj.GetNamespace(); ns != "" {
		ann[AnnotationNamespace] = ns
	}
	return ann
}
