package admission

import (
	"reflect"
	"testing"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
)

func TestUnmarshalGlobalContext(t *testing.T) {
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
			result, err := UnmarshalGlobalContextEntry(test.raw)
			if test.expectErr {
				if err == nil {
					t.Error("Expected an error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				var gctx *kyvernov2alpha1.GlobalContextEntry
				json.Unmarshal(test.raw, &gctx)
				if !reflect.DeepEqual(result, gctx) {
					t.Errorf("Expected %+v, got %+v", gctx, result)
				}
			}
		})
	}
}

func TestGetGlobalContext(t *testing.T) {
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
			g1, g2, _ := GetGlobalContextEntry(test.args.request)
			var empty *kyvernov2alpha1.GlobalContextEntry
			expectedG1, err := UnmarshalGlobalContextEntry(test.args.request.Object.Raw)
			if err != nil {
				expectedG2 := empty
				if !reflect.DeepEqual(expectedG1, g1) || !reflect.DeepEqual(expectedG2, g2) {
					t.Errorf("Expected policies %+v and %+v , got %+v and %+v ", expectedG1, expectedG2, g1, g2)
				}
			} else if test.args.request.Operation == admissionv1.Update {
				expectedG2, err := UnmarshalGlobalContextEntry(test.args.request.OldObject.Raw)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if !reflect.DeepEqual(expectedG1, g1) || !reflect.DeepEqual(expectedG2, g2) {
					t.Errorf("Expected policies %+v and %+v , got %+v and %+v ", expectedG1, expectedG2, g1, g2)
				}
			} else {
				expectedG2 := empty
				if !reflect.DeepEqual(expectedG1, g1) || !reflect.DeepEqual(expectedG2, g2) {
					t.Errorf("Expected policies %+v and %+v , got %+v and %+v ", expectedG1, expectedG2, g1, g2)
				}
			}
		})
	}
}
