package admission

import (
	"reflect"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
)

func TestUnmarshalPolicy(t *testing.T) {
	tests := []struct {
		name string
		kind string
		raw  []byte
	}{
		{
			name: "Policy",
			kind: "Policy",
			raw:  []byte(`{"field":"value"}`),
		},
		{
			name: "ClusterPolicy",
			kind: "ClusterPolicy",
			raw:  []byte(`{"field":"value"}`),
		},
		{
			name: "ValidatingPolicy",
			kind: "ValidatingPolicy",
			raw:  []byte(`{"field":"value"}`),
		},
		{
			name: "ImageValidatingPolicy",
			kind: "ImageValidatingPolicy",
			raw:  []byte(`{"field":"value"}`),
		},
		{
			name: "DeletingPolicy",
			kind: "DeletingPolicy",
			raw:  []byte(`{"field":"value"}`),
		},
		{
			name: "InvalidKind",
			kind: "InvalidKind",
			raw:  []byte(`{"field":"value"}`),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy, err := UnmarshalPolicy(test.kind, test.raw)
			switch test.kind {
			case "ClusterPolicy":
				var expectedPolicy *kyvernov1.ClusterPolicy
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if err := json.Unmarshal(test.raw, &expectedPolicy); err != nil {
					expectedPolicy = nil
				}
				if !reflect.DeepEqual(policy.AsKyvernoPolicy(), expectedPolicy) {
					t.Errorf("Expected policy %+v, got %+v", expectedPolicy, policy.AsKyvernoPolicy())
				}
			case "Policy":
				var expectedPolicy *kyvernov1.Policy
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if err := json.Unmarshal(test.raw, &expectedPolicy); err != nil {
					expectedPolicy = nil
				}
				if !reflect.DeepEqual(policy.AsKyvernoPolicy(), expectedPolicy) {
					t.Errorf("Expected policy %+v, got %+v", expectedPolicy, policy.AsKyvernoPolicy())
				}
			case "ValidatingPolicy":
				var expectedPolicy *v1alpha1.ValidatingPolicy
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if err := json.Unmarshal(test.raw, &expectedPolicy); err != nil {
					expectedPolicy = nil
				}
				if !reflect.DeepEqual(policy.AsValidatingPolicy(), expectedPolicy) {
					t.Errorf("Expected policy %+v, got %+v", expectedPolicy, policy.AsValidatingPolicy())
				}
			case "ImageValidatingPolicy":
				var expectedPolicy *v1alpha1.ImageValidatingPolicy
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if err := json.Unmarshal(test.raw, &expectedPolicy); err != nil {
					expectedPolicy = nil
				}
				if !reflect.DeepEqual(policy.AsImageValidatingPolicy(), expectedPolicy) {
					t.Errorf("Expected policy %+v, got %+v", expectedPolicy, policy.AsImageValidatingPolicy())
				}
			case "DeletingPolicy":
				var expectedPolicy *v1alpha1.DeletingPolicy
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if err := json.Unmarshal(test.raw, &expectedPolicy); err != nil {
					expectedPolicy = nil
				}
				if !reflect.DeepEqual(policy.AsDeletingPolicy(), expectedPolicy) {
					t.Errorf("Expected policy %+v, got %+v", expectedPolicy, policy.AsDeletingPolicy())
				}
			default:
				if !reflect.DeepEqual(policy, nil) {
					t.Errorf("Expected policy %+v, got %+v", nil, policy)
				}
			}
		})
	}
}

func TestGetPolicies(t *testing.T) {
	type args struct {
		request admissionv1.AdmissionRequest
	}
	tests := []struct {
		name string
		args args
	}{{
		name: "Policy",
		args: args{
			request: admissionv1.AdmissionRequest{
				Kind: v1.GroupVersionKind{
					Kind: "Policy",
				},
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
		name: "ClusterPolicy",
		args: args{
			request: admissionv1.AdmissionRequest{
				Kind: v1.GroupVersionKind{
					Kind: "ClusterPolicy",
				},
				Object: runtime.RawExtension{
					Raw: []byte(`{"field":"value"}`),
				},
				OldObject: runtime.RawExtension{
					Raw: []byte(`{"field":"value"}`),
				},
				Operation: "UPDATE",
			},
		},
	}, {
		name: "InvalidKind1",
		args: args{
			request: admissionv1.AdmissionRequest{
				Kind: v1.GroupVersionKind{
					Kind: "InvalidKind1",
				},
				Object: runtime.RawExtension{
					Raw: []byte(`{"field":"value"}`),
				},
				OldObject: runtime.RawExtension{
					Raw: []byte(`{"field":"value"}`),
				},
				Operation: "DELETE",
			},
		},
	}, {
		name: "InvalidKind2",
		args: args{
			request: admissionv1.AdmissionRequest{
				Kind: v1.GroupVersionKind{
					Kind: "InvalidKind2",
				},
				Object: runtime.RawExtension{
					Raw: []byte(`{"field":"value"}`),
				},
				OldObject: runtime.RawExtension{
					Raw: []byte(`{"field":"value"}`),
				},
				Operation: "CONNECT",
			},
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p1, p2, _ := GetPolicies(test.args.request)
			expectedP1, err := UnmarshalPolicy(test.args.request.Kind.Kind, test.args.request.Object.Raw)
			if err != nil {
				if !reflect.DeepEqual(expectedP1, p1) || !reflect.DeepEqual(nil, p2) {
					t.Errorf("Expected policies %+v and %+v , got %+v and %+v ", expectedP1, nil, p1, p2)
				}
			} else if test.args.request.Operation == admissionv1.Update {
				expectedP2, err := UnmarshalPolicy(test.args.request.Kind.Kind, test.args.request.Object.Raw)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if !reflect.DeepEqual(expectedP1, p1) || !reflect.DeepEqual(expectedP2, p2) {
					t.Errorf("Expected policies %+v and %+v , got %+v and %+v ", expectedP1, expectedP2, p1, p2)
				}
			} else {
				if !reflect.DeepEqual(expectedP1, p1) || !reflect.DeepEqual(nil, p2) {
					t.Errorf("Expected policies %+v and %+v , got %+v and %+v ", expectedP1, nil, p1, p2)
				}
			}
		})
	}
}
