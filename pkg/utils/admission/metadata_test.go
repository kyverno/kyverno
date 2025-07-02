package admission

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/json"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestUnmarshalPartialObjectMetadata(t *testing.T) {
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
			result, err := UnmarshalPartialObjectMetadata(test.raw)
			if test.expectErr {
				if err == nil {
					t.Error("Expected an error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				var object *metav1.PartialObjectMetadata
				json.Unmarshal(test.raw, &object)
				if !reflect.DeepEqual(result, object) {
					t.Errorf("Expected %+v, got %+v", object, result)
				}
			}
		})
	}
}

func TestGetPartialObjectMetadatas(t *testing.T) {
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
			p1, p2, _ := GetPartialObjectMetadatas(test.args.request)
			object, err := UnmarshalPartialObjectMetadata(test.args.request.Object.Raw)
			var nil_object *metav1.PartialObjectMetadata = nil
			if err != nil {
				if !reflect.DeepEqual(nil_object, p1) || !reflect.DeepEqual(nil_object, p2) {
					t.Errorf("Expected policies %+v and %+v , got %+v and %+v ", nil_object, nil_object, p1, p2)
				}
			} else if test.args.request.Operation != admissionv1.Update {
				if !reflect.DeepEqual(object, p1) || !reflect.DeepEqual(nil_object, p2) {
					t.Errorf("Expected policies %+v and %+v , got %+v and %+v ", object, nil_object, p1, p2)
				}
			} else {
				oldObject, err := UnmarshalPartialObjectMetadata(test.args.request.OldObject.Raw)
				if err != nil {
					if !reflect.DeepEqual(nil_object, p1) || !reflect.DeepEqual(nil_object, p2) {
						t.Errorf("Expected policies %+v and %+v , got %+v and %+v ", nil_object, nil_object, p1, p2)
					}
				}
				if !reflect.DeepEqual(object, p1) || !reflect.DeepEqual(oldObject, p2) {
					t.Errorf("Expected policies %+v and %+v , got %+v and %+v ", object, oldObject, p1, p2)
				}
			}
		})
	}
}
