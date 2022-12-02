package admission

import (
	"errors"
	"reflect"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResponse(t *testing.T) {
	type args struct {
		err      error
		warnings []string
	}
	tests := []struct {
		name string
		args args
		want *admissionv1.AdmissionResponse
	}{{
		name: "no error, no warnings",
		args: args{
			err:      nil,
			warnings: nil,
		},
		want: &admissionv1.AdmissionResponse{
			Allowed: true,
		},
	}, {
		name: "no error, warnings",
		args: args{
			err:      nil,
			warnings: []string{"foo", "bar"},
		},
		want: &admissionv1.AdmissionResponse{
			Allowed:  true,
			Warnings: []string{"foo", "bar"},
		},
	}, {
		name: "error, no warnings",
		args: args{
			err:      errors.New("an error has occured"),
			warnings: nil,
		},
		want: &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "an error has occured",
			},
		},
	}, {
		name: "error, warnings",
		args: args{
			err:      errors.New("an error has occured"),
			warnings: []string{"foo", "bar"},
		},
		want: &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "an error has occured",
			},
			Warnings: []string{"foo", "bar"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Response("", tt.args.err, tt.args.warnings...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Response() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResponseSuccess(t *testing.T) {
	type args struct {
		warnings []string
	}
	tests := []struct {
		name string
		args args
		want *admissionv1.AdmissionResponse
	}{{
		name: "no warnings",
		args: args{
			warnings: nil,
		},
		want: &admissionv1.AdmissionResponse{
			Allowed: true,
		},
	}, {
		name: "warnings",
		args: args{
			warnings: []string{"foo", "bar"},
		},
		want: &admissionv1.AdmissionResponse{
			Allowed:  true,
			Warnings: []string{"foo", "bar"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResponseSuccess("", tt.args.warnings...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ResponseSuccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutationResponse(t *testing.T) {
	type args struct {
		patch    []byte
		warnings []string
	}
	tests := []struct {
		name string
		args args
		want *admissionv1.AdmissionResponse
	}{{
		name: "no patch, no warnings",
		args: args{
			patch:    nil,
			warnings: nil,
		},
		want: &admissionv1.AdmissionResponse{
			Allowed: true,
		},
	}, {
		name: "no patch, warnings",
		args: args{
			patch:    nil,
			warnings: []string{"foo", "bar"},
		},
		want: &admissionv1.AdmissionResponse{
			Allowed:  true,
			Warnings: []string{"foo", "bar"},
		},
	}, {
		name: "patch, no warnings",
		args: args{
			patch:    []byte{1, 2, 3, 4},
			warnings: nil,
		},
		want: &admissionv1.AdmissionResponse{
			Allowed:   true,
			Patch:     []byte{1, 2, 3, 4},
			PatchType: &patchTypeJSONPatch,
		},
	}, {
		name: "patch, warnings",
		args: args{
			patch:    []byte{1, 2, 3, 4},
			warnings: []string{"foo", "bar"},
		},
		want: &admissionv1.AdmissionResponse{
			Allowed:   true,
			Patch:     []byte{1, 2, 3, 4},
			Warnings:  []string{"foo", "bar"},
			PatchType: &patchTypeJSONPatch,
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MutationResponse("", tt.args.patch, tt.args.warnings...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MutationResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}
