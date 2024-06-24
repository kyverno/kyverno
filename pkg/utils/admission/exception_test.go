package admission

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/json"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestUnmarshalPolicyException(t *testing.T) {
	testCases := []struct {
		name      string
		raw       []byte
		expectErr bool
	}{
		{
			name: "Valid JSON",
			raw:  []byte(`{"field": "value"}`),
		},
		{
			name:      "Invalid JSON data",
			raw:       []byte(`invalid JSON data`),
			expectErr: true,
		},
		{
			name: "Empty JSON",
			raw:  []byte(`{}`),
		},
		{
			name:      "Missing Field",
			raw:       []byte(``),
			expectErr: true,
		},
		{
			name: "Nested Array",
			raw:  []byte(`{"nested": [{"field": "value"}]}`),
		},
		{
			name:      "Invalid Type",
			raw:       []byte(`123`),
			expectErr: true,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			result, err := UnmarshalPolicyException(test.raw)
			if test.expectErr {
				if err == nil {
					t.Error("Expected an error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				var exception *kyvernov2.PolicyException
				json.Unmarshal(test.raw, &exception)
				if !reflect.DeepEqual(result, exception) {
					t.Errorf("Expected %+v, got %+v", exception, result)
				}
			}
		})
	}
}

func TestGetPolicyExceptions(t *testing.T) {
	type args struct {
		request admissionv1.AdmissionRequest
	}
	testCases := []struct {
		name string
		args args
	}{{
		name: "Valid JSON",
		args: args{
			request: admissionv1.AdmissionRequest{
				Object: runtime.RawExtension{
					Raw: []byte(`{"field":"value"}`),
				},
				OldObject: runtime.RawExtension{
					Raw: []byte(`{"field":"value"}`),
				},
				Operation: "CREATE",
			},
		},
	}, {
		name: "Invalid JSON data",
		args: args{
			request: admissionv1.AdmissionRequest{
				Object: runtime.RawExtension{
					Raw: []byte(`invalid JSON data`),
				},
				OldObject: runtime.RawExtension{
					Raw: []byte(`{"field":"value"}`),
				},
				Operation: "UPDATE",
			},
		},
	}, {
		name: "Empty JSON",
		args: args{
			request: admissionv1.AdmissionRequest{
				Object: runtime.RawExtension{
					Raw: []byte(`{}`),
				},
				OldObject: runtime.RawExtension{
					Raw: []byte(`{"field":"value"}`),
				},
				Operation: "DELETE",
			},
		},
	}, {
		name: "Missing Field",
		args: args{
			request: admissionv1.AdmissionRequest{
				Object: runtime.RawExtension{
					Raw: []byte(``),
				},
				OldObject: runtime.RawExtension{
					Raw: []byte(`{"field":"value"}`),
				},
				Operation: "CONNECT",
			},
		},
	}, {
		name: "Nested Array",
		args: args{
			request: admissionv1.AdmissionRequest{
				Object: runtime.RawExtension{
					Raw: []byte(`{"nested": [{"field": "value"}]}`),
				},
				OldObject: runtime.RawExtension{
					Raw: []byte(`{"field":"value"}`),
				},
				Operation: "DELETE",
			},
		},
	}, {
		name: "Invalid Type",
		args: args{
			request: admissionv1.AdmissionRequest{
				Object: runtime.RawExtension{
					Raw: []byte(`123`),
				},
				OldObject: runtime.RawExtension{
					Raw: []byte(`{"field":"value"}`),
				},
				Operation: "UPDATE",
			},
		},
	}}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			p1, p2, _ := GetPolicyExceptions(test.args.request)
			var empty *kyvernov2.PolicyException
			expectedP1, err := UnmarshalPolicyException(test.args.request.Object.Raw)
			if err != nil {
				expectedP2 := empty
				if !reflect.DeepEqual(expectedP1, p1) || !reflect.DeepEqual(expectedP2, p2) {
					t.Errorf("Expected policies %+v and %+v , got %+v and %+v ", expectedP1, expectedP2, p1, p2)
				}
			} else if test.args.request.Operation == admissionv1.Update {
				expectedP2, err := UnmarshalPolicyException(test.args.request.OldObject.Raw)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if !reflect.DeepEqual(expectedP1, p1) || !reflect.DeepEqual(expectedP2, p2) {
					t.Errorf("Expected policies %+v and %+v , got %+v and %+v ", expectedP1, expectedP2, p1, p2)
				}
			} else {
				expectedP2 := empty
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if !reflect.DeepEqual(expectedP1, p1) || !reflect.DeepEqual(expectedP2, p2) {
					t.Errorf("Expected policies %+v and %+v , got %+v and %+v ", expectedP1, expectedP2, p1, p2)
				}
			}
		})
	}
}
