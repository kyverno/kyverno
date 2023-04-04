package admission

import (
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
)

func TestIsDryRun(t *testing.T) {
	true := true
	false := false
	type args struct {
		request admissionv1.AdmissionRequest
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{
			request: admissionv1.AdmissionRequest{},
		},
		want: false,
	}, {
		args: args{
			request: admissionv1.AdmissionRequest{
				DryRun: &true,
			},
		},
		want: true,
	}, {
		args: args{
			request: admissionv1.AdmissionRequest{
				DryRun: &false,
			},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDryRun(tt.args.request); got != tt.want {
				t.Errorf("IsDryRun() = %v, want %v", got, tt.want)
			}
		})
	}
}
