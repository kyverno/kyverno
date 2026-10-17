package internal

import (
	"reflect"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAnnotationsValidatingPolicy(t *testing.T) {
	vp := &policiesv1beta1.ValidatingPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ValidatingPolicy",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "check-labels",
		},
	}
	got := Annotations(vp)
	want := map[string]string{
		AnnotationKind:       "ValidatingPolicy",
		AnnotationName:       "check-labels",
		AnnotationApiVersion: "policies.kyverno.io/v1beta1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Annotations(ValidatingPolicy) = %v, want %v", got, want)
	}
}

func TestAnnotationsDeletingPolicy(t *testing.T) {
	dp := &policiesv1beta1.DeletingPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeletingPolicy",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "delete-stale",
		},
	}
	got := Annotations(dp)
	want := map[string]string{
		AnnotationKind:       "DeletingPolicy",
		AnnotationName:       "delete-stale",
		AnnotationApiVersion: "policies.kyverno.io/v1beta1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Annotations(DeletingPolicy) = %v, want %v", got, want)
	}
}

func TestAnnotationsMutatingPolicy(t *testing.T) {
	mp := &policiesv1beta1.MutatingPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "MutatingPolicy",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "add-labels",
		},
	}
	got := Annotations(mp)
	want := map[string]string{
		AnnotationKind:       "MutatingPolicy",
		AnnotationName:       "add-labels",
		AnnotationApiVersion: "policies.kyverno.io/v1beta1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Annotations(MutatingPolicy) = %v, want %v", got, want)
	}
}

func TestAnnotationsCELPolicyException(t *testing.T) {
	pe := &policiesv1beta1.PolicyException{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PolicyException",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-exception",
			Namespace: "default",
		},
	}
	got := Annotations(pe)
	want := map[string]string{
		AnnotationKind:       "PolicyException",
		AnnotationName:       "my-exception",
		AnnotationNamespace:  "default",
		AnnotationApiVersion: "policies.kyverno.io/v1beta1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Annotations(CEL PolicyException) = %v, want %v", got, want)
	}
}

func TestAnnotationsNamespaceOmittedWhenEmpty(t *testing.T) {
	vp := &policiesv1beta1.ValidatingPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ValidatingPolicy",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-scoped",
		},
	}
	got := Annotations(vp)
	if _, ok := got[AnnotationNamespace]; ok {
		t.Errorf("expected no namespace annotation for cluster-scoped resource, got %v", got)
	}
}
