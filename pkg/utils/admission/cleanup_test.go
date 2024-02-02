package admission

import (
	"reflect"
	"testing"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
)

func TestUnmarshalCleanupPolicy(t *testing.T) {
	tests := []struct {
		name string
		kind string
		raw  []byte
	}{
		{
			name: "CleanupPolicy",
			kind: "CleanupPolicy",
			raw:  []byte(`{"field":"value"}`),
		},
		{
			name: "ClusterCleanupPolicy",
			kind: "ClusterCleanupPolicy",
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
			policy, err := UnmarshalCleanupPolicy(test.kind, test.raw)
			var expectedPolicy kyvernov2alpha1.CleanupPolicyInterface
			switch test.kind {
			case "CleanupPolicy":
				var tempPolicy kyvernov2alpha1.CleanupPolicy
				expectedPolicy = &tempPolicy
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if err := json.Unmarshal(test.raw, expectedPolicy); err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if !reflect.DeepEqual(policy, expectedPolicy) {
					t.Errorf("Expected policy %+v, got %+v", expectedPolicy, policy)
				}
			case "ClusterCleanupPolicy":
				var tempPolicy kyvernov2alpha1.ClusterCleanupPolicy
				expectedPolicy = &tempPolicy
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if err := json.Unmarshal(test.raw, expectedPolicy); err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if !reflect.DeepEqual(policy, expectedPolicy) {
					t.Errorf("Expected policy %+v, got %+v", expectedPolicy, policy)
				}
			default:
				expectedPolicy = nil
				if !reflect.DeepEqual(policy, expectedPolicy) {
					t.Errorf("Expected policy %+v, got %+v", expectedPolicy, policy)
				}
			}
		})
	}
}

func TestGetCleanupPolicies(t *testing.T) {
	type args struct {
		request admissionv1.AdmissionRequest
	}
	tests := []struct {
		name string
		args args
	}{{
		name: "CleanupPolicy",
		args: args{
			request: admissionv1.AdmissionRequest{
				Kind: v1.GroupVersionKind{
					Kind: "CleanupPolicy",
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
		name: "ClusterCleanupPolicy",
		args: args{
			request: admissionv1.AdmissionRequest{
				Kind: v1.GroupVersionKind{
					Kind: "ClusterCleanupPolicy",
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
			p1, p2, _ := GetCleanupPolicies(test.args.request)
			var emptypolicy kyvernov2alpha1.CleanupPolicyInterface
			expectedP1, err := UnmarshalCleanupPolicy(test.args.request.Kind.Kind, test.args.request.Object.Raw)
			if err != nil {
				expectedP2 := emptypolicy
				if !reflect.DeepEqual(expectedP1, p1) || !reflect.DeepEqual(expectedP2, p2) {
					t.Errorf("Expected policies %+v and %+v , got %+v and %+v ", expectedP1, expectedP2, p1, p2)
				}
			} else if test.args.request.Operation == admissionv1.Update {
				expectedP2, err := UnmarshalCleanupPolicy(test.args.request.Kind.Kind, test.args.request.Object.Raw)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if !reflect.DeepEqual(expectedP1, p1) || !reflect.DeepEqual(expectedP2, p2) {
					t.Errorf("Expected policies %+v and %+v , got %+v and %+v ", expectedP1, expectedP2, p1, p2)
				}
			} else {
				expectedP2 := emptypolicy
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
