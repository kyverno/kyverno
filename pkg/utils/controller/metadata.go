package controller

import (
	"github.com/kyverno/kyverno/api/kyverno"
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

func GetLabel(obj metav1.Object, key string) string {
	labels := obj.GetLabels()
	if labels == nil {
		return ""
	}
	return labels[key]
}

func SetManagedByKyvernoLabel(obj metav1.Object) {
	SetLabel(obj, kyverno.LabelAppManagedBy, kyverno.ValueKyvernoApp)
}

func IsManagedByKyverno(obj metav1.Object) bool {
	return CheckLabel(obj, kyverno.LabelAppManagedBy, kyverno.ValueKyvernoApp)
}

func HasLabel(obj metav1.Object, key string) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}
	_, exists := labels[key]
	return exists
}

func SetAnnotation(obj metav1.Object, key, value string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[key] = value
	obj.SetAnnotations(annotations)
}

func GetAnnotation(obj metav1.Object, key string) string {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return ""
	}
	return annotations[key]
}

func HasAnnotation(obj metav1.Object, key string) bool {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, exists := annotations[key]
	return exists
}

func SetOwner(obj metav1.Object, apiVersion, kind, name string, uid types.UID) {
	obj.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		UID:        uid,
	}})
}
