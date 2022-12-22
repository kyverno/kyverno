package admission

import (
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetResourceName(t *testing.T) {
	type args struct {
		request *admissionv1.AdmissionRequest
	}
	tests := []struct {
		name string
		args args
		want string
	}{{
		name: "with namespace",
		args: args{
			request: &admissionv1.AdmissionRequest{
				Kind: v1.GroupVersionKind{
					Kind: "Pod",
				},
				Name:      "dummy",
				Namespace: "ns",
			},
		},
		want: "ns/Pod/dummy",
	}, {
		name: "without namespace",
		args: args{
			request: &admissionv1.AdmissionRequest{
				Kind: v1.GroupVersionKind{
					Kind: "Namespace",
				},
				Name: "dummy",
			},
		},
		want: "Namespace/dummy",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetResourceName(tt.args.request); got != tt.want {
				t.Errorf("GetResourceName() = %v, want %v", got, tt.want)
			}
		})
	}
}
