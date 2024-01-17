package internal

import (
	"reflect"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAnnotations(t *testing.T) {
	tests := []struct {
		name   string
		policy kyvernov1.PolicyInterface
		want   map[string]string
	}{{
		name:   "nil",
		policy: nil,
		want:   nil,
	}, {
		name: "cluster policy",
		policy: &kyvernov1.ClusterPolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       "kyverno.io/v1",
				APIVersion: "ClusterPolicy",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		},
		want: map[string]string{
			AnnotationKind:       "ClusterPolicy",
			AnnotationName:       "test",
			AnnotationApiVersion: "kyverno.io/v1",
		},
	}, {
		name: "policy",
		policy: &kyvernov1.Policy{
			TypeMeta: metav1.TypeMeta{
				Kind:       "kyverno.io/v1",
				APIVersion: "Policy",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		},
		want: map[string]string{
			AnnotationKind:       "Policy",
			AnnotationName:       "test",
			AnnotationApiVersion: "kyverno.io/v1",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Annotations(tt.policy); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Annotations() = %v, want %v", got, tt.want)
			}
		})
	}
}
