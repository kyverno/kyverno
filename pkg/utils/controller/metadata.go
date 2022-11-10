package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func SetLabel(obj metav1.Object, key, value string) map[string]string {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[key] = value
	obj.SetLabels(labels)
	return labels
}

func CheckLabel(obj metav1.Object, key, value string) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}
	return labels[key] == value
}

func SetAnnotation(obj metav1.Object, key, value string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[key] = value
	obj.SetAnnotations(annotations)
}

func SetOwner(obj metav1.Object, apiVersion, kind, name string, uid types.UID) {
	obj.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		UID:        uid,
	}})
}
